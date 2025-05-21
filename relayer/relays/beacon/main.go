package beacon

import (
	"context"
	"time"

	"github.com/snowfork/go-substrate-rpc-client/v4/signature"
	"github.com/snowfork/snowbridge/relayer/chain/solochain"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/config"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/header"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/header/syncer/api"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/protocol"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/store"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type Relay struct {
	config  *config.Config
	keypair *signature.KeyringPair
}

func NewRelay(
	config *config.Config,
	keypair *signature.KeyringPair,
) *Relay {
	return &Relay{
		config:  config,
		keypair: keypair,
	}
}

func (r *Relay) Start(ctx context.Context, eg *errgroup.Group) error {
	specSettings := r.config.Source.Beacon.Spec
	log.WithField("spec", specSettings).Info("spec settings")

	soloconn := solochain.NewConnection(r.config.Sink.Solochain.Endpoint, r.keypair)

	err := soloconn.ConnectWithHeartBeat(ctx, 30*time.Second)
	if err != nil {
		return err
	}

	writer := solochain.NewSolochainWriter(
		soloconn,
		r.config.Sink.Solochain.MaxWatchedExtrinsics,
	)

	p := protocol.New(specSettings, r.config.Sink.Solochain.HeaderRedundancy)

	err = writer.Start(ctx, eg)
	if err != nil {
		return err
	}

	s := store.New(r.config.Source.Beacon.DataStore.Location, r.config.Source.Beacon.DataStore.MaxEntries, *p)
	err = s.Connect()
	if err != nil {
		return err
	}

	beaconAPI := api.NewBeaconClient(r.config.Source.Beacon.Endpoint, r.config.Source.Beacon.StateEndpoint)
	headers := header.New(
		writer,
		beaconAPI,
		specSettings,
		&s,
		p,
		r.config.Sink.UpdateSlotInterval,
	)

	return headers.Sync(ctx, eg)
}
