package solochain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/snowfork/go-substrate-rpc-client/v4/scale"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"
	gsrpc "github.com/snowfork/go-substrate-rpc-client/v4"
	"github.com/snowfork/go-substrate-rpc-client/v4/types"
	"github.com/snowfork/snowbridge/relayer/chain/ethereum"
	"github.com/snowfork/snowbridge/relayer/chain/parachain"
	"github.com/snowfork/snowbridge/relayer/contracts"
	"github.com/snowfork/snowbridge/relayer/ofac"
)

type Scanner struct {
	config   *SourceConfig
	ethConn  *ethereum.Connection
	soloConn *parachain.Connection
	ofac     *ofac.OFAC
	tasks    chan<- *Task
}

// Scans for all solochain message commitments that need to be relayed and can be
// proven using the MMR root at the specified beefyBlockNumber of the solochain.
// The algorithm fetchs the PendingOrders storage in the OutboundQueue pallet and
// relays each order which has not been processed on Ethereum yet.
func (s *Scanner) Scan(ctx context.Context, beefyBlockNumber uint64) ([]*Task, error) {
	log.Infof("Scanner starting scan for messages up to solochain block %d", beefyBlockNumber)

	// Fetch the last block for which we can prove its message using the MMR root at beefyBlockNumber
	beefyBlockMinusOneHash, err := s.soloConn.API().RPC.Chain.GetBlockHash(uint64(beefyBlockNumber - 1))
	if err != nil {
		return nil, fmt.Errorf("fetch block hash for block %v: %w", beefyBlockNumber, err)
	}

	// Find all message commitments in the corresponding block which need to be relayed
	tasks, err := s.findTasks(ctx, beefyBlockMinusOneHash)
	if err != nil {
		return nil, fmt.Errorf("Finding tasks on solochain: %w", err)
	}

	log.Infof("Scanner found %d potential tasks to process with MMR root of solochain block %d", len(tasks), beefyBlockNumber)
	return tasks, nil
}

// findTasks finds all the messages which need to be relayed from the solochain at the corresponding block.
func (s *Scanner) findTasks(
	ctx context.Context,
	solochainBlockHash types.Hash,
) ([]*Task, error) {
	// Fetch PendingOrders storage in the solochain outbound queue at the corresponding block
	storageKeyPrefix := types.NewStorageKey(types.CreateStorageKeyPrefix("EthereumOutboundQueueV2", "PendingOrders"))
	keys, err := s.soloConn.API().RPC.State.GetKeys(storageKeyPrefix, solochainBlockHash)
	if err != nil {
		return nil, fmt.Errorf("Fetching nonces from PendingOrders at blockhash '%v': %w", solochainBlockHash, err)
	}

	var pendingOrders []PendingOrder

	// Iterate over the pending order nonces
	for _, key := range keys {
		var pendingOrder PendingOrder

		// Get the corresponding pending order from storage
		value, err := s.soloConn.API().RPC.State.GetStorageRaw(key, solochainBlockHash)
		if err != nil {
			return nil, fmt.Errorf("Failed to fetch value for PendingOrder with nonce '%v' at blockhash '%v': %w", key, solochainBlockHash, err)
		}
		if value == nil || len(*value) == 0 {
			return nil, fmt.Errorf("Empty value for PendingOrder with nonce '%v' at blockhash '%v'", key, solochainBlockHash)
		}

		// Decode the pending order
		decoder := scale.NewDecoder(bytes.NewReader(*value))
		err = decoder.Decode(&pendingOrder)
		if err != nil {
			return nil, fmt.Errorf("Failed to decode PendingOrder with nonce '%v' at blockhash '%v': %w", key, solochainBlockHash, err)
		}
		pendingOrders = append(pendingOrders, pendingOrder)
	}

	// Filter pending orders so only profitable and undelivered orders remain, and convert them to tasks
	tasks, err := s.filterTasks(
		ctx,
		pendingOrders,
	)
	if err != nil {
		return nil, err
	}

	// Gather the neccessary proof inputs for each task
	err = s.gatherProofInputs(tasks)
	if err != nil {
		return nil, fmt.Errorf("Gathering proof inputs for tasks: %w", err)
	}

	return tasks, nil
}

