package solochain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/snowfork/go-substrate-rpc-client/v4/types"
	"github.com/snowfork/snowbridge/relayer/crypto/merkle"
)

// ByLeafIndex implements sort.Interface based on the LeafIndex field.
type ByOutboundMessage []OutboundQueueMessage

func (b ByOutboundMessage) Len() int           { return len(b) }
func (b ByOutboundMessage) Less(i, j int) bool { return b[i].Nonce < b[j].Nonce }
func (b ByOutboundMessage) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

type MerkleProofData struct {
	PreLeaves       PreLeaves `json:"preLeaves"`
	NumberOfLeaves  int       `json:"numberOfLeaves"`
	ProvenPreLeaf   HexBytes  `json:"provenPreLeaf"`
	ProvenLeaf      HexBytes  `json:"provenLeaf"`
	ProvenLeafIndex int64     `json:"provenLeafIndex"`
	Root            HexBytes  `json:"root"`
	Proof           Proof     `json:"proof"`
}

type PreLeaves [][]byte
type Proof [][32]byte
type HexBytes []byte

func (h HexBytes) MarshalJSON() ([]byte, error) {
	b, _ := json.Marshal("0x" + hex.EncodeToString(h))
	return b, nil
}

func (h HexBytes) String() string {
	b, _ := json.Marshal(h)
	return string(b)
}

func (h HexBytes) Hex() string {
	return "0x" + hex.EncodeToString(h)
}

func (d PreLeaves) MarshalJSON() ([]byte, error) {
	items := make([]string, 0, len(d))
	for _, v := range d {
		items = append(items, "0x"+hex.EncodeToString(v))
	}
	b, _ := json.Marshal(items)
	return b, nil
}

func (d Proof) MarshalJSON() ([]byte, error) {
	items := make([]string, 0, len(d))
	for _, v := range d {
		items = append(items, "0x"+hex.EncodeToString(v[:]))
	}
	b, _ := json.Marshal(items)
	return b, nil
}

func (d MerkleProofData) String() string {
	b, _ := json.Marshal(d)
	return string(b)
}

func CreateMessagesMerkleProof(messages []OutboundQueueMessage, messageNonce uint64) (MerkleProofData, error) {
	// Sort slice by message nonce
	sort.Sort(ByOutboundMessage(messages))

	// Loop messages, convert to pre leaves and find message being proven
	preLeaves := make([][]byte, 0, len(messages))
	var messageToProve []byte
	var messageIndex int64
	for i, message := range messages {
		preLeaf, err := types.EncodeToBytes(message)
		if err != nil {
			return MerkleProofData{}, err
		}
		preLeaves = append(preLeaves, preLeaf)
		if uint64(message.Nonce) == messageNonce {
			messageToProve = preLeaf
			messageIndex = int64(i)
		}
	}

	// Reference implementation of MerkleTree in substrate
	// https://github.com/paritytech/substrate/blob/ea387c634715793f806286abf1e64cabf9b7026f/frame/beefy-mmr/primitives/src/lib.rs#L45-L54
	leaf, root, proof, err := merkle.GenerateMerkleProof(preLeaves, messageIndex)
	if err != nil {
		return MerkleProofData{}, fmt.Errorf("create parachain merkle proof: %w", err)
	}

	return MerkleProofData{
		PreLeaves:       preLeaves,
		NumberOfLeaves:  len(preLeaves),
		ProvenPreLeaf:   messageToProve,
		ProvenLeaf:      leaf,
		ProvenLeafIndex: messageIndex,
		Root:            root,
		Proof:           proof,
	}, nil
}
