package exporter

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/sirupsen/logrus"
)

type DBExporter interface {
	Export() (int64, error)
}

type Exporter struct {
	Args      []string
	TableName string
	ProjectID string
	CreatedAt time.Time
	Out       string
}

func (ex *Exporter) Export(ctx context.Context, exportRepo datastore.ExportRepository) (int64, error) {
	writer, err := GetOutputWriter(ex.Out)
	if err != nil {
		log.WithError(err).Error("error opening output stream")
		return 0, err
	}

	if writer == nil {
		writer = os.Stdout
	} else {
		defer writer.Close()
	}

	numDocs, err := exportRepo.ExportRecords(ctx, ex.TableName, ex.ProjectID, ex.CreatedAt, writer)
	if err != nil {
		log.WithError(err).Error("failed to export records")
		return 0, err
	}

	log.Printf("exported %v record(s) from %v", numDocs, ex.TableName)

	return numDocs, nil
}

// GetOutputWriter opens and returns an io.WriteCloser for the output
// options or nil if none is set. The caller is responsible for closing it.
func GetOutputWriter(out string) (io.WriteCloser, error) {
	// If the directory in which the output file is to be
	// written does not exist, create it
	fileDir := filepath.Dir(out)
	err := os.MkdirAll(fileDir, 0o750)
	if err != nil {
		return nil, err
	}

	file, err := os.Create(toUniversalPath(out))
	if err != nil {
		return nil, err
	}
	return file, err
}

func toUniversalPath(path string) string {
	return filepath.FromSlash(path)
}
