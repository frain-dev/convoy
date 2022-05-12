package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis/delayed"
	"github.com/frain-dev/convoy/util"
	disqRedis "github.com/frain-dev/disq/brokers/redis"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addQueueCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Get info about queue",
	}
	cmd.AddCommand(purge(a))
	cmd.AddCommand(getQueueLength(a))
	cmd.AddCommand(getZSETLength(a))
	cmd.AddCommand(getStreamInfo(a))
	cmd.AddCommand(getConsumersInfo(a))
	cmd.AddCommand(getPendingInfo(a))
	cmd.AddCommand(checkEventDeliveryinStream(a))
	cmd.AddCommand(checkEventDeliveryinZSET(a))
	cmd.AddCommand(checkEventDeliveryinPending(a))
	cmd.AddCommand(checkBatchEventDeliveryinStream(a))
	cmd.AddCommand(checkBatchEventDeliveryinZSET(a))
	cmd.AddCommand(checkBatchEventDeliveryinPending(a))
	cmd.AddCommand(exportStreamMessages(a))
	cmd.AddCommand(requeueMessagesinStream(a))
	return cmd
}

func purge(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "purge queue",
		RunE: func(cmd *cobra.Command, args []string) error {
			q := a.eventQueue
			err := q.Broker().(*disqRedis.Stream).Purge()
			if err != nil {
				return err
			}
			log.Infof("Queue purged succesfully.")
			return nil
		},
	}
	return cmd
}

//Get queue length, number of entries in the stream
func getQueueLength(a *app) *cobra.Command {
	var timeInterval int
	cmd := &cobra.Command{
		Use:   "length",
		Short: "queue length",
		RunE: func(cmd *cobra.Command, args []string) error {
			q := a.eventQueue
			ctx := context.Background()
			ticker := time.NewTicker(time.Duration(timeInterval) * time.Millisecond)

			for {
				select {
				case <-ticker.C:
					length, err := q.Broker().Len()
					if err != nil {
						log.Printf("Error getting queue length: %v", err)
					}
					log.Printf("Queue Length: %+v\n", length)
				case <-ctx.Done():
					return nil
				}
			}
		},
	}
	cmd.Flags().IntVar(&timeInterval, "interval", 2000, "Log time interval")
	return cmd
}

//get length of ZSET, delayed msgs
func getZSETLength(a *app) *cobra.Command {
	var timeInterval int
	cmd := &cobra.Command{
		Use:   "zsetlength",
		Short: "get ZSET Length",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			if cfg.Queue.Type != config.RedisQueueProvider {
				log.Fatalf("Queue type error: Command is available for redis queue only.")
			}
			q := a.eventQueue.(*redisqueue.DelayedQueue)
			ctx := context.Background()
			ticker := time.NewTicker(time.Duration(timeInterval) * time.Millisecond)
			for {
				select {
				case <-ticker.C:
					bodies, err := q.ZRangebyScore(ctx, "-inf", "+inf")
					if err != nil {
						log.Printf("Error ZSET Length: %v", err)
					}
					log.Printf("ZSET Length: %+v\n", len(bodies))
				case <-ctx.Done():
					return nil
				}
			}
		},
	}
	cmd.Flags().IntVar(&timeInterval, "interval", 2000, "Log time interval")
	return cmd
}

// Get general stream info
func getStreamInfo(a *app) *cobra.Command {
	var timeInterval int
	cmd := &cobra.Command{
		Use:   "streaminfo",
		Short: "get stream info",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			if cfg.Queue.Type != config.RedisQueueProvider {
				log.Fatalf("Queue type error: Command is available for redis queue only.")
			}
			ctx := context.Background()
			q := a.eventQueue.(*redisqueue.DelayedQueue)
			ticker := time.NewTicker(time.Duration(timeInterval) * time.Millisecond)
			for {
				select {
				case <-ticker.C:
					r, err := q.XInfoStream(ctx).Result()
					if err != nil {
						log.Printf("XInfoStream err: %v", err)
					}
					log.Printf("Stream Info: %+v\n\n", r)
				case <-ctx.Done():
					return nil
				}
			}
		},
	}
	cmd.Flags().IntVar(&timeInterval, "interval", 2000, "Log time interval")
	return cmd
}