// filterTasks filters profitable and undelivered orders and converts them to tasks.
// Todo: check order is profitable or not with some price oracle
// or some fee estimation api
func (s *Scanner) filterTasks(
	ctx context.Context,
	pendingOrders []PendingOrder,
) ([]*Task, error) {
	var tasks []*Task

	// Iterate over the pending orders
	for _, order := range pendingOrders {
		// Check if the order has already been relayed
		isRelayed, err := s.isNonceRelayed(ctx, uint64(order.Nonce))
		if err != nil {
			return nil, fmt.Errorf("Checking if nonce is relayed: %w", err)
		}
		if isRelayed {
			log.WithFields(log.Fields{
				"nonce": uint64(order.Nonce),
			}).Debug("Message already relayed, skipping")
			continue
		}

		// Fetch the header for the solochain block when this order was issued
		orderBlockNumber := uint64(order.BlockNumber)

		log.WithFields(log.Fields{
			"blockNumber": orderBlockNumber,
		}).Debug("Checking header")

		blockHash, err := s.soloConn.API().RPC.Chain.GetBlockHash(orderBlockNumber)
		if err != nil {
			return nil, fmt.Errorf("Fetching block hash for block %v: %w", orderBlockNumber, err)

		}

		header, err := s.soloConn.API().RPC.Chain.GetHeader(blockHash)
		if err != nil {
			return nil, fmt.Errorf("Fetching header for block hash %v: %w", blockHash.Hex(), err)
		}

		commitmentHash, err := ExtractCommitmentFromDigest(header.Digest)
		if err != nil {
			return nil, fmt.Errorf("Failed to extract commitment from digest for block %d (order nonce %d): %v", orderBlockNumber, order.Nonce, err)
		}
		if commitmentHash == nil {
			log.Debugf("No commitment found in digest for block %d (order nonce %d), skipping", orderBlockNumber, order.Nonce)
			continue
		}

		// Create the storage key to be able to get the messages from the EthereumOutboundQueueV2 pallet
		messagesKey, err := types.CreateStorageKey(s.soloConn.Metadata(), "EthereumOutboundQueueV2", "Messages", nil, nil)
		if err != nil {
			return nil, fmt.Errorf("Creating storage key for Messages of EthereumOutboundQueueV2: %w", err)
		}

		// Get the messages in the corresponding block
		var messagesInBlock []OutboundQueueMessage
		rawMessages, err := s.soloConn.API().RPC.State.GetStorageRaw(messagesKey, blockHash)
		if err != nil {
			return nil, fmt.Errorf("Fetching committed messages for block %v: %w", blockHash.Hex(), err)
		}
		if rawMessages == nil || len(*rawMessages) == 0 {
			log.Debugf("No messages found in EthereumOutboundQueueV2.Messages for solochain block %v (order nonce %d), skipping", blockHash.Hex(), order.Nonce)
			continue
		}

		// Prepare the message decoder and decode the number of messages
		messagesDecoder := scale.NewDecoder(bytes.NewReader(*rawMessages))
		numMessagesCompact, err := messagesDecoder.DecodeUintCompact()
		if err != nil {
			return nil, fmt.Errorf("Decoding message length from block '%v': %w", blockHash.Hex(), err)
		}
		numMessages := numMessagesCompact.Uint64()

		// Iterate over all messages present in the block, decoding them and checking if they contain banned addresses
		for i := uint64(0); i < numMessages; i++ {
			m := OutboundQueueMessage{}
			err = messagesDecoder.Decode(&m)
			if err != nil {
				return nil, fmt.Errorf("Decoding message at block '%v': %w", blockHash.Hex(), err)
			}

			// Check OFAC
			isBanned, err := s.IsBanned(m)
			if err != nil {
				log.WithError(err).Fatal("Error checking if address is banned")
				return nil, fmt.Errorf("Banned check: %w", err)
			}
			if isBanned {
				log.Fatal("Banned address found")
				return nil, errors.New("Banned address found")
			}
			messagesInBlock = append(messagesInBlock, m)
		}

		// For the outbound channel, the commitment hash is the merkle root of the messages
		// https://github.com/Snowfork/snowbridge/blob/75a475cbf8fc8e13577ad6b773ac452b2bf82fbb/parachain/pallets/basic-channel/src/outbound/mod.rs#L275-L277
		// To verify it we fetch the message proof from the solochain
		proofResult, err := scanForOutboundQueueProofs(
			s.soloConn.API(),
			blockHash,
			*commitmentHash,
			messagesInBlock,
		)
		if err != nil {
			return nil, fmt.Errorf("Failed to scan for outbound queue proofs for block %v (order nonce %d): %v", blockHash.Hex(), order.Nonce, err)
		}

		// If we get a proof after scanning, we can set up the corresponding task to relay the order
		if len(proofResult.proofs) > 0 {

			task := Task{
				Header:        header,
				MessageProofs: &proofResult.proofs,
				ProofInput:    nil,
			}
			tasks = append(tasks, &task)
			log.Infof("Created task for message nonce %d from block %d", order.Nonce, orderBlockNumber)

		} else {
			log.Debugf("No proofs generated by scanForOutboundQueueProofs for solochain block %v (commitment %s), skipping order nonce %d.", blockHash.Hex(), commitmentHash.Hex(), order.Nonce)
		}
	}

	return tasks, nil
}

