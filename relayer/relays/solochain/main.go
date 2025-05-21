package solochain

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/snowfork/go-substrate-rpc-client/v4/signature"
	"github.com/snowfork/snowbridge/relayer/chain/ethereum"
	"github.com/snowfork/snowbridge/relayer/chain/solochain"
	"github.com/snowfork/snowbridge/relayer/crypto/secp256k1"

	"github.com/snowfork/snowbridge/relayer/ofac"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/header"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/header/syncer/api"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/protocol"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/store"

	log "github.com/sirupsen/logrus"
)

type Relay struct {
	config                *Config
	solochainConn         *solochain.Connection
	ethereumConnWriter    *ethereum.Connection
	ethereumConnBeefy     *ethereum.Connection
	ethereumChannelWriter *EthereumWriter
	beefyListener         *BeefyListener
	solochainWriter       *solochain.SolochainWriter
	beaconHeader          *header.Header
	headerCache           *ethereum.HeaderCache
}

func NewRelay(config *Config, ethKeypair *secp256k1.Keypair, substrateKeypair *signature.KeyringPair) (*Relay, error) {
	log.Info("Creating worker")

	solochainConn := solochain.NewConnection(config.Source.Solochain.Endpoint, nil)

	ethereumConnWriter := ethereum.NewConnection(&config.Sink.Ethereum, ethKeypair)
	ethereumConnBeefy := ethereum.NewConnection(&config.Source.Ethereum, ethKeypair)

	ofacClient := ofac.New(config.OFAC.Enabled, config.OFAC.ApiKey)

	// channel for messages from beefy listener to ethereum writer
	var tasks = make(chan *Task, 1)

	ethereumChannelWriter, err := NewEthereumWriter(
		&config.Sink,
		ethereumConnWriter,
		tasks,
		config,
	)
	if err != nil {
		return nil, err
	}

	beefyListener := NewBeefyListener(
		&config.Source,
		&config.Schedule,
		ethereumConnBeefy,
		solochainConn,
		ofacClient,
		tasks,
	)

	solochainWriterConn := solochain.NewConnection(config.Source.Solochain.Endpoint, substrateKeypair)

	solochainWriter := solochain.NewSolochainWriter(
		solochainWriterConn,
		8,
	)
	headerCache, err := ethereum.NewHeaderBlockCache(
		&ethereum.DefaultBlockLoader{Conn: ethereumConnWriter},
	)
	if err != nil {
		return nil, err
	}
	p := protocol.New(config.Source.Beacon.Spec, 20)
	store := store.New(config.Source.Beacon.DataStore.Location, config.Source.Beacon.DataStore.MaxEntries, *p)
	store.Connect()
	beaconAPI := api.NewBeaconClient(config.Source.Beacon.Endpoint, config.Source.Beacon.StateEndpoint)
	beaconHeader := header.New(
		solochainWriter,
		beaconAPI,
		config.Source.Beacon.Spec,
		&store,
		p,
		0, // setting is not used in the execution relay
	)
	return &Relay{
		config:                config,
		solochainConn:         solochainConn,
		ethereumConnWriter:    ethereumConnWriter,
		ethereumConnBeefy:     ethereumConnBeefy,
		ethereumChannelWriter: ethereumChannelWriter,
		beefyListener:         beefyListener,
		solochainWriter:       solochainWriter,
		beaconHeader:          &beaconHeader,
		headerCache:           headerCache,
	}, nil
}

func (relay *Relay) Start(ctx context.Context, eg *errgroup.Group) error {
	err := relay.solochainConn.ConnectWithHeartBeat(ctx, 30*time.Second)
	if err != nil {
		return err
	}

	err = relay.ethereumConnWriter.Connect(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to ethereum: writer: %w", err)
	}

	err = relay.ethereumConnBeefy.Connect(ctx)
	if err != nil {
		return fmt.Errorf("unable to connect to ethereum: beefy: %w", err)
	}

	log.Info("Starting beefy listener")
	err = relay.beefyListener.Start(ctx, eg)
	if err != nil {
		return err
	}

	log.Info("Starting ethereum writer")
	err = relay.ethereumChannelWriter.Start(ctx, eg)
	if err != nil {
		return err
	}

	err = relay.solochainWriter.Start(ctx, eg)
	if err != nil {
		return err
	}

	err = relay.startDeliverProof(ctx, eg)
	if err != nil {
		return err
	}

	log.Info("Current relay's ID:", relay.config.Schedule.ID)

	return nil
}
