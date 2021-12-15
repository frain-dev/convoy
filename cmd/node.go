package main

import (
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	convoyQueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/server"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addNodeCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Start a master/worker node",
	}

	cmd.AddCommand(nodeMasterCommand(a))
	cmd.AddCommand(nodeWorkerCommand(a))

	return cmd
}

func nodeMasterCommand(a *app) *cobra.Command {
	var client *api.Client
	var sID string
	var doneCh chan struct{}
	cmd := &cobra.Command{
		Use:   "master",
		Short: "Create a master node",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}

			//Start Consul Session

			client, sID, doneCh = startConsulSession(cfg)

			kv, _, err := client.KV().Get(convoy.ServiceKey, nil)
			if err != nil {
				log.Fatalf("kv acquire err: %v", err)
			}

			if kv != nil && kv.Session != "" {
				// there is a master already
				log.Fatalf("There is a master node already.")
			} else {
				hostName, err := os.Hostname()
				if err != nil {
					log.Fatalf("hostname err: %v", err)
				}

				acquireKv := &api.KVPair{
					Session: sID,
					Key:     convoy.ServiceKey,
					Value:   []byte(hostName),
				}
				acquired, _, err := client.KV().Acquire(acquireKv, nil)
				if err != nil {
					log.Fatalf("kv acquire err: %v", err)
				}

				if acquired {
					log.Printf("Master node intitialized!\n")
					startConvoyServer(a, cfg)
				}
			}
			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			// wait for SIGINT or SIGTERM, clean up and exit
			sigCh := make(chan os.Signal)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			<-sigCh
			close(doneCh)
			log.Printf("Destroying consul session and leaving ...")
			_, err := client.Session().Destroy(sID, nil)
			if err != nil {
				log.Fatalf("session destroy err: %v", err)
			}
			os.Exit(0)
			return nil
		},
	}

	return cmd
}

func nodeWorkerCommand(a *app) *cobra.Command {
	var isLeader = false
	var isConsuming = false
	var client *api.Client
	var sID string
	var doneCh chan struct{}

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Create a worker node",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}

			//Start Consul Session

			client, sID, doneCh = startConsulSession(cfg)

			go func() {
				hostName, err := os.Hostname()
				if err != nil {
					log.Fatalf("hostname err: %v", err)
				}

				acquireKv := &api.KVPair{
					Session: sID,
					Key:     convoy.ServiceKey,
					Value:   []byte(hostName),
				}
				//Leader aquisition loop
				for {
					if !isLeader {
						acquired, _, err := client.KV().Acquire(acquireKv, nil)
						if err != nil {
							log.Fatalf("kv acquire err: %v", err)
						}
						if !isConsuming && !acquired {
							log.Printf("Worker node intitialized!\n")
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
							log.Printf("I'm the master now!\n")
							startConvoyServer(a, cfg)
						}
					}

					time.Sleep(time.Duration(convoy.TTL/2) * time.Second)
				}
			}()

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			// wait for SIGINT or SIGTERM, clean up and exit
			sigCh := make(chan os.Signal)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			<-sigCh
			close(doneCh)
			log.Printf("Destroying consul session and leaving ...")
			_, err := client.Session().Destroy(sID, nil)
			if err != nil {
				log.Fatalf("session destroy err: %v", err)
			}
			os.Exit(0)
			return nil
		},
	}
	return cmd
}

func startConsulSession(cfg config.Configuration) (*api.Client, string, chan struct{}) {
	// build consul client
	config := api.DefaultConfig()
	config.Address = cfg.Consul.DSN
	client, err := api.NewClient(config)
	if err != nil {
		log.Fatalf("Consul client err: %v", err)
	}

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

	log.Printf("Starting consul session!\n")

	return client, sID, doneCh
}

func startConvoyServer(a *app, cfg config.Configuration) error {
	start := time.Now()
	log.Info("Starting Convoy server...")
	if util.IsStringEmpty(string(cfg.GroupConfig.Signature.Header)) {
		cfg.GroupConfig.Signature.Header = config.DefaultSignatureHeader
		log.Warnf("signature header is blank. setting default %s", config.DefaultSignatureHeader)
	}

	err := realm_chain.Init(&cfg.Auth)
	if err != nil {
		log.WithError(err).Fatal("failed to initialize realm chain")
	}

	if cfg.Server.HTTP.Port <= 0 {
		return errors.New("please provide the HTTP port in the convoy.json file")
	}
	// register tasks.
	handler := task.ProcessEventDelivery(a.applicationRepo, a.eventDeliveryRepo, a.groupRepo)
	if err := task.CreateTasks(a.groupRepo, handler); err != nil {
		log.WithError(err).Error("failed to register tasks")
		return err
	}
	srv := server.New(cfg, a.eventRepo, a.eventDeliveryRepo, a.applicationRepo, a.groupRepo, a.eventQueue)

	log.Infof("Started convoy server in %s", time.Since(start))

	httpConfig := cfg.Server.HTTP
	if httpConfig.SSL {
		log.Infof("Started server with SSL: cert_file: %s, key_file: %s", httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
		return srv.ListenAndServeTLS(httpConfig.SSLCertFile, httpConfig.SSLKeyFile)
	}
	return srv.ListenAndServe()
}
