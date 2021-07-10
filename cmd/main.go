package main

import (
	"context"
	"log"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/config"
	"github.com/hookcamp/hookcamp/datastore"
	"github.com/hookcamp/hookcamp/queue"
	"github.com/hookcamp/hookcamp/queue/redis"
	awssqs "github.com/hookcamp/hookcamp/queue/sqs"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/mongo"
)

func main() {
	os.Setenv("TZ", "") // Use UTC by default :)

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

			switch cfg.Queue.Type {
			case config.RedisQueueProvider:
				queuer, err = redis.New(cfg)
				if err != nil {
					return err
				}
			case config.SqsQueueProvider:
				queuer, err = awssqs.New(cfg)
				if err != nil {
					return err
				}
			}

			conn := db.Database("hookcamp", nil)

			app.orgRepo = datastore.NewOrganisationRepo(conn)
			app.applicationRepo = datastore.NewApplicationRepo(conn)
			// app.endpointRepo = datastore.NewEndpointRepository(db)
			// app.messageRepo = datastore.NewMessageRepository(db)
			app.queue = queuer

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return db.Disconnect(context.Background())
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

type app struct {
	orgRepo         hookcamp.OrganisationRepository
	applicationRepo hookcamp.ApplicationRepository
	messageRepo     hookcamp.MessageRepository
	queue           queue.Queuer
}

func getCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Second*1)
}
