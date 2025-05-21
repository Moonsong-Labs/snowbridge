// Copyright 2020 Snowfork
// SPDX-License-Identifier: LGPL-3.0-only

package ethereum

import (
	"bytes"

	etypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	etrie "github.com/ethereum/go-ethereum/trie"
	"github.com/snowfork/go-substrate-rpc-client/v4/types"
	"github.com/snowfork/snowbridge/relayer/chain/solochain"
)

func MakeMessageFromEvent(event *etypes.Log, receiptsTrie *etrie.Trie) (*solochain.Message, error) {
	// RLP encode event log's Address, Topics, and Data
	var buf bytes.Buffer
	err := event.EncodeRLP(&buf)
	if err != nil {
		return nil, err
	}

	receiptKey, err := rlp.EncodeToBytes(event.TxIndex)
	if err != nil {
		return nil, err
	}

	proof := solochain.NewProofData()
	err = receiptsTrie.Prove(receiptKey, proof)
	if err != nil {
		return nil, err
	}

	var convertedTopics []types.H256
	for _, topic := range event.Topics {
		convertedTopics = append(convertedTopics, types.H256(topic))
	}

	m := solochain.Message{
		EventLog: solochain.EventLog{
			Address: types.H160(event.Address),
			Topics:  convertedTopics,
			Data:    event.Data,
		},
		Proof: solochain.Proof{
			ReceiptProof: proof,
		},
	}

	return &m, nil
}
