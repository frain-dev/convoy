package backup_collector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"

	blobstore "github.com/frain-dev/convoy/internal/pkg/blob-store"
	log "github.com/frain-dev/convoy/pkg/logger"
)

const (
	defaultSlotName       = "convoy_backup"
	defaultPublication    = "convoy_backup"
	standbyStatusInterval = 10 * time.Second
	receiveTimeout        = 5 * time.Second
)

// BackupCollector streams WAL changes from PostgreSQL for the backup tables
// and periodically flushes them as gzip-compressed JSONL to a BlobStore.
type BackupCollector struct {
	pool        *pgxpool.Pool
	dsn         string
	slotName    string
	publication string

	replConn   *pgconn.PgConn
	relations  map[uint32]*pglogrepl.RelationMessage
	clientLSN  pglogrepl.LSN
	flushedLSN pglogrepl.LSN

	buffer *Buffer
	store  blobstore.BlobStore

	flushInterval time.Duration

	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger log.Logger
}

// NewBackupCollector creates a new CDC-based backup collector.
func NewBackupCollector(
	pool *pgxpool.Pool,
	dsn string,
	store blobstore.BlobStore,
	flushInterval time.Duration,
	logger log.Logger,
) *BackupCollector {
	return &BackupCollector{
		pool:          pool,
		dsn:           dsn,
		slotName:      defaultSlotName,
		publication:   defaultPublication,
		relations:     make(map[uint32]*pglogrepl.RelationMessage),
		buffer:        NewBuffer(),
		store:         store,
		flushInterval: flushInterval,
		logger:        logger,
	}
}

// Start initialises the replication slot (if needed) and begins streaming
// WAL changes + periodic flushing in background goroutines.
func (c *BackupCollector) Start(ctx context.Context) error {
	replConn, err := c.connectReplication(ctx)
	if err != nil {
		return fmt.Errorf("replication connect: %w", err)
	}
	c.replConn = replConn

	// Check if slot already exists (restart case)
	var startLSN pglogrepl.LSN
	var restartLSNStr *string

	err = c.pool.QueryRow(ctx,
		"SELECT restart_lsn::text FROM pg_replication_slots WHERE slot_name = $1",
		c.slotName,
	).Scan(&restartLSNStr)

	if err == nil && restartLSNStr != nil {
		// Slot exists — resume from restart LSN
		startLSN, err = pglogrepl.ParseLSN(*restartLSNStr)
		if err != nil {
			c.replConn.Close(ctx)
			return fmt.Errorf("parse restart LSN: %w", err)
		}
		c.logger.Info(fmt.Sprintf("resuming from existing slot %q at LSN %s", c.slotName, startLSN))
	} else {
		// Slot does not exist — create it
		result, createErr := pglogrepl.CreateReplicationSlot(
			ctx, c.replConn, c.slotName, "pgoutput",
			pglogrepl.CreateReplicationSlotOptions{
				Temporary:      false,
				SnapshotAction: "EXPORT_SNAPSHOT",
			},
		)
		if createErr != nil {
			c.replConn.Close(ctx)
			return fmt.Errorf("create replication slot: %w", createErr)
		}

		startLSN, err = pglogrepl.ParseLSN(result.ConsistentPoint)
		if err != nil {
			c.replConn.Close(ctx)
			return fmt.Errorf("parse consistent point LSN: %w", err)
		}

		c.logger.Info(fmt.Sprintf("created replication slot %q at LSN %s (snapshot: %s)",
			result.SlotName, startLSN, result.SnapshotName))
	}

	c.clientLSN = startLSN
	c.flushedLSN = startLSN

	err = pglogrepl.StartReplication(
		ctx, c.replConn, c.slotName, startLSN,
		pglogrepl.StartReplicationOptions{
			PluginArgs: []string{
				"proto_version '1'",
				fmt.Sprintf("publication_names '%s'", c.publication),
			},
		},
	)
	if err != nil {
		c.replConn.Close(ctx)
		return fmt.Errorf("start replication: %w", err)
	}

	streamCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(2)
	go c.streamLoop(streamCtx)
	go c.flushLoop(streamCtx)

	c.logger.Info("backup collector started — streaming WAL changes")
	return nil
}

// Stop cancels the streaming goroutines and closes the replication connection.
func (c *BackupCollector) Stop(ctx context.Context) {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()

	if c.replConn != nil {
		if err := c.replConn.Close(ctx); err != nil {
			c.logger.Warn(fmt.Sprintf("close replication connection: %v", err))
		}
	}
	c.logger.Info("backup collector stopped")
}

