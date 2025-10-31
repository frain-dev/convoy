package queue

import (
	"fmt"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
)

// Queue priority constants
const (
	QueuePriorityVeryHigh    = 7
	QueuePriorityHigh        = 5
	QueuePriorityAboveMedium = 4
	QueuePriorityMedium      = 3
	QueuePriorityNormal      = 2
	QueuePriorityLow         = 1
)

// QueuePriorities is a type alias for queue priority mappings
type QueuePriorities map[string]int

var (
	// DefaultPriorities for server and general use
	DefaultPriorities = QueuePriorities{
		string(convoy.EventQueue):         QueuePriorityHigh,
		string(convoy.CreateEventQueue):   QueuePriorityNormal,
		string(convoy.EventWorkflowQueue): QueuePriorityMedium,
		string(convoy.ScheduleQueue):      QueuePriorityLow,
		string(convoy.DefaultQueue):       QueuePriorityLow,
		string(convoy.MetaEventQueue):     QueuePriorityLow,
	}

	// EventsPriorities for EventsExecutionMode - focuses on event processing
	EventsPriorities = QueuePriorities{
		string(convoy.EventQueue):         QueuePriorityHigh,
		string(convoy.CreateEventQueue):   QueuePriorityHigh,
		string(convoy.EventWorkflowQueue): QueuePriorityHigh,
	}

	// RetryPriorities for RetryExecutionMode - focuses on retry processing
	RetryPriorities = QueuePriorities{
		string(convoy.RetryEventQueue):    QueuePriorityVeryHigh,
		string(convoy.ScheduleQueue):      QueuePriorityLow,
		string(convoy.DefaultQueue):       QueuePriorityLow,
		string(convoy.MetaEventQueue):     QueuePriorityLow,
		string(convoy.BatchRetryQueue):    QueuePriorityHigh,
		string(convoy.EventWorkflowQueue): QueuePriorityAboveMedium,
	}

	// BothPriorities for DefaultExecutionMode - processes both events and retries
	BothPriorities = QueuePriorities{
		string(convoy.EventQueue):         QueuePriorityAboveMedium,
		string(convoy.CreateEventQueue):   QueuePriorityAboveMedium,
		string(convoy.EventWorkflowQueue): QueuePriorityMedium,
		string(convoy.RetryEventQueue):    QueuePriorityLow,
		string(convoy.ScheduleQueue):      QueuePriorityLow,
		string(convoy.DefaultQueue):       QueuePriorityLow,
		string(convoy.MetaEventQueue):     QueuePriorityLow,
		string(convoy.BatchRetryQueue):    QueuePriorityLow,
	}
)

func (qp QueuePriorities) ToMap() map[string]int {
	return map[string]int(qp)
}

func GetQueuePriorities(commandName string, executionMode config.ExecutionMode) (map[string]int, error) {
	isWorkerCommand := commandName == "worker" || commandName == "agent"
	if isWorkerCommand {
		if executionMode == "" {
			executionMode = config.DefaultExecutionMode
		}

		switch executionMode {
		case config.RetryExecutionMode:
			return RetryPriorities.ToMap(), nil
		case config.EventsExecutionMode:
			return EventsPriorities.ToMap(), nil
		case config.DefaultExecutionMode:
			return BothPriorities.ToMap(), nil
		default:
			return nil, fmt.Errorf("unknown execution mode: %s", executionMode)
		}
	}

	return DefaultPriorities.ToMap(), nil
}