//Get info on all consumers
func getConsumersInfo(a *app) *cobra.Command {
	var timeInterval int
	cmd := &cobra.Command{
		Use:   "consumerinfo",
		Short: "get consumers info",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			if cfg.Queue.Type != config.RedisQueueProvider {
				log.Fatalf("Queue type error: Command is available for redis queue only.")
			}
			q := a.eventQueue.(*redisqueue.DelayedQueue)
			ctx := context.Background()
			ticker := time.NewTicker(time.Duration(timeInterval) * time.Millisecond)
			for {
				select {
				case <-ticker.C:
					ci, err := q.XInfoConsumers(ctx).Result()
					if err != nil {
						log.Errorf("XInfoConsumers err: %v", err)
					}
					log.Printf("Consumers Info: %+v\n\n", ci)
				case <-ctx.Done():
					return nil
				}
			}
		},
	}
	cmd.Flags().IntVar(&timeInterval, "interval", 2000, "Log time interval")
	return cmd
}

//Check Pending info
func getPendingInfo(a *app) *cobra.Command {
	var timeInterval int
	cmd := &cobra.Command{
		Use:   "pendinginfo",
		Short: "get Pending info",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			if cfg.Queue.Type != config.RedisQueueProvider {
				log.Fatalf("Queue type error: Command is available for redis queue only.")
			}
			q := a.eventQueue.(*redisqueue.DelayedQueue)
			ctx := context.Background()
			ticker := time.NewTicker(time.Duration(timeInterval) * time.Millisecond)
			for {
				select {
				case <-ticker.C:
					pending, err := q.XPending(ctx)
					if err != nil {
						log.Errorf("Error Pending: %v", err)
					}
					log.Printf("Pending: %+v\n", pending)
				case <-ctx.Done():
					return nil
				}
			}
		},
	}
	cmd.Flags().IntVar(&timeInterval, "interval", 2000, "Log time interval")
	return cmd
}

//Check if eventDelivery is on the queue (stream)
func checkEventDeliveryinStream(a *app) *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "checkstream",
		Short: "Event delivery in stream",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			if cfg.Queue.Type != config.RedisQueueProvider {
				log.Fatalf("Queue type error: Command is available for redis queue only.")
			}
			if util.IsStringEmpty(id) {
				return errors.New("please provide an eventDelivery ID")
			}
			ctx := context.Background()
			q := a.eventQueue.(*redisqueue.DelayedQueue)

			onQueue, err := q.CheckEventDeliveryinStream(ctx, id, "-", "+")
			if err != nil {
				return err
			}

			if onQueue {
				log.Printf("ID: %v on Queue: True", id)
			} else {
				log.Printf("ID: %v on Queue: False", id)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "eventDelivery ID")
	return cmd
}

//Check batch eventDelivery is on the queue (stream)
func checkBatchEventDeliveryinStream(a *app) *cobra.Command {
	var file string
	var outputfile = "inStream_" + uuid.NewString() + ".txt"
	cmd := &cobra.Command{
		Use:   "batchcheckstream",
		Short: "Event delivery in stream",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			if cfg.Queue.Type != config.RedisQueueProvider {
				log.Fatalf("Queue type error: Command is available for redis queue only.")
			}
			if util.IsStringEmpty(file) {
				return errors.New("please provide a file name")
			}
			ctx := context.Background()
			q := a.eventQueue.(*redisqueue.DelayedQueue)
			file, err := os.Open(file)
			if err != nil {
				log.Fatal(err)
			}
			outputfile, err := os.Create(outputfile)
			if err != nil {
				log.Fatalf("failed creating outputfile: %s", err)
			}
			defer outputfile.Close()
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				onQueue, err := q.CheckEventDeliveryinStream(ctx, scanner.Text(), "-", "+")
				if err != nil {
					return err
				}
				out := scanner.Text() + "\t\t" + strconv.FormatBool(onQueue) + "\n"
				_, err = outputfile.WriteString(out)
				if err != nil {
					log.Fatalf("failed writing to file: %s", err)
				}
			}

			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to file with batch IDs")
	return cmd
}

