package solochain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"golang.org/x/sync/errgroup"

	"github.com/snowfork/go-substrate-rpc-client/v4/types"
	"github.com/snowfork/snowbridge/relayer/chain/ethereum"
	"github.com/snowfork/snowbridge/relayer/chain/parachain"
	"github.com/snowfork/snowbridge/relayer/contracts"
	"github.com/snowfork/snowbridge/relayer/crypto/merkle"
	"github.com/snowfork/snowbridge/relayer/ofac"

	log "github.com/sirupsen/logrus"
)

type BeefyListener struct {
	config              *SourceConfig
	scheduleConfig      *ScheduleConfig
	ethereumConn        *ethereum.Connection
	beefyClientContract *contracts.BeefyClient
	solochainConn       *parachain.Connection
	ofac                *ofac.OFAC
	tasks               chan<- *Task
	scanner             *Scanner
}

func NewBeefyListener(
	config *SourceConfig,
	scheduleConfig *ScheduleConfig,
	ethereumConn *ethereum.Connection,
	solochainConn *parachain.Connection,
	ofac *ofac.OFAC,
	tasks chan<- *Task,
) *BeefyListener {
	return &BeefyListener{
		config:         config,
		scheduleConfig: scheduleConfig,
		ethereumConn:   ethereumConn,
		solochainConn:  solochainConn,
		ofac:           ofac,
		tasks:          tasks,
	}
}

func (li *BeefyListener) Start(ctx context.Context, eg *errgroup.Group) error {
	// Set up light client bridge contract
	address := common.HexToAddress(li.config.Contracts.BeefyClient)
	beefyClientContract, err := contracts.NewBeefyClient(address, li.ethereumConn.Client())
	if err != nil {
		return err
	}
	li.beefyClientContract = beefyClientContract

	li.scanner = &Scanner{
		config:   li.config,
		ethConn:  li.ethereumConn,
		soloConn: li.solochainConn,
		ofac:     li.ofac,
	}

	eg.Go(func() error {
		defer close(li.tasks)

		// Subscribe NewMMRRoot event logs and fetch solochain message commitments
		// since latest beefy block
		beefyBlockNumber, _, err := li.fetchLatestBeefyBlock(ctx)
		if err != nil {
			return fmt.Errorf("fetch latest beefy block: %w", err)
		}

		log.Infof("Initial scan up to solochain beefy block %d (latest verified on Ethereum)", beefyBlockNumber)
		err = li.doScan(ctx, beefyBlockNumber)
		if err != nil {
			return fmt.Errorf("Initial scan for sync tasks bounded by BEEFY block %v: %w", beefyBlockNumber, err)
		}

		err = li.subscribeNewMMRRoots(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("Subscribing to new MMR roots: %w", err)
		}

		return nil
	})

	return nil
}

// subscribeNewMMRRoots subscribes to new MMR roots on Ethereum
func (li *BeefyListener) subscribeNewMMRRoots(ctx context.Context) error {
	headers := make(chan *gethTypes.Header, 5)

	sub, err := li.ethereumConn.Client().SubscribeNewHead(ctx, headers)
	if err != nil {
		return fmt.Errorf("creating ethereum header subscription: %w", err)
	}
	defer sub.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-sub.Err():
			return fmt.Errorf("header subscription: %w", err)
		case gethheader := <-headers:
			blockNumber := gethheader.Number.Uint64()
			contractEvents, err := li.queryBeefyClientEvents(ctx, blockNumber, &blockNumber)
			if err != nil {
				return fmt.Errorf("Failed to query NewMMRRoot event logs in block %v: %v", blockNumber, err)
			}

			if len(contractEvents) > 0 {
				log.Info(fmt.Sprintf("Found %d BeefyLightClient.NewMMRRoot events in block %d", len(contractEvents), blockNumber))
				// Only process the last emitted event in the block
				event := contractEvents[len(contractEvents)-1]
				log.WithFields(log.Fields{
					"beefyBlockNumber":    event.BlockNumber,
					"ethereumBlockNumber": event.Raw.BlockNumber,
					"ethereumTxHash":      event.Raw.TxHash.Hex(),
				}).Info("Witnessed a new MMRRoot event")

				err = li.doScan(ctx, event.BlockNumber)
				if err != nil {
					return fmt.Errorf("Scan for sync tasks bounded by BEEFY block %v failed: %w", event.BlockNumber, err)
				}
			}
		}
	}
}