// gatherProofInputs will search to find the messages present in the block that corresponds
// to the block number of the task and populate the ProofInput for each one.
func (s *Scanner) gatherProofInputs(
	tasks []*Task,
) error {
	// Iterate over the tasks
	for _, task := range tasks {

		log.WithFields(log.Fields{
			"BlockNumber": task.Header.Number,
		}).Debug("Gathering proof inputs for block")

		solochainBlockNumber := uint64(task.Header.Number)

		// Fetch the block hash of the solochain block
		solochainBlockHash, err := s.soloConn.API().RPC.Chain.GetBlockHash(solochainBlockNumber)
		if err != nil {
			return fmt.Errorf("Failed to get block hash for message block %d: %v", solochainBlockNumber, err)
		}

		// Create the storage key to be able to get the messages from the EthereumOutboundQueueV2 pallet
		messagesKey, err := types.CreateStorageKey(s.soloConn.Metadata(), "EthereumOutboundQueueV2", "Messages", nil, nil)
		if err != nil {
			return fmt.Errorf("Creating storage key for Messages of EthereumOutboundQueueV2: %w", err)
		}

		// Get the messages in the corresponding block
		var messagesInBlock []OutboundQueueMessage
		rawMessages, err := s.soloConn.API().RPC.State.GetStorageRaw(messagesKey, solochainBlockHash)
		if err != nil {
			return fmt.Errorf("Fetching committed messages for block %v: %w", solochainBlockHash.Hex(), err)
		}
		if rawMessages == nil || len(*rawMessages) == 0 {
			return fmt.Errorf("No messages found in EthereumOutboundQueueV2.Messages for solochain block %v", solochainBlockHash.Hex())
		}

		// Decode the messages
		messagesDecoder := scale.NewDecoder(bytes.NewReader(*rawMessages))
		err = messagesDecoder.Decode(&messagesInBlock)
		if err != nil {
			return fmt.Errorf("Failed to decode messages for block %v: %w", solochainBlockHash.Hex(), err)
		}

		log.WithFields(log.Fields{
			"SolochainBlockNumber": solochainBlockNumber,
			"SolochainBlockHash":   solochainBlockHash.Hex(),
			"Messages":             messagesInBlock,
		}).Debug("Gathered proof inputs for task")

		task.ProofInput = &ProofInput{
			SolochainBlockNumber: solochainBlockNumber,
			SolochainBlockHash:   solochainBlockHash,
			Messages:             messagesInBlock,
			MessageNonce:         uint64((*task.MessageProofs)[0].Message.Nonce),
		}
	}

	return nil
}

