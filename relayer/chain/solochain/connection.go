// Copyright 2020 Snowfork
// SPDX-License-Identifier: LGPL-3.0-only

package solochain

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	gsrpc "github.com/snowfork/go-substrate-rpc-client/v4"
	"github.com/snowfork/go-substrate-rpc-client/v4/signature"
	"github.com/snowfork/go-substrate-rpc-client/v4/types"

	log "github.com/sirupsen/logrus"
)

type Connection struct {
	endpoint    string
	kp          *signature.KeyringPair
	api         *gsrpc.SubstrateAPI
	metadata    types.Metadata
	genesisHash types.Hash
}

func (co *Connection) API() *gsrpc.SubstrateAPI {
	return co.api
}

func (co *Connection) Metadata() *types.Metadata {
	return &co.metadata
}

func (co *Connection) Keypair() *signature.KeyringPair {
	return co.kp
}

func NewConnection(endpoint string, kp *signature.KeyringPair) *Connection {
	return &Connection{
		endpoint: endpoint,
		kp:       kp,
	}
}

func (co *Connection) Connect(_ context.Context) error {
	// Initialize API
	api, err := gsrpc.NewSubstrateAPI(co.endpoint)
	if err != nil {
		return err
	}
	co.api = api

	// Fetch metadata
	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return err
	}
	co.metadata = *meta

	// Fetch genesis hash
	genesisHash, err := api.RPC.Chain.GetBlockHash(0)
	if err != nil {
		return err
	}
	co.genesisHash = genesisHash

	log.WithFields(logrus.Fields{
		"endpoint":    co.endpoint,
		"metaVersion": meta.Version,
	}).Info("Connected to chain")

	return nil
}

func (co *Connection) ConnectWithHeartBeat(ctx context.Context, heartBeat time.Duration) error {
	err := co.Connect(ctx)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(heartBeat)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, err := co.API().RPC.System.Version()
				if err != nil {
					log.WithField("endpoint", co.endpoint).Error("Connection heartbeat failed")
					return
				}
			}
		}
	}()

	return nil
}

func (co *Connection) Close() {
	// TODO: Fix design issue in GSRPC preventing on-demand closing of connections
}

func (co *Connection) GenesisHash() types.Hash {
	return co.genesisHash
}

func (co *Connection) GetFinalizedHeader() (*types.Header, error) {
	finalizedHash, err := co.api.RPC.Chain.GetFinalizedHead()
	if err != nil {
		return nil, err
	}

	finalizedHeader, err := co.api.RPC.Chain.GetHeader(finalizedHash)
	if err != nil {
		return nil, err
	}

	return finalizedHeader, nil
}

func (co *Connection) GetLatestBlockNumber() (*types.BlockNumber, error) {
	latestBlock, err := co.api.RPC.Chain.GetBlockLatest()
	if err != nil {
		return nil, err
	}

	return &latestBlock.Block.Header.Number, nil
}

func (conn *Connection) GetMMRRootHash(blockHash types.Hash) (types.Hash, error) {
	mmrRootHashKey, err := types.CreateStorageKey(conn.Metadata(), "Mmr", "RootHash", nil, nil)
	if err != nil {
		return types.Hash{}, fmt.Errorf("create storage key: %w", err)
	}
	var mmrRootHash types.Hash
	ok, err := conn.API().RPC.State.GetStorage(mmrRootHashKey, &mmrRootHash, blockHash)
	if err != nil {
		return types.Hash{}, fmt.Errorf("query storage for mmr root hash at block %v: %w", blockHash.Hex(), err)
	}
	if !ok {
		return types.Hash{}, fmt.Errorf("Mmr.RootHash storage item does not exist")
	}
	return mmrRootHash, nil
}

func (conn *Connection) FetchMMRLeafCount(relayBlockhash types.Hash) (uint64, error) {
	mmrLeafCountKey, err := types.CreateStorageKey(conn.Metadata(), "Mmr", "NumberOfLeaves", nil, nil)
	if err != nil {
		return 0, err
	}
	var mmrLeafCount uint64

	ok, err := conn.API().RPC.State.GetStorage(mmrLeafCountKey, &mmrLeafCount, relayBlockhash)
	if err != nil {
		return 0, err
	}

	if !ok {
		return 0, fmt.Errorf("MMR Leaf Count Not Found")
	}

	log.WithFields(log.Fields{
		"mmrLeafCount": mmrLeafCount,
	}).Info("MMR Leaf Count")

	return mmrLeafCount, nil
}

func (conn *Connection) fetchKeys(keyPrefix []byte, blockHash types.Hash) ([]types.StorageKey, error) {
	const pageSize = 200
	var startKey *types.StorageKey

	if pageSize < 1 {
		return nil, fmt.Errorf("page size cannot be zero")
	}

	var results []types.StorageKey
	log.WithFields(log.Fields{
		"keyPrefix": keyPrefix,
		"blockHash": blockHash.Hex(),
		"pageSize":  pageSize,
	}).Trace("Fetching paged keys.")

	pageIndex := 0
	for {
		response, err := conn.API().RPC.State.GetKeysPaged(keyPrefix, pageSize, startKey, blockHash)
		if err != nil {
			return nil, err
		}

		log.WithFields(log.Fields{
			"keysInPage": len(response),
			"pageIndex":  pageIndex,
		}).Trace("Fetched a page of keys.")

		results = append(results, response...)
		if uint32(len(response)) < pageSize {
			break
		} else {
			startKey = &response[len(response)-1]
			pageIndex++
		}
	}

	log.WithFields(log.Fields{
		"totalNumKeys":  len(results),
		"totalNumPages": pageIndex + 1,
	}).Trace("Fetching of paged keys complete.")

	return results, nil
}

func (co *Connection) GenerateProofForBlock(
	blockNumber uint64,
	latestBeefyBlockHash types.Hash,
) (types.GenerateMMRProofResponse, error) {
	log.WithFields(log.Fields{
		"blockNumber": blockNumber,
		"blockHash":   latestBeefyBlockHash.Hex(),
	}).Debug("Getting MMR Leaf for block...")

	proofResponse, err := co.API().RPC.MMR.GenerateProof(uint32(blockNumber), latestBeefyBlockHash)
	if err != nil {
		return types.GenerateMMRProofResponse{}, err
	}

	var proofItemsHex = []string{}
	for _, item := range proofResponse.Proof.Items {
		proofItemsHex = append(proofItemsHex, item.Hex())
	}

	log.WithFields(log.Fields{
		"BlockHash": proofResponse.BlockHash.Hex(),
		"Leaf": log.Fields{
			"ParentNumber":    proofResponse.Leaf.ParentNumberAndHash.ParentNumber,
			"ParentHash":      proofResponse.Leaf.ParentNumberAndHash.Hash.Hex(),
			"BeefyExtraField": proofResponse.Leaf.BeefyExtraField.Hex(),
			"NextAuthoritySet": log.Fields{
				"Id":   proofResponse.Leaf.BeefyNextAuthoritySet.ID,
				"Len":  proofResponse.Leaf.BeefyNextAuthoritySet.Len,
				"Root": proofResponse.Leaf.BeefyNextAuthoritySet.Root.Hex(),
			},
		},
		"Proof": log.Fields{
			"LeafIndex": proofResponse.Proof.LeafIndex,
			"LeafCount": proofResponse.Proof.LeafCount,
			"Items":     proofItemsHex,
		},
	}).Debug("Generated MMR proof")

	return proofResponse, nil
}
