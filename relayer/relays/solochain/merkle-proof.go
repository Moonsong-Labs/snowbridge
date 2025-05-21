package solochain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ethereum/go-ethereum/accounts/abi"
	log "github.com/sirupsen/logrus"

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

// --- ABI Encoding Structs and Setup ---

// AbiCommandWrapper corresponds to Solidity:
//
//	struct CommandWrapper {
//	    uint8 kind;
//	    uint64 gas;
//	    bytes payload;
//	}
type AbiCommandWrapper struct {
	Kind    uint8
	Gas     uint64
	Payload []byte
}

// AbiOutboundMessageWrapper corresponds to Solidity:
//
//	struct OutboundMessageWrapper {
//	    bytes32 origin;
//	    uint64 nonce;
//	    bytes32 topic;
//	    CommandWrapper[] commands;
//	}
type AbiOutboundMessageWrapper struct {
	Origin   [32]byte // types.H256 is [32]byte
	Nonce    uint64   // types.U64 is uint64
	Topic    [32]byte // types.H256 is [32]byte
	Commands []AbiCommandWrapper
}

var (
	// finalABIArguments is used to pack the AbiOutboundMessageWrapper struct.
	// It's configured to pack a single argument of the OutboundMessageWrapper tuple type.
	finalABIArguments abi.Arguments
)

func init() {
	// Define ABI type for CommandWrapper: (uint8,uint64,bytes)
	commandComponents := []abi.ArgumentMarshaling{
		{Name: "kind", Type: "uint8"},
		{Name: "gas", Type: "uint64"},
		{Name: "payload", Type: "bytes"},
	}

	// Define ABI type for OutboundMessageWrapper: (bytes32,uint64,bytes32,CommandWrapper[])
	// The "commands" field is an array of the CommandWrapper tuple.
	// InternalType "CommandWrapper[]" is for easier debugging/reflection if needed by some tools,
	// Type "tuple[]" with Components defines the structure for ABI encoding.
	outboundMessageWrapperComponents := []abi.ArgumentMarshaling{
		{Name: "origin", Type: "bytes32"},
		{Name: "nonce", Type: "uint64"},
		{Name: "topic", Type: "bytes32"},
		{Name: "commands", Type: "tuple[]", Components: commandComponents, InternalType: "CommandWrapper[]"},
	}

	// Create the ABI type for the OutboundMessageWrapper struct itself
	outboundMessageWrapperABIType, err := abi.NewType("tuple", "OutboundMessageWrapper", outboundMessageWrapperComponents)
	if err != nil {
		log.Fatalf("Failed to create OutboundMessageWrapper ABI type: %v", err)
	}

	// finalABIArguments will be used to pack a single argument of type OutboundMessageWrapper
	finalABIArguments = abi.Arguments{{Type: outboundMessageWrapperABIType, Name: "message"}}
}

// --- End ABI Encoding Structs and Setup ---

func CreateMessagesMerkleProof(messages []OutboundQueueMessage, messageNonce uint64) (MerkleProofData, error) {
	log.Debugf("Sorting messages and creating merkle proof for message nonce %d", messageNonce)

	// Sort slice by message nonce (TODO: Sort probably not needed since messages are appeneded in order)
	sort.Sort(ByOutboundMessage(messages))

	// Loop messages, convert to pre leaves and find message being proven
	preLeaves := make([][]byte, 0, len(messages))
	var messageToProve []byte
	var messageIndex int64
	for i, message := range messages {
		log.Debugf("Processing message at index %d: %v", i, message)

		abiCommands := make([]AbiCommandWrapper, len(message.Commands))
		for j, cmd := range message.Commands {
			abiCommands[j] = AbiCommandWrapper{
				Kind:    uint8(cmd.Kind),
				Gas:     uint64(cmd.MaxDispatchGas),
				Payload: cmd.Params,
			}
		}

		abiWrapper := AbiOutboundMessageWrapper{
			Origin:   message.Origin,
			Nonce:    uint64(message.Nonce),
			Topic:    message.Topic,
			Commands: abiCommands,
		}

		preLeaf, err := finalABIArguments.Pack(abiWrapper)
		if err != nil {
			return MerkleProofData{}, fmt.Errorf("failed to ABI-encode message (nonce %d, index %d): %w", message.Nonce, i, err)
		}

		preLeaves = append(preLeaves, preLeaf)
		if uint64(message.Nonce) == messageNonce {
			log.Debugf("Message to prove found! Index: %d, Nonce: %d. ABI Encoded ProvenPreLeaf will be used.", i, message.Nonce)
			messageToProve = preLeaf
			messageIndex = int64(i)
		}
	}

	// Reference implementation of MerkleTree in substrate
	// https://github.com/paritytech/substrate/blob/ea387c634715793f806286abf1e64cabf9b7026f/frame/beefy-mmr/primitives/src/lib.rs#L45-L54
	leaf, root, proof, err := merkle.GenerateMerkleProof(preLeaves, messageIndex)
	if err != nil {
		return MerkleProofData{}, fmt.Errorf("create solochain messages merkle proof: %w", err)
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
