package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/socket"
	convoyNet "github.com/frain-dev/convoy/net"
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

			if util.IsStringEmpty(forwardTo) {
				log.Fatal("flag forward-to cannot be empty")
			}

			listenRequest := socket.ListenRequest{
				HostName:   c.Host,
				DeviceID:   c.ActiveDeviceID,
				SourceID:   source,
				EventTypes: strings.Split(events, ","),
			}

			body, _ := json.Marshal(listenRequest)
			if err != nil {
				log.Fatal("Error marshalling json:", err)
			}

			hostInfo, err := url.Parse(c.Host)
			if err != nil {
				log.Fatal("Error parsing host URL: ", err)
			}

			url := url.URL{Scheme: "ws", Host: hostInfo.Host, Path: "/stream/listen"}
			conn, response, err := websocket.DefaultDialer.Dial(url.String(), http.Header{
				"Authorization": []string{"Bearer " + c.ActiveApiKey},
				"Body":          []string{string(body)},
			})

			if err != nil {
				buf, e := io.ReadAll(response.Body)
				if e != nil {
					log.Fatal("Error parsing request body", e)
				}
				defer response.Body.Close()

				log.Fatal("Error connecting to Websocket Server\n", err, "\nhttp: ", string(buf))
			}

			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

			defer conn.Close()
			go receiveHandler(conn, forwardTo)

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

					// stop the health checks
					ticker.Stop()

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
	cmd.Flags().StringVar(&events, "events", "*", "Events types")
	cmd.Flags().StringVar(&forwardTo, "forward-to", "", "Host to forward events to")

	return cmd
}

func receiveHandler(connection *websocket.Conn, url string) {
	defer close(done)
	for {
		_, msg, err := connection.ReadMessage()
		if err != nil {
			if !websocket.IsUnexpectedCloseError(err,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure) {
				return
			}

			log.Println("Error in receive:", err)
			return
		}

		var event socket.CLIEvent
		err = json.Unmarshal(msg, &event)
		if err != nil {
			log.Println("Error in reading json:", err)
			continue
		}

		ack := &socket.AckEventDelivery{UID: event.UID}
		j, err := json.Marshal(ack)
		if err != nil {
			log.Println("Error in marshalling json:", err)
			continue
		}

		// write an ack message back to the connection here
		err = connection.WriteMessage(websocket.TextMessage, j)
		if err != nil {
			log.Println("Error in writing to websocket connection")
		}

		// send request to the recepient
		d := convoyNet.NewDispatcher(time.Second * 10)
		res, err := d.ForwardCliEvent(url, convoy.HttpPost, event.Data, event.Headers)
		if err != nil {
			log.Println(err)
			continue
		}

		log.Println(string(res.Body))
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
		data, err := os.ReadFile(path)
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
