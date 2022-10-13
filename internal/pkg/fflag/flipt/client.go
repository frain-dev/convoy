package flipt

import (
	"context"
	"errors"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	flipt "go.flipt.io/flipt-grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	ErrFliptServerError  = errors.New("something went wrong with the flipt server")
	ErrFliptFlagNotFound = errors.New("flag not found")
)

type Flipt struct {
	client flipt.FliptClient
	conn   *grpc.ClientConn
}

func NewFliptClient(host string) (*Flipt, error) {
	conn, err := grpc.Dial(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	log.Infof("connected to flipt Server at: %s", host)

	client := flipt.NewFliptClient(conn)

	return &Flipt{
		client: client,
		conn:   conn,
	}, nil
}

func (f *Flipt) IsEnabled(flagKey string, evaluate map[string]string) (bool, error) {
	flag, err := f.client.GetFlag(context.Background(), &flipt.GetFlagRequest{
		Key: flagKey,
	})

	if err != nil {
		log.WithError(err).Error("failed to connect to flipt server")
		return false, ErrFliptServerError
	}

	if flag == nil {
		return false, ErrFliptFlagNotFound
	}

	// The flag not being enabled means everybody has
	// access to that feature
	if !flag.Enabled {
		return true, nil
	}

	result, err := f.client.Evaluate(context.Background(), &flipt.EvaluationRequest{
		FlagKey:  flagKey,
		EntityId: uuid.NewString(),
		Context:  evaluate,
	})

	if err != nil {
		log.WithError(err).Error("failed to connect to flipt server")
		return false, ErrFliptServerError
	}

	return result.Match, nil
}

func (f *Flipt) Disconnect() error {
	return f.conn.Close()
}