//check if eventDelivery is in ZSET
func checkEventDeliveryinZSET(a *app) *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "checkzset",
		Short: "Event delivery in ZSET",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			if cfg.Queue.Type != config.RedisQueueProvider {
				log.Fatalf("Queue type error: Command is available for redis queue only.")
			}
			if util.IsStringEmpty(id) {
				return errors.New("please provide an eventDelivery ID")
			}
			ctx := context.Background()
			q := a.eventQueue.(*redisqueue.DelayedQueue)

			inZSET, err := q.CheckEventDeliveryinZSET(ctx, id, "-inf", "+inf")
			if err != nil {
				return err
			}

			if inZSET {
				log.Printf("Event ID: %v in inZSET: True", id)
			} else {
				log.Printf("Event ID: %v in inZSET: False", id)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "eventDelivery ID")
	return cmd
}

//check if batch eventDelivery is in ZSET
func checkBatchEventDeliveryinZSET(a *app) *cobra.Command {
	var file string
	var outputfile = "inZset_" + uuid.NewString() + ".txt"
	cmd := &cobra.Command{
		Use:   "batchcheckzset",
		Short: "Batch Event delivery in ZSET",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			if cfg.Queue.Type != config.RedisQueueProvider {
				log.Fatalf("Queue type error: Command is available for redis queue only.")
			}
			if util.IsStringEmpty(file) {
				return errors.New("please provide file containing IDs")
			}
			ctx := context.Background()
			q := a.eventQueue.(*redisqueue.DelayedQueue)
			file, err := os.Open(file)
			if err != nil {
				log.Fatal(err)
			}
			outputfile, err := os.Create(outputfile)
			if err != nil {
				log.Fatalf("failed creating outputfile: %s", err)
			}
			defer outputfile.Close()
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				inZSET, err := q.CheckEventDeliveryinZSET(ctx, scanner.Text(), "-inf", "+inf")
				if err != nil {
					return err
				}
				out := scanner.Text() + "\t\t" + strconv.FormatBool(inZSET) + "\n"
				_, err = outputfile.WriteString(out)
				if err != nil {
					log.Fatalf("failed writing to file: %s", err)
				}
			}

			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to file with batch IDs")
	return cmd
}

//Check if eventDelivery is in pending (stream)
func checkEventDeliveryinPending(a *app) *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "checkpending",
		Short: "Event delivery on pending",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			if cfg.Queue.Type != config.RedisQueueProvider {
				log.Fatalf("Queue type error: Command is available for redis queue only.")
			}
			if util.IsStringEmpty(id) {
				return errors.New("please provide an eventDelivery Id")
			}
			ctx := context.Background()
			q := a.eventQueue.(*redisqueue.DelayedQueue)
			inPending, err := q.CheckEventDeliveryinPending(ctx, id)
			if err != nil {
				log.Printf("Error fetching Pending: %v", err)
			}
			if inPending {
				log.Printf("ID: %v in Pending: True", id)
			} else {
				log.Printf("ID: %v in Pending: False", id)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "eventDelivery ID")
	return cmd
}

