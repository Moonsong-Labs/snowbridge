package solochain

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/snowfork/go-substrate-rpc-client/v4/types"
	"github.com/snowfork/snowbridge/relayer/chain/ethereum"
	"github.com/snowfork/snowbridge/relayer/chain/parachain"
	"github.com/snowfork/snowbridge/relayer/contracts"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/header"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/header/syncer/scale"
	"github.com/snowfork/snowbridge/relayer/relays/util"
	"golang.org/x/sync/errgroup"
)

// startDeliverProof starts a goroutine that delivers proofs for undelivered orders on solochain
func (relay *Relay) startDeliverProof(ctx context.Context, eg *errgroup.Group) error {
	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(60 * time.Second):
				orders, err := relay.beefyListener.scanner.findOrderUndelivered(ctx)
				if err != nil {
					return fmt.Errorf("Error finding undelivered orders on solochain: %v", err)
				}
				rewardAddress, err := util.HexStringTo32Bytes(relay.config.RewardAddress)
				if err != nil {
					return fmt.Errorf("Converting reward address string to address: %w", err)
				}
				for _, order := range orders {
					event, err := relay.findEvent(ctx, order.Nonce)
					if err != nil {
						return fmt.Errorf("Error finding GatewayInboundMessageDispatched event for nonce %d: %v", order.Nonce, err)
					}
					if event.RewardAddress != rewardAddress {
						log.Infof("Order nonce %d not relayed by this relayer (event reward address %s vs relayer reward address %s), skipping delivery proof.",
							order.Nonce,
							types.HexEncodeToString(event.RewardAddress[:]),
							types.HexEncodeToString(rewardAddress[:]))
						continue
					}
					err = relay.doSubmitDeliveryProof(ctx, event)
					if err != nil {
						return fmt.Errorf("Error submitting delivery proof for nonce %d to solochain: %v", order.Nonce, err)
					}
				}
			}
		}
	})
	return nil
}

// findEvent finds the GatewayInboundMessageDispatched event for a given nonce
func (relay *Relay) findEvent(
	ctx context.Context,
	nonce uint64,
) (*contracts.GatewayInboundMessageDispatched, error) {

	const BlocksPerQuery = 4096

	var event *contracts.GatewayInboundMessageDispatched

	// Get the latest block number from Ethereum
	blockNumber, err := relay.ethereumConnWriter.Client().BlockNumber(ctx)
	if err != nil {
		return event, fmt.Errorf("Getting last ethereum block number: %w", err)
	}

	// Search for the event in the last BlocksPerQuery blocks
	done := false

	// Start from the latest block and search backwards
	for {
		var begin uint64
		if blockNumber < BlocksPerQuery {
			begin = 0
		} else {
			begin = blockNumber - BlocksPerQuery
		}

		opts := bind.FilterOpts{
			Start:   begin,
			End:     &blockNumber,
			Context: ctx,
		}

		iter, err := relay.ethereumChannelWriter.gateway.FilterInboundMessageDispatched(&opts, []uint64{nonce})
		if err != nil {
			return event, fmt.Errorf("iter dispatch event: %w", err)
		}

		for {
			more := iter.Next()
			if !more {
				err = iter.Error()
				if err != nil {
					return event, fmt.Errorf("iter dispatch event: %w", err)
				}
				break
			}
			if iter.Event.Nonce == nonce {
				event = iter.Event
				done = true
				break
			}
		}

		if done {
			iter.Close()
		}

		blockNumber = begin

		if done || begin == 0 {
			break
		}
	}

	return event, nil
}

