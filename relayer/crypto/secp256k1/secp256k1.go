// Copyright 2020 ChainSafe Systems
// SPDX-License-Identifier: LGPL-3.0-only

package secp256k1

import (
	"crypto/ecdsa"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/snowfork/go-substrate-rpc-client/v4/signature"
	"github.com/snowfork/snowbridge/relayer/crypto"

	secp256k1 "github.com/ethereum/go-ethereum/crypto"
)

var _ crypto.Keypair = &Keypair{}

const PrivateKeyLength = 32

type Keypair struct {
	keyringPair *signature.KeyringPair
}

func NewKeypairFromPrivateKey(priv []byte) (*Keypair, error) {
	return NewKeypairFromString(string(priv))
}

// NewKeypairFromString parses a string for a hex private key. Must be at least
// PrivateKeyLength long.
func NewKeypairFromString(priv string) (*Keypair, error) {
	kp, err := signature.NewEcdsaKeyringPair(priv)
	if err != nil {
		return nil, err
	}

	return &Keypair{
		keyringPair: &kp,
	}, nil
}

func NewKeypair(pk ecdsa.PrivateKey) (*Keypair, error) {
	privateKeyBytes := secp256k1.FromECDSA(&pk)
	privateKeyHex := hexutil.Encode(privateKeyBytes)
	return NewKeypairFromString(privateKeyHex)
}

func GenerateKeypair() (*Keypair, error) {
	priv, err := secp256k1.GenerateKey()
	if err != nil {
		return nil, err
	}

	return NewKeypair(*priv)
}

// Encode dumps the private key as bytes
func (kp *Keypair) Encode() []byte {
	privateKeyBytes, _ := hex.DecodeString(kp.keyringPair.URI)
	return privateKeyBytes
}

// Decode initializes the keypair using the input
func (kp *Keypair) Decode(in []byte) error {
	kp, err := NewKeypairFromPrivateKey(in)
	if err != nil {
		return err
	}

	return nil
}

// Address returns the Ethereum address format
func (kp *Keypair) Address() string {
	return kp.keyringPair.Address
}

// CommonAddress returns the Ethereum address in the common.Address Format
func (kp *Keypair) CommonAddress() common.Address {
	return common.HexToAddress(kp.Address())
}

// PublicKey returns the public key hex encoded
func (kp *Keypair) PublicKey() string {
	return hexutil.Encode(kp.keyringPair.PublicKey)
}

// PrivateKey returns the keypair's private key
func (kp *Keypair) PrivateKey() *ecdsa.PrivateKey {
	pk, _ := secp256k1.ToECDSA(kp.Encode())
	return pk
}

func (kp *Keypair) AsKeyringPair() *signature.KeyringPair {
	return kp.keyringPair
}
