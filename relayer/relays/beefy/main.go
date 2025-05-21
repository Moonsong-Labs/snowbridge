package beefy

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/snowfork/snowbridge/relayer/chain/ethereum"
	"github.com/snowfork/snowbridge/relayer/chain/solochain"
	"github.com/snowfork/snowbridge/relayer/crypto/secp256k1"

	log "github.com/sirupsen/logrus"
)

type Relay struct {
	config            *Config
	solochainConn     *solochain.Connection
	ethereumConn      *ethereum.Connection
	solochainListener *SolochainListener
	ethereumWriter    *EthereumWriter
}

func NewRelay(config *Config, ethereumKeypair *secp256k1.Keypair) (*Relay, error) {
	solochainConn := solochain.NewConnection(config.Source.Solochain.Endpoint, nil)
	ethereumConn := ethereum.NewConnection(&config.Sink.Ethereum, ethereumKeypair)

	solochainListener := NewSolochainListener(
		&config.Source,
		solochainConn,
	)

	ethereumWriter := NewEthereumWriter(&config.Sink, ethereumConn)

	log.Info("Beefy solo created")

	return &Relay{
		config:            config,
		solochainConn:     solochainConn,
		ethereumConn:      ethereumConn,
		solochainListener: solochainListener,
		ethereumWriter:    ethereumWriter,
	}, nil
}

func (relay *Relay) Start(ctx context.Context, eg *errgroup.Group) error {
	err := relay.solochainConn.ConnectWithHeartBeat(ctx, 30*time.Second)
	if err != nil {
		return fmt.Errorf("create solochain connection: %w", err)
	}

	err = relay.ethereumConn.Connect(ctx)
	if err != nil {
		return fmt.Errorf("create ethereum connection: %w", err)
	}
	err = relay.ethereumWriter.initialize(ctx)
	if err != nil {
		return fmt.Errorf("initialize ethereum writer: %w", err)
	}

	initialState, err := relay.ethereumWriter.queryBeefyClientState(ctx)
	if err != nil {
		return fmt.Errorf("fetch BeefyClient current state: %w", err)
	}
	log.WithFields(log.Fields{
		"beefyBlock":     initialState.LatestBeefyBlock,
		"validatorSetID": initialState.CurrentValidatorSetID,
	}).Info("Retrieved current BeefyClient state")

	requests, err := relay.solochainListener.Start(ctx, eg, initialState.LatestBeefyBlock)
	if err != nil {
		return fmt.Errorf("initialize solochain listener: %w", err)
	}

	err = relay.ethereumWriter.Start(ctx, eg, requests)
	if err != nil {
		return fmt.Errorf("start ethereum writer: %w", err)
	}

	return nil
}