// connectReplication opens a pgconn connection with the replication protocol.
func (c *BackupCollector) connectReplication(ctx context.Context) (*pgconn.PgConn, error) {
	cfg, err := pgconn.ParseConfig(c.dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.RuntimeParams["replication"] = "database"
	return pgconn.ConnectConfig(ctx, cfg)
}

// streamLoop receives WAL messages and buffers INSERT records.
func (c *BackupCollector) streamLoop(ctx context.Context) {
	defer c.wg.Done()

	nextStandbyDeadline := time.Now().Add(standbyStatusInterval)

	for {
		if ctx.Err() != nil {
			return
		}

		if time.Now().After(nextStandbyDeadline) {
			if err := c.sendStandbyStatus(ctx); err != nil {
				c.logger.Warn(fmt.Sprintf("send standby status: %v", err))
			}
			nextStandbyDeadline = time.Now().Add(standbyStatusInterval)
		}

		recvCtx, cancel := context.WithDeadline(ctx, time.Now().Add(receiveTimeout))
		rawMsg, err := c.replConn.ReceiveMessage(recvCtx)
		cancel()

		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if pgconn.Timeout(err) || recvCtx.Err() != nil {
				continue
			}
			c.logger.Error(fmt.Sprintf("receive WAL message: %v", err))
			return
		}

		if errMsg, ok := rawMsg.(*pgproto3.ErrorResponse); ok {
			c.logger.Error(fmt.Sprintf("WAL stream error: severity=%s message=%s code=%s",
				errMsg.Severity, errMsg.Message, string(errMsg.Code)))
			return
		}

		msg, ok := rawMsg.(*pgproto3.CopyData)
		if !ok {
			continue
		}

		switch msg.Data[0] {
		case pglogrepl.XLogDataByteID:
			xld, parseErr := pglogrepl.ParseXLogData(msg.Data[1:])
			if parseErr != nil {
				c.logger.Error(fmt.Sprintf("parse XLogData: %v", parseErr))
				continue
			}
			c.handleXLogData(xld)

		case pglogrepl.PrimaryKeepaliveMessageByteID:
			pkm, parseErr := pglogrepl.ParsePrimaryKeepaliveMessage(msg.Data[1:])
			if parseErr != nil {
				c.logger.Error(fmt.Sprintf("parse keepalive: %v", parseErr))
				continue
			}
			if pkm.ServerWALEnd > c.clientLSN {
				c.clientLSN = pkm.ServerWALEnd
			}
			if pkm.ReplyRequested {
				if statusErr := c.sendStandbyStatus(ctx); statusErr != nil {
					c.logger.Warn(fmt.Sprintf("send standby status (reply requested): %v", statusErr))
				}
				nextStandbyDeadline = time.Now().Add(standbyStatusInterval)
			}
		}
	}
}

func (c *BackupCollector) sendStandbyStatus(ctx context.Context) error {
	return pglogrepl.SendStandbyStatusUpdate(ctx, c.replConn, pglogrepl.StandbyStatusUpdate{
		WALWritePosition: c.flushedLSN,
	})
}

func (c *BackupCollector) handleXLogData(xld pglogrepl.XLogData) {
	logicalMsg, err := pglogrepl.Parse(xld.WALData)
	if err != nil {
		c.logger.Warn(fmt.Sprintf("parse logical message: %v", err))
		return
	}

	switch m := logicalMsg.(type) {
	case *pglogrepl.RelationMessage:
		c.relations[m.RelationID] = m
		c.logger.Info(fmt.Sprintf("CDC relation: %s.%s (id=%d, cols=%d)", m.Namespace, m.RelationName, m.RelationID, len(m.Columns)))

	case *pglogrepl.InsertMessage:
		rel, ok := c.relations[m.RelationID]
		if !ok {
			c.logger.Warn(fmt.Sprintf("CDC insert for unknown relation %d", m.RelationID))
			return
		}
		values := tupleToMap(rel, m.Tuple)
		if values != nil {
			c.buffer.Append(rel.RelationName, values, xld.WALStart+pglogrepl.LSN(len(xld.WALData)))
		}

	case *pglogrepl.BeginMessage, *pglogrepl.CommitMessage,
		*pglogrepl.UpdateMessage, *pglogrepl.DeleteMessage:
		// Ignored — we only back up INSERTs
	}

	if xld.WALStart > 0 {
		newLSN := xld.WALStart + pglogrepl.LSN(len(xld.WALData))
		if newLSN > c.clientLSN {
			c.clientLSN = newLSN
		}
	}
}

// tupleToMap converts a pglogrepl TupleData into a map of column name → text value.
func tupleToMap(rel *pglogrepl.RelationMessage, tuple *pglogrepl.TupleData) map[string]string {
	if tuple == nil {
		return nil
	}
	values := make(map[string]string, len(rel.Columns))
	for i, col := range rel.Columns {
		if i >= len(tuple.Columns) {
			break
		}
		tc := tuple.Columns[i]
		switch tc.DataType {
		case pglogrepl.TupleDataTypeText:
			values[col.Name] = string(tc.Data)
		case pglogrepl.TupleDataTypeNull:
			// null — skip
		case pglogrepl.TupleDataTypeToast:
			// unchanged toast — skip (shouldn't happen for INSERTs)
		}
	}
	return values
}
