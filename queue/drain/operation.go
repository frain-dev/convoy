// Package drain implements the durable lifecycle of administrative queue
// operations. An inventory snapshot alone can never complete an operation.
package drain

import (
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/queue"
)

type Purpose string

const (
	Quiesce       Purpose = "quiesce"
	Drain         Purpose = "drain"
	DrainPrevious Purpose = "drain_previous"
)

type State string

const (
	Fencing   State = "fencing"
	Quiescing State = "quiescing"
	Draining  State = "draining"
	Settling  State = "settling"
	Verifying State = "verifying"
	Quiesced  State = "quiesced"
	Drained   State = "drained"
	Stopping  State = "stopping"
	Stopped   State = "stopped"
	Resuming  State = "resuming"
	Resumed   State = "resumed"
)

var (
	ErrConflict = errors.New("a conflicting queue operation exists")
	ErrStale    = errors.New("queue operation revision changed; refresh before retrying")
	ErrNotFound = errors.New("queue operation not found")
	ErrFenced   = queue.ErrAdmissionClosed
	ErrEvidence = errors.New("queue operation evidence is incomplete")
)

// Target is immutable for the lifetime of an operation. Configuration changes
// require reconciliation, not silently running the old operation on a new store.
type Target struct {
	ResumePaused          bool    `json:"resume_paused"`
	PausedQueues          string  `json:"paused_queue_manifest,omitempty"`
	OtherStoreID          string  `json:"other_store_id,omitempty"`
	Scope                 string  `json:"scope"`
	StoreID               string  `json:"store_id"`
	Provider              string  `json:"provider"`
	ConfigurationRevision string  `json:"configuration_revision"`
	Purpose               Purpose `json:"purpose"`
}

func (t Target) validate() error {
	if t.Scope == "" || t.StoreID == "" || t.ConfigurationRevision == "" {
		return errors.New("scope, store identity and configuration revision are required")
	}
	if len(t.Scope) > 200 || len(t.StoreID) > 200 || len(t.ConfigurationRevision) > 200 {
		return errors.New("queue operation identifiers exceed 200 characters")
	}
	if t.Provider != "redis" && t.Provider != "postgres" {
		return errors.New("unsupported queue provider")
	}
	if t.Purpose != Quiesce && t.Purpose != Drain && t.Purpose != DrainPrevious {
		return errors.New("unsupported queue operation purpose")
	}
	return nil
}

type Operation struct {
	ID string `json:"id"`
	Target
	Epoch     int64     `json:"epoch"`
	Revision  int64     `json:"revision"`
	State     State     `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Evidence is supplied by a runtime coordinator, never by the administrative
// HTTP client. A coordinator must hold the final execution barrier while it
// obtains this evidence and commits the transition. Expired heartbeats do not
// prove that a writer has lost authority.
type Evidence struct {
	Epoch                 int64
	ConfigurationRevision string
	AdmissionClosed       bool
	WritersFenced         bool
	ClaimsStopped         bool
	Unsettled             int64
	Pending               int64
	Processing            int64
	Future                int64
	Aggregating           int64
	Archived              int64
	Unknown               int64
	InspectionSucceeded   bool
	RuntimeReady          bool
	AdmissionOpen         bool
}

func (e Evidence) barrier(o Operation) error {
	if e.Epoch != o.Epoch || e.ConfigurationRevision != o.ConfigurationRevision {
		return ErrStale
	}
	if !e.AdmissionClosed || !e.WritersFenced || !e.InspectionSucceeded {
		return ErrEvidence
	}
	for _, n := range []int64{e.Unsettled, e.Pending, e.Processing, e.Future, e.Aggregating, e.Archived, e.Unknown} {
		if n < 0 {
			return ErrEvidence
		}
	}
	if e.Unsettled != 0 || e.Unknown != 0 {
		return ErrEvidence
	}
	return nil
}

func validateTransition(o Operation, next State, e Evidence) error {
	switch next {
	case Quiescing, Draining:
		if o.State != Fencing {
			return ErrConflict
		}
		if (next == Quiescing) != (o.Purpose == Quiesce) {
			return ErrConflict
		}
		return e.barrier(o)
	case Settling:
		if o.State != Draining {
			return ErrConflict
		}
		if err := e.barrier(o); err != nil {
			return err
		}
		if e.Pending != 0 || e.Processing != 0 || e.Future != 0 || e.Aggregating != 0 || e.Archived != 0 {
			return ErrEvidence
		}
	case Verifying:
		if o.State != Quiescing && o.State != Draining && o.State != Settling {
			return ErrConflict
		}
		if err := e.barrier(o); err != nil {
			return err
		}
		if !e.ClaimsStopped || e.Processing != 0 {
			return ErrEvidence
		}
		if o.Purpose != Quiesce && (e.Pending != 0 || e.Future != 0 || e.Aggregating != 0 || e.Archived != 0) {
			return ErrEvidence
		}
	case Quiesced, Drained:
		if o.State != Verifying {
			return ErrConflict
		}
		if (next == Quiesced) != (o.Purpose == Quiesce) {
			return ErrConflict
		}
		// Reconcile again under the barrier; entering verification was not a
		// completion certificate and must not authorize a later stale sample.
		previous := o
		if o.Purpose == Quiesce {
			previous.State = Quiescing
		} else {
			previous.State = Draining
		}
		return validateTransition(previous, Verifying, e)
	case Stopping:
		if o.Purpose != DrainPrevious || (o.State != Fencing && o.State != Draining && o.State != Settling && o.State != Verifying) {
			return ErrConflict
		}
	case Stopped:
		if o.State != Stopping {
			return ErrConflict
		}
		if err := e.barrier(o); err != nil {
			return err
		}
		if !e.ClaimsStopped || e.Processing != 0 {
			return ErrEvidence
		}
	case Fencing:
		if o.Purpose != DrainPrevious || o.State != Stopped {
			return ErrConflict
		}
	case Resuming:
		// Source-only Stop/Resume never grants the retired store permission to
		// accept external traffic. A completed source keeps its fence too.
		if o.Purpose == DrainPrevious || o.State == Resumed || o.State == Resuming {
			return ErrConflict
		}
	case Resumed:
		if o.Purpose == DrainPrevious || o.State != Resuming {
			return ErrConflict
		}
		if e.Epoch != o.Epoch || e.ConfigurationRevision != o.ConfigurationRevision {
			return ErrStale
		}
		if !e.RuntimeReady || !e.AdmissionOpen {
			return ErrEvidence
		}
	default:
		return fmt.Errorf("unsupported queue operation state %q", next)
	}
	return nil
}

func (o Operation) IncludesStore(id, provider string) bool {
	return (o.StoreID == id && o.Provider == provider) || (o.Purpose != DrainPrevious && o.OtherStoreID != "" && o.OtherStoreID == id)
}