// doScan scans for sync tasks bounded by a BEEFY block number
func (li *BeefyListener) doScan(ctx context.Context, beefyBlockNumber uint64) error {
	tasks, err := li.scanner.Scan(ctx, beefyBlockNumber)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		solochainNonce := (*task.MessageProofs)[0].Message.Nonce
		waitingPeriod := (uint64(solochainNonce) + li.scheduleConfig.TotalRelayerCount - li.scheduleConfig.ID) % li.scheduleConfig.TotalRelayerCount
		err = li.waitAndSend(ctx, task, waitingPeriod)
		if err != nil {
			return fmt.Errorf("Failed to wait for task for nonce %d: %w", solochainNonce, err)
		}
	}

	return nil
}

// queryBeefyClientEvents queries ContractNewMMRRoot events from the BeefyClient contract
func (li *BeefyListener) queryBeefyClientEvents(
	ctx context.Context, start uint64,
	end *uint64,
) ([]*contracts.BeefyClientNewMMRRoot, error) {
	var events []*contracts.BeefyClientNewMMRRoot
	filterOps := bind.FilterOpts{Start: start, End: end, Context: ctx}

	iter, err := li.beefyClientContract.FilterNewMMRRoot(&filterOps)
	if err != nil {
		return nil, err
	}

	for {
		more := iter.Next()
		if !more {
			err = iter.Error()
			if err != nil {
				return nil, err
			}
			break
		}

		events = append(events, iter.Event)
	}

	return events, nil
}

// fetchLatestBeefyBlock fetches the latest verified solochain BEEFY block number and hash from the BeefyClient contract on Ethereum
func (li *BeefyListener) fetchLatestBeefyBlock(ctx context.Context) (uint64, types.Hash, error) {
	beefyBlockNumber, err := li.beefyClientContract.LatestBeefyBlock(&bind.CallOpts{
		Pending: false,
		Context: ctx,
	})
	if err != nil {
		return 0, types.Hash{}, fmt.Errorf("Fetching latest BEEFY block from light client: %w", err)
	}

	beefyBlockHash, err := li.solochainConn.API().RPC.Chain.GetBlockHash(beefyBlockNumber)
	if err != nil {
		return 0, types.Hash{}, fmt.Errorf("Fetching BEEFY block hash for block %d: %w", beefyBlockNumber, err)
	}

	return beefyBlockNumber, beefyBlockHash, nil
}

// Generates a proof for an MMR leaf, and then generates a merkle proof for our message commitment, which should be verifiable against the
// message commitment root found in the `extra` field of the BEEFY MMR leaf.
func (li *BeefyListener) generateProof(ctx context.Context, input *ProofInput, solochainHeaderWithCommitment *types.Header) (*ProofOutput, error) {
	latestBeefyBlockNumber, latestBeefyBlockHash, err := li.fetchLatestBeefyBlock(ctx)
	if err != nil {
		return nil, fmt.Errorf("Fetching latest beefy block: %w", err)
	}

	log.WithFields(log.Fields{
		"beefyBlockNumber": latestBeefyBlockNumber,
		"leafIndex":        input.SolochainBlockNumber,
	}).Info("Generating MMR proof")

	// Generate the MMR proof for the solochain block.
	mmrProof, err := li.solochainConn.GenerateProofForBlock(
		input.SolochainBlockNumber+1,
		latestBeefyBlockHash,
	)
	if err != nil {
		return nil, fmt.Errorf("Generating MMR leaf proof: %w", err)
	}

	simplifiedProof, err := merkle.ConvertToSimplifiedMMRProof(
		mmrProof.BlockHash,
		uint64(mmrProof.Proof.LeafIndex),
		mmrProof.Leaf,
		uint64(mmrProof.Proof.LeafCount),
		mmrProof.Proof.Items,
	)
	if err != nil {
		return nil, fmt.Errorf("Simplifying MMR leaf proof: %w", err)
	}

	mmrRootHash, err := li.solochainConn.GetMMRRootHash(latestBeefyBlockHash)
	if err != nil {
		return nil, fmt.Errorf("Retrieving MMR root hash at block %v: %w", latestBeefyBlockHash.Hex(), err)
	}

	var merkleProofData *MerkleProofData
	merkleProofData, input.Messages, err = li.generateAndValidateMessagesMerkleProof(input, &mmrProof)
	if err != nil {
		return nil, fmt.Errorf("Generating and validating message commitments merkle proof: %w", err)
	}

	log.Debug("Created solochain BEEFY MMR proof data")

	output := ProofOutput{
		MMRProof:        simplifiedProof,
		MMRRootHash:     mmrRootHash,
		Header:          *solochainHeaderWithCommitment,
		MerkleProofData: *merkleProofData,
	}

	return &output, nil
}

