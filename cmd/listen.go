package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var done chan interface{}
var interrupt chan os.Signal

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 10 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

func addListenCommand(a *app) *cobra.Command {
	var source string
	var events string
	var forwardTo string

	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Starts a websocket client that listens to events streamed by the server",
		Run: func(cmd *cobra.Command, args []string) {
			done = make(chan interface{})    // Channel to indicate that the receiverHandler is done
			interrupt = make(chan os.Signal) // Channel to listen for interrupt signal to terminate gracefully

			signal.Notify(interrupt, os.Interrupt) // Notify the interrupt channel for SIGINT

			c, err := loadConfig()
			if err != nil {
				log.Fatal("Error loading config file:", err)
			}

			eventTypes := strings.Split(events, ",")
			if util.IsStringEmpty(events) {
				eventTypes = []string{"*"}
			}

			listenRequest := services.ListenRequest{
				HostName:   c.Host,
				DeviceID:   c.ActiveDeviceID,
				SourceID:   source,
				EventTypes: eventTypes,
			}

			body, _ := json.Marshal(listenRequest)
			if err != nil {
				log.Fatal("Error marshalling json:", err)
			}

			url := "ws://localhost:5008/stream/listen"
			conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{
				"Authorization": []string{"Bearer " + c.ActiveApiKey},
				"Body":          []string{string(body)},
			})

			if err != nil {
				log.Fatal("Error connecting to Websocket Server:", err)
			}

			defer conn.Close()
			go receiveHandler(conn)

			ticker := time.NewTicker(pingPeriod)
			defer ticker.Stop()

			// Our main loop for the client
			// We send our relevant packets here
			for {
				select {
				case <-ticker.C:
					err := conn.SetWriteDeadline(time.Now().Add(writeWait))
					if err != nil {
						log.Println(err)
					}

					if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						return
					}

				case <-interrupt:
					// We received a SIGINT (Ctrl + C). Terminate gracefully...
					log.Println("Received SIGINT interrupt signal. Closing all pending connections")

					// Close our websocket connection
					err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
					if err != nil {
						log.Println("Error during closing websocket:", err)
						return
					}

					select {
					case <-done:
						log.Println("Receiver Channel Closed! Exiting....")
					case <-time.After(time.Duration(1) * time.Second):
						log.Println("Timeout in closing receiving channel. Exiting....")
					}
					return
				}
			}
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "Source ID")
	cmd.Flags().StringVar(&events, "events", "", "Events types")
	cmd.Flags().StringVar(&forwardTo, "forward-to", "", "Host to forward events to")

	return cmd
}

func receiveHandler(connection *websocket.Conn) {
	defer close(done)
	for {
		_, msg, err := connection.ReadMessage()
		if err != nil {
			log.Println("Error in receive:", err)
			return
		}
		// do some stuff here
		log.Printf("Received: %s\n", msg)
	}
}

func loadConfig() (*Config, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(homedir, defaultConfigDir)

	c := &Config{path: path}
	c.hasDefaultConfigFile = HasDefaultConfigFile(path)

	if !c.hasDefaultConfigFile {
		return nil, errors.New("config file not found")
	}

	if c.hasDefaultConfigFile {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}

		err = yaml.Unmarshal(data, &c)
		if err != nil {
			return nil, err
		}

		return c, nil
	}

	return nil, nil
}
