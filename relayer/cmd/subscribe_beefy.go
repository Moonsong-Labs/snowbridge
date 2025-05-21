package cmd

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/snowfork/go-substrate-rpc-client/v4/types"
	"github.com/snowfork/snowbridge/relayer/chain/solochain"
	"github.com/snowfork/snowbridge/relayer/crypto/keccak"
	"github.com/snowfork/snowbridge/relayer/crypto/merkle"
	"github.com/spf13/cobra"
)

func subBeefyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sub-beefy",
		Short: "Subscribe beefy messages",
		Args:  cobra.ExactArgs(0),
		RunE:  SubBeefyFn,
	}

	cmd.Flags().StringP("url", "u", "ws://127.0.0.1:9944", "Solochain URL")

	return cmd
}

func SubBeefyFn(cmd *cobra.Command, _ []string) error {
	subBeefyJustifications(cmd.Context(), cmd)
	return nil
}

func subBeefyJustifications(ctx context.Context, cmd *cobra.Command) error {
	url, _ := cmd.Flags().GetString("url")

	conn := solochain.NewConnection(url, nil)
	err := conn.Connect(ctx)
	if err != nil {
		log.Error(err)
		return err
	}

	sub, err := conn.API().RPC.Beefy.SubscribeJustifications()
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	for {
		select {
		case commitment, ok := <-sub.Chan():
			if !ok {
				return nil
			}

			blockNumber := commitment.Commitment.BlockNumber

			if len(commitment.Signatures) == 0 {
				log.Info("BEEFY commitment has no signatures, skipping...")
				continue
			}

			blockHash, err := conn.API().RPC.Chain.GetBlockHash(uint64(blockNumber))
			if err != nil {
				return err
			}

			proof, err := conn.API().RPC.MMR.GenerateProof(blockNumber, blockHash)
			if err != nil {
				return err
			}

			simpleProof, err := merkle.ConvertToSimplifiedMMRProof(
				proof.BlockHash,
				uint64(proof.Proof.LeafIndex),
				proof.Leaf,
				uint64(proof.Proof.LeafCount),
				proof.Proof.Items,
			)
			if err != nil {
				return err
			}

			leafEncoded, err := types.EncodeToBytes(simpleProof.Leaf)
			if err != nil {
				return err
			}
			leafHashBytes := (&keccak.Keccak256{}).Hash(leafEncoded)

			var leafHash types.H256
			copy(leafHash[:], leafHashBytes[0:32])

			root := merkle.CalculateMerkleRoot(&simpleProof, leafHash)
			if err != nil {
				return err
			}

			var actualMmrRoot types.H256

			mmrRootKey, err := types.CreateStorageKey(conn.Metadata(), "Mmr", "RootHash", nil, nil)
			if err != nil {
				return err
			}

			_, err = conn.API().RPC.State.GetStorage(mmrRootKey, &actualMmrRoot, blockHash)
			if err != nil {
				return err
			}

			actualParentHash, err := conn.API().RPC.Chain.GetBlockHash(uint64(proof.Leaf.ParentNumberAndHash.ParentNumber))
			if err != nil {
				return err
			}

			fmt.Printf("Commitment { BlockNumber: %v, ValidatorSetID: %v}\n",
				blockNumber,
				commitment.Commitment.ValidatorSetID,
			)

			fmt.Printf("Leaf { ParentNumber: %v, ParentHash: %v, NextValidatorSetID: %v}\n",
				proof.Leaf.ParentNumberAndHash.ParentNumber,
				proof.Leaf.ParentNumberAndHash.Hash.Hex(),
				proof.Leaf.BeefyNextAuthoritySet.ID,
			)

			fmt.Printf("Actual ParentHash: %v %v\n", actualParentHash.Hex(), actualParentHash == proof.Leaf.ParentNumberAndHash.Hash)

			fmt.Printf("MMR Root: computed=%v actual=%v %v\n",
				root.Hex(), actualMmrRoot.Hex(), root == actualMmrRoot,
			)

			fmt.Printf("\n")

			if root != actualMmrRoot {
				return nil
			}

		}
	}
}

func printCommitment(commitment *types.SignedCommitment, conn *solochain.Connection) error {
	blockNumber := commitment.Commitment.BlockNumber

	blockHash, err := conn.API().RPC.Chain.GetBlockHash(uint64(blockNumber))
	if err != nil {
		return err
	}

	proof, err := conn.API().RPC.MMR.GenerateProof(blockNumber, blockHash)
	if err != nil {
		return err
	}

	fmt.Printf("Commitment { BlockNumber: %v, ValidatorSetID: %v}; Leaf { ParentNumber: %v, NextValidatorSetID: %v }\n",
		blockNumber, commitment.Commitment.ValidatorSetID, proof.Leaf.ParentNumberAndHash.ParentNumber, proof.Leaf.BeefyNextAuthoritySet.ID,
	)

	return nil
}