//Check if eventDelivery is in pending (stream)
func checkBatchEventDeliveryinPending(a *app) *cobra.Command {
	var file string
	var outputfile = "inPending_" + uuid.NewString() + ".txt"
	cmd := &cobra.Command{
		Use:   "batchcheckpending",
		Short: "Event delivery on pending",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			if cfg.Queue.Type != config.RedisQueueProvider {
				log.Fatalf("Queue type error: Command is available for redis queue only.")
			}
			if util.IsStringEmpty(file) {
				return errors.New("please provide file containing batch Ids")
			}
			ctx := context.Background()
			q := a.eventQueue.(*redisqueue.DelayedQueue)
			file, err := os.Open(file)
			if err != nil {
				log.Fatal(err)
			}
			outputfile, err := os.Create(outputfile)
			if err != nil {
				log.Fatalf("failed creating outputfile: %s", err)
			}
			defer outputfile.Close()
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				inPending, err := q.CheckEventDeliveryinPending(ctx, scanner.Text())
				if err != nil {
					return err
				}
				out := scanner.Text() + "\t\t" + strconv.FormatBool(inPending) + "\n"
				_, err = outputfile.WriteString(out)
				if err != nil {
					log.Fatalf("failed writing to file: %s", err)
				}
			}

			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to file with batch IDs")
	return cmd
}

//export messages on the stream.
func exportStreamMessages(a *app) *cobra.Command {
	var outputfile = "stream_" + uuid.NewString() + ".csv"
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export messages from redis stream",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			if cfg.Queue.Type != config.RedisQueueProvider {
				log.Fatalf("Queue type error: Command is available for redis queue only.")
			}

			ctx := context.Background()
			q := a.eventQueue.(*redisqueue.DelayedQueue)
			outputfile, err := os.Create(outputfile)
			if err != nil {
				log.Errorf("failed creating outputfile: %s", err)
			}
			defer outputfile.Close()

			msgs, err := q.ExportMessagesfromStream(ctx)
			if err != nil {
				return err
			}

			if len(msgs) > 0 {
				ids := make([]string, len(msgs))
				for i := range msgs {
					msg := &msgs[i]
					value := string(msg.ArgsBin[convoy.EventDeliveryIDLength:])
					ids[i] = value

				}
				deliveries, err := a.eventDeliveryRepo.FindEventDeliveriesByIDs(ctx, ids)

				if err != nil {
					log.Errorf("failed fetch to file: %s", err)
				}
				csvwriter := csv.NewWriter(outputfile)

				for i := range deliveries {
					d := &deliveries[i]
					data := []string{d.UID, d.AppMetadata.Title, string(d.Status), d.CreatedAt.Time().String()}

					err = csvwriter.Write(data)
					if err != nil {
						log.Errorf("failed writing to file: %s", err)
					}
				}
				csvwriter.Flush()
				outputfile.Close()
			}

			return nil
		},
	}
	return cmd
}

//requeue all messages on the stream.
func requeueMessagesinStream(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "requeue",
		Aliases: []string{"req"},
		Short:   "Requeue all messages on redis stream",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}
			if cfg.Queue.Type != config.RedisQueueProvider {
				log.Fatalf("Queue type error: Command is available for redis queue only.")
			}

			ctx := context.Background()
			q := a.eventQueue.(*redisqueue.DelayedQueue)

			msgs, err := q.ExportMessagesfromStreamXACK(ctx)
			if err != nil {
				return err
			}

			if len(msgs) > 0 {
				for i := range msgs {
					msg := &msgs[i]

					value := string(msg.ArgsBin[convoy.EventDeliveryIDLength:])

					d, err := a.eventDeliveryRepo.FindEventDeliveryByID(ctx, value)
					if err != nil {
						return err
					}
					group, err := a.groupRepo.FetchGroupByID(ctx, d.AppMetadata.GroupID)
					if err != nil {
						log.WithError(err).Errorf("count: %s failed to fetch group %s for delivery %s", fmt.Sprint(i), d.AppMetadata.GroupID, d.UID)
						continue
					}
					taskName := convoy.EventProcessor.SetPrefix(group.Name)
					job := &queue.Job{
						ID:            d.UID,
						EventDelivery: d,
					}
					err = q.Publish(ctx, taskName, job, 1*time.Second)
					if err != nil {
						log.WithError(err).Errorf("count: %s failed to send event delivery %s to the queue", fmt.Sprint(i), d.ID)
					}
					log.Infof("count: %s sucessfully requeued delivery with id: %s", fmt.Sprint(i), value)

				}
			}

			return nil
		},
	}
	return cmd
}
