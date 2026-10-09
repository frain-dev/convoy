package exporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func TestStreamExport_DisabledWebhookArchiving(t *testing.T) {
	ex, err := NewExporterWithWindow(
		nil, nil,
		&datastore.Configuration{
			WebhookArchiving: &datastore.WebhookArchivingConfiguration{Enabled: false},
		},
		nil,
		time.Now().Add(-time.Hour), time.Now(),
		log.New("test", log.LevelError),
	)
	require.NoError(t, err)

	result, err := ex.StreamExport(context.Background(), nil)
	require.NoError(t, err)
	require.Nil(t, result)
}

func TestExport_DisabledWebhookArchiving(t *testing.T) {
	ex, err := NewExporterWithWindow(
		nil, nil,
		&datastore.Configuration{
			WebhookArchiving: &datastore.WebhookArchivingConfiguration{Enabled: false},
		},
		nil,
		time.Now().Add(-time.Hour), time.Now(),
		log.New("test", log.LevelError),
	)
	require.NoError(t, err)

	result, err := ex.Export(context.Background())
	require.NoError(t, err)
	require.Nil(t, result)
}
