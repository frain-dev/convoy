package blobstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/testenv"
)

var infra *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(
		context.Background(),
		testenv.WithoutPostgres(),
		testenv.WithoutRedis(),
		testenv.WithMinIO(),
		testenv.WithAzurite(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch test infrastructure: %v\n", err)
		os.Exit(1)
	}

	infra = res
	code := m.Run()

	if err := cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cleanup: %v\n", err)
	}

	os.Exit(code)
}

// ============================================================================
// OnPrem Tests
// ============================================================================

func TestOnPremClient_Upload(t *testing.T) {
	tmpDir := t.TempDir()
	logger := log.New("test", log.LevelInfo)

	client, err := NewOnPremClient(BlobStoreOptions{OnPremStorageDir: tmpDir}, logger)
	require.NoError(t, err)

	content := "hello world\n"
	err = client.Upload(context.Background(), "test/file.txt", strings.NewReader(content))
	require.NoError(t, err)

	// Verify file exists with correct content
	data, err := os.ReadFile(filepath.Join(tmpDir, "test/file.txt"))
	require.NoError(t, err)
	require.Equal(t, content, string(data))
}

func TestOnPremClient_Upload_CreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	logger := log.New("test", log.LevelInfo)

	client, err := NewOnPremClient(BlobStoreOptions{OnPremStorageDir: tmpDir}, logger)
	require.NoError(t, err)

	err = client.Upload(context.Background(), "deeply/nested/path/file.jsonl.gz", strings.NewReader("data"))
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(tmpDir, "deeply/nested/path/file.jsonl.gz"))
	require.NoError(t, err)
}

// ============================================================================
// S3 Tests (via MinIO)
// ============================================================================

func TestS3Client_Upload(t *testing.T) {
	if infra.NewMinIOClient == nil {
		t.Skip("MinIO not available")
	}

	minioClient, endpoint, err := (*infra.NewMinIOClient)(t)
	require.NoError(t, err)

	logger := log.New("test", log.LevelInfo)
	s3Client, err := NewS3Client(BlobStoreOptions{
		Bucket:    "convoy-test-exports",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Region:    "us-east-1",
		Endpoint:  "http://" + endpoint,
	}, logger)
	require.NoError(t, err)

	content := `{"uid":"123","event_type":"test"}` + "\n"
	err = s3Client.Upload(context.Background(), "test/events.jsonl", strings.NewReader(content))
	require.NoError(t, err)

	// Download and verify via MinIO client
	obj, err := minioClient.GetObject(context.Background(), "convoy-test-exports", "test/events.jsonl", minio.GetObjectOptions{})
	require.NoError(t, err)
	defer obj.Close()

	data, err := io.ReadAll(obj)
	require.NoError(t, err)
	require.Equal(t, content, string(data))
}

func TestS3Client_Upload_WithPrefix(t *testing.T) {
	if infra.NewMinIOClient == nil {
		t.Skip("MinIO not available")
	}

	_, endpoint, err := (*infra.NewMinIOClient)(t)
	require.NoError(t, err)

	logger := log.New("test", log.LevelInfo)
	s3Client, err := NewS3Client(BlobStoreOptions{
		Bucket:    "convoy-test-exports",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Region:    "us-east-1",
		Endpoint:  "http://" + endpoint,
		Prefix:    "backups",
	}, logger)
	require.NoError(t, err)

	err = s3Client.Upload(context.Background(), "events.jsonl", strings.NewReader("data"))
	require.NoError(t, err)

	// The object should be stored at "backups/events.jsonl"
	minioClient, _, err := (*infra.NewMinIOClient)(t)
	require.NoError(t, err)

	_, err = minioClient.StatObject(context.Background(), "convoy-test-exports", "backups/events.jsonl", minio.StatObjectOptions{})
	require.NoError(t, err)
}

// ============================================================================
// Azure Tests (via Azurite)
// ============================================================================

func TestAzureBlobClient_Upload(t *testing.T) {
	if infra.NewAzuriteClient == nil {
		t.Skip("Azurite not available")
	}

	azClient, endpoint, err := (*infra.NewAzuriteClient)(t)
	require.NoError(t, err)

	logger := log.New("test", log.LevelInfo)
	blobClient, err := NewAzureBlobClient(BlobStoreOptions{
		AzureAccountName:   "devstoreaccount1",
		AzureAccountKey:    "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==",
		AzureContainerName: "convoy-test-exports",
		AzureEndpoint:      endpoint,
	}, logger)
	require.NoError(t, err)

	content := `{"uid":"456","event_type":"azure.test"}` + "\n"
	err = blobClient.Upload(context.Background(), "test/events.jsonl", strings.NewReader(content))
	require.NoError(t, err)

	// Download and verify via Azure client
	resp, err := azClient.DownloadStream(context.Background(), "convoy-test-exports", "test/events.jsonl", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, content, string(data))
}