// scanForOutboundQueueProofs will fetch the message proofs for the received messages,
// check that the proof root matches the commitment hash and return the proofs.
func scanForOutboundQueueProofs(
	api *gsrpc.SubstrateAPI,
	blockHash types.Hash,
	commitmentHash types.H256,
	messages []OutboundQueueMessage,
) (*struct {
	proofs []MessageProof
}, error) {
	proofs := []MessageProof{}

	// Iterate over the messages
	for i, message := range messages {
		messageProof, err := fetchMessageProof(api, blockHash, uint64(i), message)
		if err != nil {
			return nil, fmt.Errorf("Failed to fetch message proof for index %d in block %s: %v", i, blockHash.Hex(), err)
		}
		// Check that the merkle root in the proof matches the commitment hash
		if messageProof.Proof.Root != commitmentHash {
			return nil, fmt.Errorf(
				"Halting scan:Outbound queue proof root '%v' for message index %d doesn't match commitment hash '%v' in block %s",
				messageProof.Proof.Root.Hex(),
				i,
				commitmentHash.Hex(),
				blockHash.Hex(),
			)
		}
		// Collect these proofs
		proofs = append(proofs, messageProof)
	}

	return &struct {
		proofs []MessageProof
	}{
		proofs: proofs,
	}, nil
}

// fetchMessageProof will fetch the message proof for the given message at message index from the given block hash.
func fetchMessageProof(
	api *gsrpc.SubstrateAPI,
	blockHash types.Hash,
	messageIndex uint64,
	message OutboundQueueMessage,
) (MessageProof, error) {
	var proofHex string
	var proof MessageProof

	// The parameter for OutboundQueueV2Api_prove_message is the message index.
	paramsEncoded, err := types.EncodeToHexString(messageIndex)
	if err != nil {
		return proof, fmt.Errorf("Encoding messageIndex param for prove_message: %w", err)
	}

	// Call the runtime API OutboundQueueV2Api_prove_message with the message index and block hash to get the proof
	err = api.Client.Call(&proofHex, "state_call", "OutboundQueueV2Api_prove_message", paramsEncoded, blockHash.Hex())
	if err != nil {
		return proof, fmt.Errorf("RPC call to OutboundQueueV2Api_prove_message(index: %v, block: %v): %w", messageIndex, blockHash.Hex(), err)
	}

	// Decode the proof from the response
	var optionRawMerkleProof OptionRawMerkleProof
	err = types.DecodeFromHexString(proofHex, &optionRawMerkleProof)
	if err != nil {
		return proof, fmt.Errorf("Decoding merkle proof from prove_message response: %w", err)
	}

	if !optionRawMerkleProof.HasValue {
		return proof, fmt.Errorf("OutboundQueueV2Api_prove_message returned no proof for index %v in block %v", messageIndex, blockHash.Hex())
	}

	merkleProof, err := NewMerkleProof(optionRawMerkleProof.Value)
	if err != nil {
		return proof, fmt.Errorf("Decoding RawMerkleProof to MerkleProof: %w", err)
	}

	return MessageProof{Message: message, Proof: merkleProof}, nil
}

// isNonceRelayed checks if the given nonce has been relayed to Ethereum already
func (s *Scanner) isNonceRelayed(ctx context.Context, nonce uint64) (bool, error) {
	var isRelayed bool
	gatewayAddress := common.HexToAddress(s.config.Contracts.Gateway)
	gatewayContract, err := contracts.NewGateway(
		gatewayAddress,
		s.ethConn.Client(),
	)
	if err != nil {
		return isRelayed, fmt.Errorf("create gateway contract for address '%v': %w", gatewayAddress, err)
	}

	options := bind.CallOpts{
		Pending: true,
		Context: ctx,
	}
	isRelayed, err = gatewayContract.V2IsDispatched(&options, nonce)
	if err != nil {
		return isRelayed, fmt.Errorf("Checking if nonce %d is relayed to Ethereum from gateway contract: %w", nonce, err)
	}
	return isRelayed, nil
}

