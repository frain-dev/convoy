package main

import (
	"context"
	"github.com/hookcamp/hookcamp/util"
	log "github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/config"
	"github.com/hookcamp/hookcamp/datastore"
	"github.com/hookcamp/hookcamp/queue"
	"github.com/hookcamp/hookcamp/queue/redis"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/mongo"
)

func main() {
	log.SetLevel(log.InfoLevel)

	log.SetFormatter(&prefixed.TextFormatter{
		DisableColors:   false,
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
		ForceFormatting: true,
	})
	log.SetReportCaller(true)

	err := os.Setenv("TZ", "") // Use UTC by default :)
	if err != nil {
		log.Fatal("failed to set env - ", err)
	}

	app := &app{}

	var db *mongo.Client

	cmd := &cobra.Command{
		Use:   "hookcamp",
		Short: "Opensource Webhooks as a service",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}

			err = config.LoadFromFile(cfgPath)
			if err != nil {
				return err
			}

			cfg, err := config.Get()
			if err != nil {
				return err
			}

			db, err = datastore.New(cfg)
			if err != nil {
				return err
			}

			var queuer queue.Queuer

			if cfg.Queue.Type == config.RedisQueueProvider {
				queuer, err = redis.New(cfg)
				if err != nil {
					return err
				}
			}

			if util.IsStringEmpty(string(cfg.Signature.Header)) {
				cfg.Signature.Header = config.DefaultSignatureHeader
				log.Warnf("signature header is blank. setting default %s", config.DefaultSignatureHeader)
			}

			conn := db.Database("hookcamp", nil)

			app.orgRepo = datastore.NewOrganisationRepo(conn)
			app.applicationRepo = datastore.NewApplicationRepo(conn)
			app.messageRepo = datastore.NewMessageRepository(conn)
			app.queue = queuer

			ensureMongoIndices(conn)

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			defer func() {
				err := app.queue.Close()
				if err != nil {
					log.Errorln("failed to close app queue - ", err)
				}
			}()
			err := db.Disconnect(context.Background())
			if err == nil {
				os.Exit(0)
			}
			return err
		},
	}

	var configFile string

	cmd.PersistentFlags().StringVar(&configFile, "config", "./hookcamp.json", "Configuration file for Hookcamp")

	cmd.AddCommand(addVersionCommand())
	cmd.AddCommand(addCreateCommand(app))
	cmd.AddCommand(addGetComamnd(app))
	cmd.AddCommand(addServerCommand(app))

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func ensureMongoIndices(conn *mongo.Database) {
	datastore.EnsureIndex(conn, datastore.OrgCollection, "uid", true)

	datastore.EnsureIndex(conn, datastore.AppCollections, "uid", true)

	datastore.EnsureIndex(conn, datastore.MsgCollection, "uid", true)
	datastore.EnsureIndex(conn, datastore.MsgCollection, "event_type", false)
}

type app struct {
	orgRepo         hookcamp.OrganisationRepository
	applicationRepo hookcamp.ApplicationRepository
	messageRepo     hookcamp.MessageRepository
	queue           queue.Queuer
}

func getCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Second*1)
}
