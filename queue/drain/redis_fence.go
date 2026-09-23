package drain

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

const redisFenceKey = "__convoy_queue_control:ownership"

// RedisFence requires a dedicated, standalone Redis instance with persistent
// ACLs and separate normal-writer, executor and controller users. Other Redis
// topologies need their own authority-revocation implementation; a successful
// inspection does not make this adapter suitable for them.
type RedisFence struct {
	Repository     *Repository
	Admin          *redis.Client
	Scope          string
	StoreID        string
	ProducerUser   string
	ExecutorUser   string
	ControllerUser string
}

type redisFenceRecord struct {
	Scope       string `json:"scope"`
	StoreID     string `json:"store_id"`
	Producer    string `json:"producer"`
	Executor    string `json:"executor"`
	Controller  string `json:"controller"`
	OperationID string `json:"operation_id"`
	Epoch       int64  `json:"epoch"`
	Revision    int64  `json:"revision"`
	Closed      bool   `json:"closed"`
	Verified    bool   `json:"verified"`
}

func (f RedisFence) identity() redisFenceRecord {
	return redisFenceRecord{Scope: f.Scope, StoreID: f.StoreID, Producer: f.ProducerUser, Executor: f.ExecutorUser, Controller: f.ControllerUser}
}

func (f RedisFence) owns(record redisFenceRecord) bool {
	return record.Scope == f.Scope && record.StoreID == f.StoreID && record.Producer == f.ProducerUser && record.Executor == f.ExecutorUser && record.Controller == f.ControllerUser
}

// Check is read-only. It deliberately rejects shared instances rather than
// disabling a user that may belong to a cache, another deployment or an admin.
func (f RedisFence) Check(ctx context.Context) error {
	if f.Admin == nil || f.Repository == nil || f.Scope == "" || f.StoreID == "" {
		return errors.New("redis fence configuration is incomplete")
	}
	users := []string{f.ProducerUser, f.ExecutorUser, f.ControllerUser}
	for i, user := range users {
		if user == "" || strings.ContainsAny(user, " \r\n\t") || slices.Contains(users[:i], user) {
			return errors.New("redis fence requires three distinct ACL users")
		}
	}
	if f.Admin.Options().DB != 0 {
		return errors.New("redis fence requires a dedicated instance using database zero")
	}
	who, err := f.Admin.Do(ctx, "ACL", "WHOAMI").Text()
	if err != nil {
		return err
	}
	if who != f.ControllerUser {
		return errors.New("redis fence controller credentials do not match")
	}
	info, err := f.Admin.Info(ctx, "server", "replication", "cluster", "keyspace").Result()
	if err != nil {
		return err
	}
	fields := map[string]string{}
	for _, line := range strings.Split(info, "\r\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok {
			fields[key] = value
		}
		if strings.HasPrefix(key, "db") && key != "db0" && ok {
			return errors.New("redis fence cannot control an instance shared with another database")
		}
	}
	if fields["redis_mode"] != "standalone" || fields["role"] != "master" || fields["connected_slaves"] != "0" || fields["cluster_enabled"] != "0" {
		return errors.New("redis fence requires a standalone instance without replicas")
	}
	aclfile, err := f.Admin.ConfigGet(ctx, "aclfile").Result()
	if err != nil {
		return err
	}
	if aclfile["aclfile"] == "" {
		return errors.New("redis fence requires a persistent ACL file")
	}
	listed, err := f.Admin.Do(ctx, "ACL", "USERS").StringSlice()
	if err != nil {
		return err
	}
	if len(listed) != len(users) {
		return errors.New("redis fence found users outside the registered queue ownership")
	}
	for _, user := range users {
		if !slices.Contains(listed, user) {
			return errors.New("redis fence ACL user is missing")
		}
	}
	// DRYRUN is an authorization query, not an execution of these commands.
	// A source executor must be unable to restore its old producer's authority.
	for _, command := range [][]interface{}{
		{"ACL", "SETUSER", f.ProducerUser, "on"},
		{"CONFIG", "SET", "aclfile", "forbidden"},
		{"MODULE", "LOAD", "forbidden"},
	} {
		result, checkErr := f.Admin.ACLDryRun(ctx, f.ExecutorUser, command...).Result()
		if checkErr != nil {
			return checkErr
		}
		if result == "OK" {
			return errors.New("redis executor must not have administrative authority")
		}
		if !strings.Contains(result, "no permissions to run") {
			return errors.New("redis executor authorization could not be established")
		}
	}
	return nil
}

// Bind is explicit store provisioning; inventory never calls it.
func (f RedisFence) Bind(ctx context.Context) error {
	if err := f.Check(ctx); err != nil {
		return err
	}
	data, err := json.Marshal(f.identity())
	if err != nil {
		return err
	}
	if err := f.Admin.SetNX(ctx, redisFenceKey, data, 0).Err(); err != nil {
		return err
	}
	raw, err := f.Admin.Get(ctx, redisFenceKey).Bytes()
	if err != nil {
		return err
	}
	var record redisFenceRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return err
	}
	if !f.owns(record) {
		return ErrConflict
	}
	return nil
}

