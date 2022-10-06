package flipt

import (
	"context"
	"errors"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	flipt "go.flipt.io/flipt-grpc"
	"google.golang.org/grpc"
)

var (
	ErrFliptServerError  = errors.New("something went wrong with the flipt server")
	ErrFliptFlagNotFound = errors.New("flag not found")
)

type Flipt struct {
	client flipt.FliptClient
}

func NewFliptClient(host string) (*Flipt, error) {
	conn, err := grpc.Dial(host, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	log.Infof("connected to flipt Server at: %s", host)

	client := flipt.NewFliptClient(conn)

	return &Flipt{
		client: client,
	}, nil
}

func (f *Flipt) IsEnabled(flagKey string, evaluate map[string]string) (bool, error) {
	flag, err := f.client.Evaluate(context.Background(), &flipt.EvaluationRequest{
		FlagKey:  flagKey,
		EntityId: uuid.NewString(),
		Context:  evaluate,
	})

	if err != nil {
		log.WithError(err).Errorf("failed to connect to flipt server")
		return false, ErrFliptServerError
	}

	if flag == nil {
		return false, ErrFliptFlagNotFound
	}

	return flag.Match, nil
}
