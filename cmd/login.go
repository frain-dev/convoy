package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/net"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addLoginCommand(a *app) *cobra.Command {

	var apiKey string
	var host string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Starts events search indexer",
		RunE: func(cmd *cobra.Command, args []string) error {
			buff := bytes.NewBuffer([]byte{})
			encoder := json.NewEncoder(buff)
			encoder.SetEscapeHTML(false)

			if err := encoder.Encode("{\"key\":\"value\"}"); err != nil {
				log.WithError(err).Error("Failed to encode data")
			}
			body := strings.TrimSuffix(buff.String(), "\n")

			dispatch := net.NewDispatcher(time.Second * 10)

			resp, err := dispatch.SendCliRequest(fmt.Sprintf("%s/cli/login", host), convoy.HttpPost, apiKey, []byte(body))
			if err != nil {
				return err
			}

			fmt.Printf("%+v\n", string(resp.Body))

			return nil
		},
	}

	cmd.Flags().StringVar(&apiKey, "api-key", "", "API Key")
	cmd.Flags().StringVar(&host, "host", "", "Host")

	return cmd
}