func TestAzureBlobClient_Upload_WithPrefix(t *testing.T) {
	if infra.NewAzuriteClient == nil {
		t.Skip("Azurite not available")
	}

	azClient, endpoint, err := (*infra.NewAzuriteClient)(t)
	require.NoError(t, err)

	logger := log.New("test", log.LevelInfo)
	blobClient, err := NewAzureBlobClient(BlobStoreOptions{
		AzureAccountName:   "devstoreaccount1",
		AzureAccountKey:    "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==",
		AzureContainerName: "convoy-test-exports",
		AzureEndpoint:      endpoint,
		Prefix:             "prefixed",
	}, logger)
	require.NoError(t, err)

	err = blobClient.Upload(context.Background(), "data.jsonl", strings.NewReader("prefixed data"))
	require.NoError(t, err)

	// Verify stored at "prefixed/data.jsonl"
	resp, err := azClient.DownloadStream(context.Background(), "convoy-test-exports", "prefixed/data.jsonl", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "prefixed data", string(data))
}

// ============================================================================
// Factory Tests
// ============================================================================

func TestNewBlobStoreClient_S3(t *testing.T) {
	if infra.NewMinIOClient == nil {
		t.Skip("MinIO not available")
	}

	_, endpoint, err := (*infra.NewMinIOClient)(t)
	require.NoError(t, err)

	logger := log.New("test", log.LevelInfo)
	client, err := NewBlobStoreClient(&datastore.StoragePolicyConfiguration{
		Type: datastore.S3,
		S3: &datastore.S3Storage{
			Bucket:    null.NewString("convoy-test-exports", true),
			AccessKey: null.NewString("minioadmin", true),
			SecretKey: null.NewString("minioadmin", true),
			Region:    null.NewString("us-east-1", true),
			Endpoint:  null.NewString("http://"+endpoint, true),
		},
	}, logger)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Verify it works
	err = client.Upload(context.Background(), "factory-test/s3.txt", strings.NewReader("factory s3"))
	require.NoError(t, err)
}

func TestNewBlobStoreClient_OnPrem(t *testing.T) {
	tmpDir := t.TempDir()
	logger := log.New("test", log.LevelInfo)

	client, err := NewBlobStoreClient(&datastore.StoragePolicyConfiguration{
		Type: datastore.OnPrem,
		OnPrem: &datastore.OnPremStorage{
			Path: null.NewString(tmpDir, true),
		},
	}, logger)
	require.NoError(t, err)
	require.NotNil(t, client)

	err = client.Upload(context.Background(), "factory-test.txt", strings.NewReader("factory onprem"))
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tmpDir, "factory-test.txt"))
	require.NoError(t, err)
	require.Equal(t, "factory onprem", string(data))
}

func TestNewBlobStoreClient_Azure(t *testing.T) {
	if infra.NewAzuriteClient == nil {
		t.Skip("Azurite not available")
	}

	_, endpoint, err := (*infra.NewAzuriteClient)(t)
	require.NoError(t, err)

	logger := log.New("test", log.LevelInfo)
	client, err := NewBlobStoreClient(&datastore.StoragePolicyConfiguration{
		Type: datastore.AzureBlob,
		AzureBlob: &datastore.AzureBlobStorage{
			AccountName:   null.NewString("devstoreaccount1", true),
			AccountKey:    null.NewString("Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==", true),
			ContainerName: null.NewString("convoy-test-exports", true),
			Endpoint:      null.NewString(endpoint, true),
		},
	}, logger)
	require.NoError(t, err)
	require.NotNil(t, client)

	err = client.Upload(context.Background(), "factory-test/azure.txt", strings.NewReader("factory azure"))
	require.NoError(t, err)
}

func TestNewBlobStoreClient_InvalidType(t *testing.T) {
	logger := log.New("test", log.LevelInfo)
	_, err := NewBlobStoreClient(&datastore.StoragePolicyConfiguration{
		Type: "invalid",
	}, logger)
	require.Error(t, err)
}

// Ensure imports are used
var _ = (*minio.Client)(nil)
var _ = (*azblob.Client)(nil)
var _ = (*bytes.Buffer)(nil)
var _ = credentials.NewStaticV4
