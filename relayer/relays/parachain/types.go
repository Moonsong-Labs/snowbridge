package solochain

import (
	"math/big"

	"github.com/snowfork/go-substrate-rpc-client/v4/scale"
	"github.com/snowfork/go-substrate-rpc-client/v4/types"
	"github.com/snowfork/snowbridge/relayer/contracts"
	"github.com/snowfork/snowbridge/relayer/crypto/merkle"
)

// A Task contains the working state for message commitments in a single solochain block
type Task struct {
	// Solochain header
	Header *types.Header
	// Inputs for MMR proof generation
	ProofInput *ProofInput
	// Outputs of MMR proof generation
	ProofOutput *ProofOutput
	// Proofs for messages from outbound channel on the solochain
	MessageProofs *[]MessageProof
}

// A ProofInput is data needed to generate a proof of a message commitments root
type ProofInput struct {
	// Solochain block number
	SolochainBlockNumber uint64
	// Solochain block hash
	SolochainBlockHash types.Hash
	// Messages to be proven
	Messages []OutboundQueueMessage
	// Message nonce
	MessageNonce uint64
}

// A ProofOutput represents the generated message proof
type ProofOutput struct {
	MMRProof        merkle.SimplifiedMMRProof
	MMRRootHash     types.Hash
	Header          types.Header
	MerkleProofData MerkleProofData
}

type OptionRawMerkleProof struct {
	HasValue bool
	Value    RawMerkleProof
}

func (o OptionRawMerkleProof) Encode(encoder scale.Encoder) error {
	return encoder.EncodeOption(o.HasValue, o.Value)
}

func (o *OptionRawMerkleProof) Decode(decoder scale.Decoder) error {
	return decoder.DecodeOption(&o.HasValue, &o.Value)
}

type RawMerkleProof struct {
	Root           types.H256
	Proof          []types.H256
	NumberOfLeaves uint64
	LeafIndex      uint64
	Leaf           types.H256
}

type MerkleProof struct {
	Root        types.H256
	InnerHashes [][32]byte
}

func NewMerkleProof(rawProof RawMerkleProof) (MerkleProof, error) {
	var proof MerkleProof

	byteArrayProof := make([][32]byte, len(rawProof.Proof))
	for i := 0; i < len(rawProof.Proof); i++ {
		byteArrayProof[i] = ([32]byte)(rawProof.Proof[i])
	}

	proof = MerkleProof{
		Root:        rawProof.Root,
		InnerHashes: byteArrayProof,
	}

	return proof, nil
}

type OutboundQueueMessage struct {
	Origin   types.H256
	Nonce    types.U64
	Topic    types.H256
	Commands []CommandWrapper
}

type OutboundQueueMessageWithFee struct {
	OriginalMessage OutboundQueueMessage
	// Attached fee in Ether
	Fee big.Int
}

type CommandWrapper struct {
	Kind           types.U8
	MaxDispatchGas types.U64
	Params         types.Bytes
}

func (r CommandWrapper) IntoCommand() contracts.Command {
	return contracts.Command{
		Kind:    uint8(r.Kind),
		Gas:     uint64(r.MaxDispatchGas),
		Payload: r.Params,
	}
}

func (m OutboundQueueMessage) IntoInboundMessage() contracts.InboundMessage {
	var commands []contracts.Command
	for _, command := range m.Commands {
		commands = append(commands, command.IntoCommand())
	}
	return contracts.InboundMessage{
		Origin:   m.Origin,
		Nonce:    uint64(m.Nonce),
		Topic:    m.Topic,
		Commands: commands,
	}
}

type MessageProof struct {
	Message OutboundQueueMessageWithFee
	Proof   MerkleProof
}
