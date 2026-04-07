package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/dataplane"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/util"
)

func AddAgentCommand(a *cli.App) *cobra.Command {
	var agentPort uint32
	var consumerPoolSize int
	var interval int

	var smtpSSL bool
	var smtpUsername string
	var smtpPassword string
	var smtpReplyTo string
	var smtpFrom string
	var smtpProvider string
	var executionMode string
	var smtpUrl string
	var smtpPort uint32

	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Start agent instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			quit := make(chan os.Signal, 1)
			signal.Notify(quit, os.Interrupt)

			defer func() {
				signal.Stop(quit)
				cancel()
			}()

			// override config with cli flags
			cliConfig, err := buildAgentCliConfiguration(cmd)
			if err != nil {
				return err
			}

			if err = config.Override(cliConfig); err != nil {
				return err
			}

			cfg, err := config.Get()
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			runtime, err := dataplane.New(ctx, buildRuntimeOpts(a), cfg, interval)
			if err != nil {
				return err
			}

			runtimeErr := make(chan error, 1)
			go func() {
				runtimeErr <- runtime.Run(ctx)
			}()

			select {
			case <-quit:
				cancel()
				return nil
			case eRrr := <-runtimeErr:
				if eRrr != nil && !errors.Is(eRrr, context.Canceled) {
					return eRrr
				}
				return nil
			case <-ctx.Done():
			}

			return ctx.Err()
		},
	}

	cmd.Flags().BoolVar(&smtpSSL, "smtp-ssl", false, "Enable SMTP SSL")
	cmd.Flags().StringVar(&smtpUsername, "smtp-username", "", "SMTP authentication username")
	cmd.Flags().StringVar(&smtpPassword, "smtp-password", "", "SMTP authentication password")
	cmd.Flags().StringVar(&smtpFrom, "smtp-from", "", "Sender email address")
	cmd.Flags().StringVar(&smtpReplyTo, "smtp-reply-to", "", "Email address to reply to")
	cmd.Flags().StringVar(&smtpProvider, "smtp-provider", "", "SMTP provider")
	cmd.Flags().StringVar(&smtpUrl, "smtp-url", "", "SMTP provider URL")
	cmd.Flags().Uint32Var(&smtpPort, "smtp-port", 0, "SMTP Port")

	cmd.Flags().Uint32Var(&agentPort, "port", 0, "Agent port")

	cmd.Flags().IntVar(&consumerPoolSize, "consumers", -1, "Size of the consumers pool.")
	cmd.Flags().IntVar(&interval, "interval", 10, "the time interval, measured in seconds to update the in-memory store from the database")
	cmd.Flags().StringVar(&executionMode, "mode", "", "Execution Mode (one of events, retry and default)")

	return cmd
}

func buildAgentCliConfiguration(cmd *cobra.Command) (*config.Configuration, error) {
	c := &config.Configuration{}

	// PORT
	port, err := cmd.Flags().GetUint32("port")
	if err != nil {
		return nil, err
	}

	if port != 0 {
		c.Server.HTTP.AgentPort = port
	}

	// CONVOY_WORKER_POOL_SIZE
	consumerPoolSize, err := cmd.Flags().GetInt("consumers")
	if err != nil {
		return nil, err
	}

	if consumerPoolSize >= 0 {
		c.ConsumerPoolSize = consumerPoolSize
	}

	// CONVOY_SMTP_PROVIDER
	smtpProvider, err := cmd.Flags().GetString("smtp-provider")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpProvider) {
		c.SMTP.Provider = smtpProvider
	}

	// CONVOY_SMTP_URL
	smtpUrl, err := cmd.Flags().GetString("smtp-url")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpUrl) {
		c.SMTP.URL = smtpUrl
	}

	// CONVOY_SMTP_USERNAME
	smtpUsername, err := cmd.Flags().GetString("smtp-username")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpUsername) {
		c.SMTP.Username = smtpUsername
	}

	// CONVOY_SMTP_PASSWORD
	smtpPassword, err := cmd.Flags().GetString("smtp-password")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpPassword) {
		c.SMTP.Password = smtpPassword
	}

	// CONVOY_SMTP_FROM
	smtpFrom, err := cmd.Flags().GetString("smtp-from")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpFrom) {
		c.SMTP.From = smtpFrom
	}

	// CONVOY_SMTP_REPLY_TO
	smtpReplyTo, err := cmd.Flags().GetString("smtp-reply-to")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(smtpReplyTo) {
		c.SMTP.ReplyTo = smtpReplyTo
	}

	// CONVOY_SMTP_PORT
	smtpPort, err := cmd.Flags().GetUint32("smtp-port")
	if err != nil {
		return nil, err
	}

	if smtpPort != 0 {
		c.SMTP.Port = smtpPort
	}

	// CONVOY_WORKER_EXECUTION_MODE
	executionMode, err := cmd.Flags().GetString("mode")
	if err != nil {
		return nil, err
	}

	if !util.IsStringEmpty(executionMode) {
		c.WorkerExecutionMode = config.ExecutionMode(executionMode)
	}

	return c, nil
}

func buildRuntimeOpts(a *cli.App) dataplane.RuntimeOpts {
	return dataplane.RuntimeOpts{
		DB:            a.DB,
		Redis:         a.Redis,
		Queue:         a.Queue,
		Logger:        a.Logger,
		Cache:         a.Cache,
		Rate:          a.Rate,
		Licenser:      a.Licenser,
		TracerBackend: a.TracerBackend,
	}
}