// generateAndValidateMessagesMerkleProof generates a merkle proof for the messages in the proof input
func (li *BeefyListener) generateAndValidateMessagesMerkleProof(input *ProofInput, mmrProof *types.GenerateMMRProofResponse) (*MerkleProofData, []OutboundQueueMessage, error) {
	log.Infof("Generating and validating messages merkle proof for block %d", input.SolochainBlockNumber)
	log.Infof("Mmr proof blockhash: %v", mmrProof.BlockHash.Hex())
	log.Infof("Mmr proof leaf version: %v", mmrProof.Leaf.Version)
	log.Infof("Mmr proof leaf parent number: %v", mmrProof.Leaf.ParentNumberAndHash.ParentNumber)
	log.Infof("Mmr proof leaf parent hash: %v", mmrProof.Leaf.ParentNumberAndHash.Hash.Hex())
	log.Infof("Mmr proof leaf beefy next authority set ID: %v", mmrProof.Leaf.BeefyNextAuthoritySet.ID)
	log.Infof("Mmr proof leaf beefy next authority set Len: %v", mmrProof.Leaf.BeefyNextAuthoritySet.Len)
	log.Infof("Mmr proof leaf beefy next authority set Root: %v", mmrProof.Leaf.BeefyNextAuthoritySet.Root.Hex())
	log.Infof("Mmr proof leaf parachain heads (extra field): %v", mmrProof.Leaf.ParachainHeads.Hex())
	log.Infof("Mmr proof leaf index: %v", mmrProof.Proof.LeafIndex)
	log.Infof("Mmr proof leaf count: %v", mmrProof.Proof.LeafCount)
	proofItemsHex := make([]string, len(mmrProof.Proof.Items))
	for i, item := range mmrProof.Proof.Items {
		proofItemsHex[i] = item.Hex()
	}
	log.Infof("Mmr proof proof items: %v", proofItemsHex)

	log.Infof("Proof input solochain block number: %v", input.SolochainBlockNumber)
	log.Infof("Proof input solochain block hash: %v", input.SolochainBlockHash.Hex())
	log.Infof("Proof input message nonce: %v", input.MessageNonce)
	proofInputHex := make([]string, len(input.Messages))
	for i, message := range input.Messages {
		commandsStr := ""
		for _, command := range message.Commands {
			commandsStr += fmt.Sprintf("Command{Kind: %d, MaxDispatchGas: %d, Params: %s}, ", command.Kind, command.MaxDispatchGas, command.Params.Hex())
		}
		// Remove trailing comma and space
		if len(commandsStr) > 0 {
			commandsStr = commandsStr[:len(commandsStr)-2]
		}
		proofInputHex[i] = fmt.Sprintf("Message{Origin: %s, Nonce: %d, Topic: %s, Commands: [%s]}", message.Origin.Hex(), message.Nonce, message.Topic.Hex(), commandsStr)
	}
	log.Infof("Proof input messages: %v", proofInputHex)

	messages := input.Messages
	merkleProofData, err := CreateMessagesMerkleProof(messages, input.MessageNonce)
	if err != nil {
		return nil, messages, fmt.Errorf("Creating messages merkle proof: %w", err)
	}
	log.Infof("Merkle proof generated data: %v", merkleProofData)

	// Verify merkle root generated is same as value generated in the solochain and if so exit early
	if merkleProofData.Root.Hex() == mmrProof.Leaf.ParachainHeads.Hex() {
		return &merkleProofData, messages, nil
	} else {
		log.WithFields(log.Fields{
			"computedMmr": merkleProofData.Root.Hex(),
			"mmr":         mmrProof.Leaf.ParachainHeads.Hex(),
		}).Warn("MMR message commitments merkle root does not match calculated merkle root.")
		return nil, messages, fmt.Errorf("MMR message commitments merkle root does not match calculated merkle root")
	}
}

func (li *BeefyListener) waitAndSend(ctx context.Context, task *Task, waitingPeriod uint64) error {
	solochainNonce := (*task.MessageProofs)[0].Message.Nonce
	log.Infof("Waiting for solochain message (nonce %d) to be potentially picked up by another relayer", solochainNonce)
	var cnt uint64
	var err error
	for {
		isRelayed, err := li.scanner.isNonceRelayed(ctx, uint64(solochainNonce))
		if err != nil {
			return fmt.Errorf("Checking if solochain nonce %d is relayed: %w", solochainNonce, err)
		}
		if isRelayed {
			log.Infof("Solochain message (nonce %d) picked up by another relayer, skipping", solochainNonce)
			return nil
		}
		if cnt == waitingPeriod {
			break
		}
		time.Sleep(time.Duration(li.scheduleConfig.SleepInterval) * time.Second)
		cnt++
	}
	log.Infof("Solochain message (nonce %d) not picked up by others, proceeding to submit", solochainNonce)

	task.ProofOutput, err = li.generateProof(ctx, task.ProofInput, task.Header)
	if err != nil {
		return fmt.Errorf("Generating proof for solochain message (nonce %d): %w", solochainNonce, err)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case li.tasks <- task:
		log.Info("Beefy Listener emitted new task")
	}
	return nil
}