// findOrderUndelivered finds orders that were relayed (V2IsDispatched is true on Ethereum)
// but for which a delivery receipt might not have been submitted back to the solochain.
func (s *Scanner) findOrderUndelivered(
	ctx context.Context,
) ([]*PendingOrder, error) {
	// Get all pending orders at the latest block
	storageKeyPrefix := types.NewStorageKey(types.CreateStorageKeyPrefix("EthereumOutboundQueueV2", "PendingOrders"))
	keys, err := s.soloConn.API().RPC.State.GetKeysLatest(storageKeyPrefix)
	if err != nil {
		return nil, fmt.Errorf("Fetching nonces from PendingOrders for undelivered check: %w", err)
	}

	// Iterate over the pending orders
	var undeliveredOrders []*PendingOrder
	for _, key := range keys {
		var undeliveredOrder PendingOrder
		// Get the pending order from storage
		value, err := s.soloConn.API().RPC.State.GetStorageRawLatest(key)
		if err != nil {
			return nil, fmt.Errorf("Failed to fetch value of PendingOrder with nonce '%v' for undelivered check: %v", key, err)
		}

		// Decode it
		decoder := scale.NewDecoder(bytes.NewReader(*value))
		err = decoder.Decode(&undeliveredOrder)
		if err != nil {
			return nil, fmt.Errorf("Failed to decode PendingOrder with nonce '%v' for undelivered check: %v", key, err)
		}

		// Check if the order has been relayed
		isRelayed, err := s.isNonceRelayed(ctx, uint64(undeliveredOrder.Nonce))
		if err != nil {
			return nil, fmt.Errorf("Error checking if nonce %d is relayed for undelivered order check: %v", undeliveredOrder.Nonce, err)
		}
		if isRelayed {
			// If it's been relayed to Ethereum, it's a candidate for needing a delivery receipt on solochain.
			log.WithFields(log.Fields{
				"nonce": uint64(undeliveredOrder.Nonce),
			}).Debug("Order nonce is relayed on Ethereum, needs delivery receipt on solochain.")
			undeliveredOrders = append(undeliveredOrders, &undeliveredOrder)
		}
	}
	return undeliveredOrders, nil
}

// isBanned checks if the message contains any banned addresses
func (s *Scanner) IsBanned(m OutboundQueueMessage) (bool, error) {
	destinations, err := GetDestinations(m)
	if err != nil {
		return true, err
	}
	var isBanned bool
	for _, destination := range destinations {
		isBanned, err = s.ofac.IsBanned("", destination)
		if isBanned || err != nil {
			return true, err
		}
	}
	return false, nil
}

// GetDestinations extracts the destination addresses from the message commands
func GetDestinations(message OutboundQueueMessage) ([]string, error) {
	var destinations []string
	log.WithFields(log.Fields{
		"commands": message.Commands,
	}).Debug("Checking message for OFAC")

	address := ""

	bytes32Ty, _ := abi.NewType("bytes32", "", nil)
	addressTy, _ := abi.NewType("address", "", nil)
	uint256Ty, _ := abi.NewType("uint256", "", nil)
	for _, command := range message.Commands {
		switch command.Kind {
		case 2:
			log.Debug("Unlock native token")

			transferTokenArguments := abi.Arguments{{Type: addressTy}, {Type: addressTy}, {Type: uint256Ty}}
			decodedTransferToken, err := transferTokenArguments.Unpack(command.Params)
			if err != nil {
				return destinations, fmt.Errorf("OFAC: unpack UnlockNative params: %w", err)
			}
			if len(decodedTransferToken) < 3 {
				return destinations, errors.New("OFAC: not enough params for UnlockNative to find recipient")
			}
			addressValue := decodedTransferToken[1].(common.Address)
			address = addressValue.String()
		case 4:
			log.Debug("Found MintForeignToken message")

			arguments := abi.Arguments{{Type: bytes32Ty}, {Type: addressTy}, {Type: uint256Ty}}
			decodedMessage, err := arguments.Unpack(command.Params)
			if err != nil {
				return destinations, fmt.Errorf("OFAC: unpack MintForeignToken params: %w", err)
			}
			if len(decodedMessage) < 3 {
				return destinations, fmt.Errorf("OFAC: not enough params for MintForeignToken to find recipient")
			}

			addressValue := decodedMessage[1].(common.Address)
			address = addressValue.String()
		}

		destination := strings.ToLower(address)

		log.WithField("destination", destination).Debug("extracted destination from message")

		destinations = append(destinations, destination)
	}

	return destinations, nil
}