// makeInboundMessage creates a solochain message from an Ethereum event
func (relay *Relay) makeInboundMessage(
	ctx context.Context,
	headerCache *ethereum.HeaderCache,
	event *contracts.GatewayInboundMessageDispatched,
) (*parachain.Message, error) {
	receiptTrie, err := headerCache.GetReceiptTrie(ctx, event.Raw.BlockHash)
	if err != nil {
		log.WithFields(logrus.Fields{
			"blockHash":   event.Raw.BlockHash.Hex(),
			"blockNumber": event.Raw.BlockNumber,
			"txHash":      event.Raw.TxHash.Hex(),
		}).WithError(err).Error("Failed to get receipt trie for event")
		return nil, err
	}

	msg, err := ethereum.MakeMessageFromEvent(&event.Raw, receiptTrie)
	if err != nil {
		log.WithFields(logrus.Fields{
			"address":     event.Raw.Address.Hex(),
			"blockHash":   event.Raw.BlockHash.Hex(),
			"blockNumber": event.Raw.BlockNumber,
			"txHash":      event.Raw.TxHash.Hex(),
		}).WithError(err).Error("Failed to generate message from ethereum event")
		return nil, err
	}

	log.WithFields(logrus.Fields{
		"blockHash":   event.Raw.BlockHash.Hex(),
		"blockNumber": event.Raw.BlockNumber,
		"txHash":      event.Raw.TxHash.Hex(),
	}).Info("found message")

	return msg, nil
}

// doSubmitDeliveryProof submits a delivery proof to solochain
func (relay *Relay) doSubmitDeliveryProof(ctx context.Context, ev *contracts.GatewayInboundMessageDispatched) error {
	// Make an inbound message from the Ethereum event
	inboundMsg, err := relay.makeInboundMessage(ctx, relay.headerCache, ev)
	if err != nil {
		return fmt.Errorf("Making inbound message for solochain delivery receipt: %w", err)
	}

	logger := log.WithFields(log.Fields{
		"ethNonce":    ev.Nonce,
		"msgNonce":    ev.Nonce,
		"address":     ev.Raw.Address.Hex(),
		"blockHash":   ev.Raw.BlockHash.Hex(),
		"blockNumber": ev.Raw.BlockNumber,
		"txHash":      ev.Raw.TxHash.Hex(),
		"txIndex":     ev.Raw.TxIndex,
	})

	// Get the next block header
	nextBlockNumber := new(big.Int).SetUint64(ev.Raw.BlockNumber + 1)
	blockHeader, err := relay.ethereumConnWriter.Client().HeaderByNumber(ctx, nextBlockNumber)
	if err != nil {
		return fmt.Errorf("Getting ethereum block header %d: %w", nextBlockNumber, err)
	}

	// Fetch the beacon proof of the event block
	beaconProof, err := relay.beaconHeader.FetchExecutionProof(*blockHeader.ParentBeaconRoot, false)
	if errors.Is(err, header.ErrBeaconHeaderNotFinalized) || beaconProof.HeaderPayload.ExecutionBranch == nil {
		logger.Infof("Ethereum event block %d is not yet finalized on Beacon chain for delivery proof. Will retry later.", ev.Raw.BlockNumber)
		return nil
	}
	if err != nil {
		return fmt.Errorf("Fetching ethereum execution header proof: %w", err)
	}

	err = relay.writeToSolochain(ctx, beaconProof, inboundMsg)
	if err != nil {
		return fmt.Errorf("Writing delivery receipt to solochain: %w", err)
	}

	logger.WithFields(log.Fields{"nonce": ev.Nonce}).Info("V2 delivery receipt for message submitted successfully to solochain.")

	return nil
}

// writeToSolochain writes a delivery receipt to solochain
func (relay *Relay) writeToSolochain(ctx context.Context, proof scale.ProofPayload, inboundMsg *parachain.Message) error {
	inboundMsg.Proof.ExecutionProof = proof.HeaderPayload

	log.WithFields(logrus.Fields{
		"EventLog": inboundMsg.EventLog,
		"Proof":    inboundMsg.Proof,
	}).Debug("Generated message from Ethereum log")

	err := relay.solochainWriter.WriteToParachainAndWatch(ctx, "EthereumOutboundQueueV2.submit_delivery_receipt", inboundMsg)
	if err != nil {
		return fmt.Errorf("Submitting delivery receipt to solochain outbound queue v2: %w", err)
	}

	return nil
}
