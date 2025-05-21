package cmd

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/snowfork/snowbridge/relayer/chain/solochain"
	"github.com/snowfork/snowbridge/relayer/crypto/sr25519"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/cache"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/config"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/header/syncer"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/header/syncer/api"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/protocol"
	"github.com/snowfork/snowbridge/relayer/relays/beacon/store"

	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"
)

func importExecutionHeaderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import-execution-header",
		Short: "Import the provided execution header.",
		Args:  cobra.ExactArgs(0),
		RunE:  importExecutionHeaderFn,
	}

	cmd.Flags().String("beacon-header", "", "Beacon header hash whose execution header will be imported")
	err := cmd.MarkFlagRequired("beacon-header")
	if err != nil {
		return nil
	}

	cmd.Flags().String("finalized-header", "", "Finalized header to prove execution header against")
	err = cmd.MarkFlagRequired("finalized-header")
	if err != nil {
		return nil
	}

	cmd.Flags().String("solochain-endpoint", "", "Solochain API URL")
	err = cmd.MarkFlagRequired("solochain-endpoint")
	if err != nil {
		return nil
	}

	cmd.Flags().String("lodestar-endpoint", "", "Lodestar API URL")
	err = cmd.MarkFlagRequired("lodestar-endpoint")
	if err != nil {
		return nil
	}

	cmd.Flags().String("private-key-file", "", "File containing the private key for the relayer")
	err = cmd.MarkFlagRequired("private-key-file")
	if err != nil {
		return nil
	}

	cmd.Flags().String("network", "", "Network name: valid values are mainnet, goerli, local")
	err = cmd.MarkFlagRequired("network")
	if err != nil {
		return nil
	}

	return cmd
}

func importExecutionHeaderFn(cmd *cobra.Command, _ []string) error {
	err := func() error {
		ctx := cmd.Context()

		eg, ctx := errgroup.WithContext(ctx)

		solochainEndpoint, _ := cmd.Flags().GetString("solochain-endpoint")
		privateKeyFile, _ := cmd.Flags().GetString("private-key-file")
		lodestarEndpoint, _ := cmd.Flags().GetString("lodestar-endpoint")
		beaconHeader, _ := cmd.Flags().GetString("beacon-header")
		finalizedHeader, _ := cmd.Flags().GetString("finalized-header")

		viper.SetConfigFile("web/packages/test/config/beacon-relay.json")
		if err := viper.ReadInConfig(); err != nil {
			return err
		}
		var conf config.Config
		err := viper.Unmarshal(&conf)
		if err != nil {
			return err
		}

		keypair, err := getKeyPair(privateKeyFile)
		if err != nil {
			return fmt.Errorf("get keypair from file: %w", err)
		}

		soloconn := solochain.NewConnection(solochainEndpoint, keypair.AsKeyringPair())
		err = soloconn.Connect(ctx)
		if err != nil {
			return fmt.Errorf("connect to solochain: %w", err)
		}

		writer := solochain.NewSolochainWriter(soloconn, 8)
		err = writer.Start(ctx, eg)
		if err != nil {
			return fmt.Errorf("start solochain conn: %w", err)
		}

		log.WithField("hash", beaconHeader).Info("will be syncing execution header for beacon hash")

		p := protocol.New(conf.Source.Beacon.Spec, conf.Sink.Solochain.HeaderRedundancy)
		store := store.New(conf.Source.Beacon.DataStore.Location, conf.Source.Beacon.DataStore.MaxEntries, *p)
		store.Connect()
		defer store.Close()

		client := api.NewBeaconClient(lodestarEndpoint, lodestarEndpoint)
		syncer := syncer.New(client, &store, p)

		beaconHeaderHash := common.HexToHash(finalizedHeader)

		finalizedUpdate, err := syncer.GetFinalizedUpdate()

		err = writer.WriteToSolochainAndWatch(ctx, "EthereumBeaconClient.import_finalized_header", finalizedUpdate.Payload)
		if err != nil {
			return fmt.Errorf("write to solochain: %w", err)
		}
		log.Info("imported finalized header")

		checkpoint := cache.Proof{
			FinalizedBlockRoot: finalizedUpdate.FinalizedHeaderBlockRoot,
			BlockRootsTree:     finalizedUpdate.BlockRootsTree,
			Slot:               uint64(finalizedUpdate.Payload.FinalizedHeader.Slot),
		}

		update, err := syncer.GetHeaderUpdate(beaconHeaderHash, &checkpoint)
		if err != nil {
			return fmt.Errorf("get header update: %w", err)
		}
		log.WithField("slot", update.Header.Slot).Info("found block at slot")

		err = writer.WriteToSolochainAndWatch(ctx, "EthereumBeaconClient.import_execution_header", update)
		if err != nil {
			return fmt.Errorf("write to solochain: %w", err)
		}
		log.Info("imported execution header")

		return nil
	}()
	if err != nil {
		log.WithError(err).Error("error importing execution header")
	}

	return nil
}

func getKeyPair(privateKeyFile string) (*sr25519.Keypair, error) {
	var cleanedKeyURI string
	content, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("cannot read key file: %w", err)
	}
	cleanedKeyURI = strings.TrimSpace(string(content))
	keypair, err := sr25519.NewKeypairFromSeed(cleanedKeyURI, 42)
	if err != nil {
		return nil, fmt.Errorf("parse private key URI: %w", err)
	}

	return keypair, nil
}
