package main

import (
	"context"
	"errors"
	"log"
	"time"

	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/util"
	"github.com/go-redis/redis/v8"
	"github.com/spf13/cobra"
	"github.com/vmihailenco/taskq/v3"
)

const argsSlice = 12

func addQueueCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Get info about queue",
	}

	cmd.AddCommand(getQueueLength(a))
	cmd.AddCommand(getZSETLength(a))
	cmd.AddCommand(getStreamInfo(a))
	cmd.AddCommand(getConsumersInfo(a))
	cmd.AddCommand(getPendingInfo(a))
	cmd.AddCommand(checkEventDeliveryonQueue(a))
	cmd.AddCommand(checkEventDeliveryinZSET(a))
	cmd.AddCommand(checkEventDeliveryinPending(a))
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
					length, err := q.Consumer().Queue().Len()
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
			q := a.eventQueue.(*redisqueue.RedisQueue)
			redisStreamClient := q.Inner
			ctx := context.Background()
			ticker := time.NewTicker(time.Duration(timeInterval) * time.Millisecond)
			zset := "taskq:" + "{" + q.Name + "}:zset"
			for {
				select {
				case <-ticker.C:
					bodies, err := redisStreamClient.ZRangeByScore(ctx, zset, &redis.ZRangeBy{
						Min: "-inf",
						Max: "+inf",
					}).Result()
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
			ctx := context.Background()
			q := a.eventQueue.(*redisqueue.RedisQueue)
			redisStreamClient := q.Inner
			ticker := time.NewTicker(time.Duration(timeInterval) * time.Millisecond)
			stream := "taskq:" + "{" + q.Name + "}:stream"
			for {
				select {
				case <-ticker.C:
					r, err := redisStreamClient.XInfoStream(ctx, stream).Result()
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
			q := a.eventQueue.(*redisqueue.RedisQueue)
			redisStreamClient := q.Inner
			ctx := context.Background()
			ticker := time.NewTicker(time.Duration(timeInterval) * time.Millisecond)
			stream := "taskq:" + "{" + q.Name + "}:stream"
			streamGroup := "taskq"
			for {
				select {
				case <-ticker.C:
					r, err := redisStreamClient.XInfoConsumers(ctx, stream, streamGroup).Result()
					if err != nil {
						log.Printf("XInfoConsumers err: %v", err)
					}
					log.Printf("Consumers Info: %+v\n\n", r)
				case <-ctx.Done():
					return nil
				}
			}
		},
	}
	cmd.Flags().IntVar(&timeInterval, "interval", 2000, "Log time interval")
	return cmd
}

//Check length of Pending
func getPendingInfo(a *app) *cobra.Command {
	var timeInterval int
	cmd := &cobra.Command{
		Use:   "pendinginfo",
		Short: "get Pending info",
		RunE: func(cmd *cobra.Command, args []string) error {
			q := a.eventQueue.(*redisqueue.RedisQueue)
			redisStreamClient := q.Inner
			ctx := context.Background()
			ticker := time.NewTicker(time.Duration(timeInterval) * time.Millisecond)
			stream := "taskq:" + "{" + q.Name + "}:stream"
			streamGroup := "taskq"
			for {
				select {
				case <-ticker.C:
					pending, err := redisStreamClient.XPending(ctx, stream, streamGroup).Result()
					if err != nil {
						log.Printf("Error Pending: %v", err)
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
func checkEventDeliveryonQueue(a *app) *cobra.Command {
	var id string
	var idType string
	cmd := &cobra.Command{
		Use:   "checkqueue",
		Short: "Event delivery on queue",
		RunE: func(cmd *cobra.Command, args []string) error {
			if util.IsStringEmpty(id) {
				return errors.New("please provide an eventDelivery ID or equivalent taskq.Message ID")
			}
			ctx := context.Background()
			q := a.eventQueue.(*redisqueue.RedisQueue)
			redisStreamClient := q.Inner
			stream := "taskq:" + "{" + q.Name + "}:stream"
			xmsgs, err := redisStreamClient.XRange(ctx, stream, "-", "+").Result()
			if err != nil {
				return err
			}

			onQueue := false

			msgs := make([]taskq.Message, len(xmsgs))
			for i := range xmsgs {
				xmsg := &xmsgs[i]
				msg := &msgs[i]

				err = unmarshalMessage(msg, xmsg)

				if err != nil {
					return err
				}
				switch idType {
				case "eventdev":
					value := string(msg.ArgsBin[argsSlice:])
					if value == id {
						onQueue = true
					}
				case "msg":
					if msg.ID == id {
						onQueue = true
					}
				}

			}
			if onQueue {
				log.Printf("ID: %v on Queue: True", id)
			} else {
				log.Printf("ID: %v on Queue: False", id)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "eventDelivery ID or taskq.Message ID")
	cmd.Flags().StringVar(&idType, "type", "eventdev", "eventdev or msg")
	return cmd
}

//check if eventDelivery is in ZSET
func checkEventDeliveryinZSET(a *app) *cobra.Command {
	var id string
	var idType string
	cmd := &cobra.Command{
		Use:   "checkzset",
		Short: "Event delivery in ZSET",
		RunE: func(cmd *cobra.Command, args []string) error {
			if util.IsStringEmpty(id) {
				return errors.New("please provide an eventDelivery ID or equivalent taskq.Message ID")
			}
			ctx := context.Background()
			q := a.eventQueue.(*redisqueue.RedisQueue)
			redisStreamClient := q.Inner
			zset := "taskq:" + "{" + q.Name + "}:zset"
			bodies, err := redisStreamClient.ZRangeByScore(ctx, zset, &redis.ZRangeBy{
				Min: "-inf",
				Max: "+inf",
			}).Result()
			if err != nil {
				return err
			}

			inZSET := false

			var msg taskq.Message
			for _, body := range bodies {
				err := msg.UnmarshalBinary([]byte(body))
				if err != nil {
					return err
				}
				switch idType {
				case "eventdev":
					value := string(msg.ArgsBin[argsSlice:])
					if value == id {
						inZSET = true
					}
				case "msg":
					if msg.ID == id {
						inZSET = true
					}
				}
			}
			if inZSET {
				log.Printf("Event ID: %v in inZSET: True", id)
			} else {
				log.Printf("Event ID: %v in inZSET: False", id)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "eventDelivery ID or taskq.Message ID")
	cmd.Flags().StringVar(&idType, "type", "eventdev", "eventdev or msg")
	return cmd
}

//Check if eventDelivery is in pending (stream)
func checkEventDeliveryinPending(a *app) *cobra.Command {
	var id string
	var idType string
	cmd := &cobra.Command{
		Use:   "checkpending",
		Short: "Event delivery on pending",
		RunE: func(cmd *cobra.Command, args []string) error {
			if util.IsStringEmpty(id) {
				return errors.New("please provide an eventDelivery Id or taskq.Message ID")
			}
			ctx := context.Background()
			q := a.eventQueue.(*redisqueue.RedisQueue)
			redisStreamClient := q.Inner
			stream := "taskq:" + "{" + q.Name + "}:stream"
			streamGroup := "taskq"
			pending, err := redisStreamClient.XPendingExt(ctx, &redis.XPendingExtArgs{
				Stream: stream,
				Group:  streamGroup,
				Start:  "-",
				End:    "+",
			}).Result()
			if err != nil {
				log.Printf("Error fetching Pending: %v", err)
			}

			inPending := false

			var msg *taskq.Message
			for i := range pending {
				xmsgInfo := &pending[i]
				id := xmsgInfo.ID

				xmsgs, err := redisStreamClient.XRangeN(ctx, stream, id, id, 1).Result()
				if err != nil {
					return err
				}

				if len(xmsgs) != 1 {
					log.Printf("redisq: can't find pending message id=%q in stream=%q", id, stream)
				}
				err = unmarshalMessage(msg, &xmsgs[0])
				if err != nil {
					return err
				}
				switch idType {
				case "eventdev":
					value := string(msg.ArgsBin[argsSlice:])
					if value == id {
						inPending = true
					}
				case "msg":
					if msg.ID == id {
						inPending = true
					}
				}

			}
			if inPending {
				log.Printf("ID: %v in Pending: True", id)
			} else {
				log.Printf("ID: %v in Pending: False", id)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "eventDelivery ID or taskq.Message ID")
	cmd.Flags().StringVar(&idType, "type", "eventdev", "eventdev or msg")
	return cmd
}
func unmarshalMessage(msg *taskq.Message, xmsg *redis.XMessage) error {
	body := xmsg.Values["body"].(string)
	err := msg.UnmarshalBinary([]byte(body))
	if err != nil {
		return err
	}

	msg.ID = xmsg.ID
	return nil
}
