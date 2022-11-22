package main

import (
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/gorilla/websocket"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	done      chan interface{}
	interrupt chan os.Signal
)

const (
	// Time allowed to write a message to the server.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the server.
	pongWait = 10 * time.Second

	// Send pings to server with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

func addListenCommand(a *app) *cobra.Command {
	var since string
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

			body, err := json.Marshal(listenRequest)
			if err != nil {
				log.Fatal("Error marshalling json:", err)
			}

			hostInfo, err := url.Parse(c.Host)
			if err != nil {
				log.Fatal("Error parsing host URL: ", err)
			}

			if !util.IsStringEmpty(since) {
				var sinceTime time.Time
				sinceTime, err = time.Parse(time.RFC3339, since)
				if err != nil {
					log.WithError(err).Error("since is not a valid timestamp, will try time duration")

					dur, err := time.ParseDuration(since)
					if err != nil {
						log.WithError(err).Fatal("since is neither a valid time duration or timestamp, see the listen command help menu for a valid since value")
					} else {
						since = fmt.Sprintf("since|duration|%v", since)
						sinceTime = time.Now().Add(-dur)
					}
				} else {
					since = fmt.Sprintf("since|timestamp|%v", since)
				}

				log.Printf("will resend all discarded events after: %v", sinceTime)
			}

			url := url.URL{Scheme: "ws", Host: hostInfo.Host, Path: "/stream/listen"}
			conn, response, err := websocket.DefaultDialer.Dial(url.String(), http.Header{
				"Authorization": []string{"Bearer " + c.ActiveApiKey},
				"Body":          []string{string(body)},
			})
			if err != nil {
				if response != nil {
					buf, e := io.ReadAll(response.Body)
					if e != nil {
						log.Fatal("Error parsing request body", e)
					}
					defer response.Body.Close()
					log.Fatal("\nhttp: ", string(buf))
				}

				log.Fatal(err)
			}

			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

			defer conn.Close()
			go receiveHandler(conn, forwardTo)

			ticker := time.NewTicker(pingPeriod)
			defer ticker.Stop()

			if !util.IsStringEmpty(since) {
				// Send a message to the server to resend unsuccessful events to the device
				err := conn.WriteMessage(websocket.TextMessage, []byte(since))
				if err != nil {
					log.Println("an error occured sending 'since' message", err)
				}
			}

			// Our main loop for the client
			// We send our relevant packets here
			for {
				select {
				case <-ticker.C:
					err := conn.SetWriteDeadline(time.Now().Add(writeWait))
					if err != nil {
						log.WithError(err).Errorln("failed to set write deadline")
					}

					if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						return
					}

				case <-interrupt:
					// We received a SIGINT (Ctrl + C). Terminate gracefully...
					log.Println("Received SIGINT interrupt signal. Closing all pending connections")

					// stop the health checks
					ticker.Stop()

					// Send a message to set the device to offline
					err := conn.WriteMessage(websocket.TextMessage, []byte("disconnect"))
					if err != nil {
						log.Println("Error during closing websocket:", err)
						return
					}

					// Close our websocket connection
					err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
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

	cmd.Flags().StringVar(&source, "source", "", "The source id of the source you want to receive events from (only applies to incoming projects)")
	cmd.Flags().StringVar(&since, "since", "", "Send discarded events since a timestamp (e.g. 2013-01-02T13:23:37Z) or relative time (e.g. 42m for 42 minutes)")
	cmd.Flags().StringVar(&events, "events", "*", "Events types")
	cmd.Flags().StringVar(&forwardTo, "forward-to", "", "The host/web server you want to forward events to")

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

			log.Error("an error occured in the receive handler:", err)
			return
		}

		var event socket.CLIEvent
		err = json.Unmarshal(msg, &event)
		if err != nil {
			log.Error("an error occured in unmarshaling json:", err)
			continue
		}

		// send request to the recepient
		d, err := convoyNet.NewDispatcher(time.Second*10, "")
		if err != nil {
			log.Error("an error occured while forwading the event", err)
			continue
		}

		res, err := d.ForwardCliEvent(url, convoy.HttpPost, event.Data, event.Headers)
		if err != nil {
			log.Error("an error occured while forwading the event", err)
			continue
		}

		// set the event delivery status to Success when we sucessfully forward the event
		ack := &socket.AckEventDelivery{UID: event.UID}
		mb, err := json.Marshal(ack)
		if err != nil {
			log.Error("an error occured in marshalling json:", err)
			continue
		}

		// write an ack message back to the connection here
		err = connection.WriteMessage(websocket.TextMessage, mb)
		if err != nil {
			log.Error("an error occured while acknowledging the event", err)
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
