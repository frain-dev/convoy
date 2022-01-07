package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	convoyMemberlist "github.com/frain-dev/convoy/memberlist"
	convoyQueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker"
	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addNodeCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Start a server/worker node",
	}

	cmd.AddCommand(nodeServerCommand(a))
	cmd.AddCommand(nodeWorkerCommand(a))

	return cmd
}

func nodeServerCommand(a *app) *cobra.Command {
	var client *api.Client
	var sID string
	var doneCh chan struct{}
	var serviceKey string

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Create a server node",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}

			//Start Consul Session
			client, sID, doneCh, err = startConsulSession(cfg)
			if err != nil {
				log.Fatalf("Consul session failed: %v", err)
			}

			if util.IsStringEmpty(serviceKey) {
				serviceKey = convoy.ServiceKey
			}

			kv, _, err := client.KV().Get(serviceKey, nil)
			if err != nil {
				log.Fatalf("kv acquire err: %v", err)
			}

			if kv != nil && kv.Session != "" {
				// there is a server node already (in this cluster)
				log.Fatalf("There is a server node with a lock on servicekey %v", serviceKey)
			} else {
				hostName, err := os.Hostname()
				if err != nil {
					log.Fatalf("Hostname err: %v", err)
				}

				hostName = hostName + "-" + uuid.NewString()
				acquireKv := &api.KVPair{
					Session: sID,
					Key:     serviceKey,
					Value:   []byte(hostName),
				}

				acquired, _, err := client.KV().Acquire(acquireKv, nil)
				if err != nil {
					log.Fatalf("key-value acquire err: %v", err)
				}

				if acquired {
					log.Printf("Server node intitialized!\n")

					if err := convoyMemberlist.CreateMemberlist("", hostName); err != nil {
						log.Fatal("Error creating memberlist: %v", err)
					}
					err := StartConvoyServer(a, cfg, false)
					if err != nil {
						log.Printf("Error starting convoy server: %v", err)
					}
				}
			}
			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			destroyConsulSession(client, sID, doneCh)
			return nil
		},
	}
	cmd.Flags().StringVar(&serviceKey, "servicekey", "", "service key for leader election, if blank, default is used.")
	return cmd
}

func nodeWorkerCommand(a *app) *cobra.Command {
	var isLeader = false
	var isConsuming = false
	var client *api.Client
	var sID string
	var doneCh chan struct{}
	var clusterMembers string
	var serviceKey string

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Create a worker node",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}

			//Start Consul Session

			client, sID, doneCh, err = startConsulSession(cfg)
			if err != nil {
				log.Fatalf("Consul session failed: %v", err)
			}

			go func() {
				hostName, err := os.Hostname()
				if err != nil {
					log.Fatalf("Hostname err: %v", err)
				}
				hostName = hostName + "-" + uuid.NewString()
				if util.IsStringEmpty(serviceKey) {
					serviceKey = convoy.ServiceKey
				}
				acquireKv := &api.KVPair{
					Session: sID,
					Key:     serviceKey,
					Value:   []byte(hostName),
				}
				//Leader aquisition loop
				for {
					if !isLeader {
						acquired, _, err := client.KV().Acquire(acquireKv, nil)
						if err != nil {
							log.Fatalf("key-value acquire err: %v", err)
						}
						if !isConsuming && !acquired {
							log.Printf("Worker node intitialized!\n")
							if err := convoyMemberlist.CreateMemberlist(clusterMembers, hostName); err != nil {
								log.Fatal("Error creating memberlist: %v", err)
							}
							// register workers.
							if queue, ok := a.eventQueue.(*convoyQueue.RedisQueue); ok {
								worker.NewProducer(queue).Start()
							}

							if queue, ok := a.deadLetterQueue.(*convoyQueue.RedisQueue); ok {
								worker.NewCleaner(queue).Start()
							}
							isConsuming = true
						}

						if acquired {
							isLeader = true
							log.Printf("Leader aquisition successful!\n")
							err := StartConvoyServer(a, cfg, false)
							if err != nil {
								log.Printf("Error starting convoy server: %v", err)
							}

						}
					}

					time.Sleep(time.Duration(convoy.TTL/2) * time.Second)
				}
			}()

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			destroyConsulSession(client, sID, doneCh)
			return nil
		},
	}
	cmd.Flags().StringVar(&clusterMembers, "members", "", "comma seperated list of members")
	cmd.Flags().StringVar(&serviceKey, "servicekey", "", "service key for leader election, if blank, default is used.")
	return cmd
}

func destroyConsulSession(client *api.Client, sID string, doneCh chan struct{}) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	close(doneCh)
	log.Printf("Destroying consul session and leaving ...")
	_, err := client.Session().Destroy(sID, nil)
	if err != nil {
		log.Fatalf("session destroy err: %v", err)
	}
	os.Exit(0)
}

func startConsulSession(cfg config.Configuration) (*api.Client, string, chan struct{}, error) {
	// build consul client
	config := api.DefaultConfig()
	config.Address = cfg.Consul.DSN
	client, err := api.NewClient(config)
	if err != nil {
		log.Fatalf("Consul client err: %v", err)
	}

	log.Printf("Starting consul session!\n")
	// create session
	sEntry := &api.SessionEntry{
		Name:      convoy.ServiceName,
		TTL:       convoy.TTLS,
		LockDelay: 1 * time.Millisecond,
	}
	sID, _, err := client.Session().Create(sEntry, nil)
	if err != nil {
		log.Fatalf("Consul session create err: %v", err)
	}

	// auto renew session
	doneCh := make(chan struct{})
	go func() {
		err = client.Session().RenewPeriodic(convoy.TTLS, sID, nil, doneCh)
		if err != nil {
			log.Fatalf("Consul session renew err: %v", err)
		}
	}()
	return client, sID, doneCh, err
}