func (f RedisFence) Close(ctx context.Context, op Operation) error { return f.setClosed(ctx, op, true) }
func (f RedisFence) Open(ctx context.Context, op Operation) error  { return f.setClosed(ctx, op, false) }

func (f RedisFence) setClosed(ctx context.Context, op Operation, closed bool) error {
	if op.Scope != f.Scope || !op.IncludesStore(f.StoreID, "redis") {
		return ErrConflict
	}
	if err := f.Check(ctx); err != nil {
		return err
	}
	return f.Repository.transaction(ctx, f.Scope, func(tx *sqlx.Tx, _ int64, current string) error {
		if current != op.ID {
			return ErrStale
		}
		persisted, err := readOperation(ctx, tx, f.Scope, op.ID)
		if err != nil {
			return err
		}
		if persisted.Revision != op.Revision || persisted.Epoch != op.Epoch || persisted.Target != op.Target {
			return ErrStale
		}
		if closed && persisted.State != Fencing {
			return ErrConflict
		}
		if !closed && (persisted.State != Resuming || persisted.Purpose == DrainPrevious) {
			return ErrConflict
		}
		// WATCH/EXEC protects against a controller which lost its database
		// lock and wakes after a newer Redis command. Redis retains its own
		// monotonically increasing operation epoch/revision as well.
		err = f.Admin.Watch(ctx, func(rtx *redis.Tx) error {
			raw, readErr := rtx.Get(ctx, redisFenceKey).Bytes()
			if readErr != nil {
				return readErr
			}
			var record redisFenceRecord
			if readErr := json.Unmarshal(raw, &record); readErr != nil {
				return readErr
			}
			if !f.owns(record) {
				return ErrConflict
			}
			if record.Epoch > op.Epoch || (record.Epoch == op.Epoch && (record.Revision > op.Revision || (record.OperationID != "" && record.OperationID != op.ID))) {
				return ErrStale
			}
			record.OperationID, record.Epoch, record.Revision, record.Closed, record.Verified = op.ID, op.Epoch, op.Revision, closed, false
			data, marshalErr := json.Marshal(record)
			if marshalErr != nil {
				return marshalErr
			}
			_, execErr := rtx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				flag := "on"
				if closed {
					flag = "off"
				}
				pipe.ACLSetUser(ctx, f.ProducerUser, flag)
				// Disabling authentication does not revoke existing connections.
				if closed {
					pipe.Do(ctx, "CLIENT", "KILL", "USER", f.ProducerUser, "SKIPME", "YES")
				}
				pipe.Do(ctx, "ACL", "SAVE")
				pipe.Set(ctx, redisFenceKey, data, 0)
				return nil
			})
			return execErr
		}, redisFenceKey)
		if err != nil {
			return err
		}
		// EXEC can have partially failed commands. Publish verification only
		// after every command, including ACL persistence, returned success.
		return f.Admin.Watch(ctx, func(rtx *redis.Tx) error {
			raw, readErr := rtx.Get(ctx, redisFenceKey).Bytes()
			if readErr != nil {
				return readErr
			}
			var record redisFenceRecord
			if readErr := json.Unmarshal(raw, &record); readErr != nil {
				return readErr
			}
			if !f.owns(record) || record.OperationID != op.ID || record.Epoch != op.Epoch || record.Revision != op.Revision || record.Closed != closed {
				return ErrStale
			}
			record.Verified = true
			data, marshalErr := json.Marshal(record)
			if marshalErr != nil {
				return marshalErr
			}
			_, execErr := rtx.TxPipelined(ctx, func(pipe redis.Pipeliner) error { pipe.Set(ctx, redisFenceKey, data, 0); return nil })
			return execErr
		}, redisFenceKey)
	})
}

func (f RedisFence) Closed(ctx context.Context, op Operation) (bool, error) {
	if err := f.Check(ctx); err != nil {
		return false, err
	}
	raw, err := f.Admin.Get(ctx, redisFenceKey).Bytes()
	if err != nil {
		return false, err
	}
	var record redisFenceRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return false, err
	}
	if !f.owns(record) || record.OperationID != op.ID || record.Epoch != op.Epoch {
		return false, ErrStale
	}
	if !record.Closed || !record.Verified {
		return false, nil
	}
	list, err := f.Admin.ACLList(ctx).Result()
	if err != nil {
		return false, err
	}
	off := false
	for _, rule := range list {
		parts := strings.Fields(rule)
		if len(parts) > 2 && parts[1] == f.ProducerUser {
			off = slices.Contains(parts[2:], "off")
		}
	}
	if !off {
		return false, nil
	}
	// Verified records the successful CLIENT KILL in the closure transaction.
	// New unauthenticated connections may still be labelled with the default
	// username by Redis; they cannot authenticate while that user is off.
	return true, nil
}
