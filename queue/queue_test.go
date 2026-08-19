package queue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
)

func TestCronJobID(t *testing.T) {
	tick := time.Date(2026, 8, 15, 2, 15, 0, 0, time.UTC)
	lagos := time.FixedZone("WAT", 1*60*60)

	tests := []struct {
		name  string
		task  convoy.TaskName
		at    time.Time
		other convoy.TaskName
		when  time.Time
		same  bool
	}{
		{
			name: "replicas firing the same tick agree",
			task: convoy.SnapshotUsage, at: tick,
			other: convoy.SnapshotUsage, when: tick,
			same: true,
		},
		{
			name: "a firing later in the same minute is the same tick",
			task: convoy.SnapshotUsage, at: tick,
			other: convoy.SnapshotUsage, when: tick.Add(59 * time.Second),
			same: true,
		},
		{
			name: "the bucket does not depend on the caller's location",
			task: convoy.SnapshotUsage, at: tick,
			other: convoy.SnapshotUsage, when: tick.In(lagos),
			same: true,
		},
		{
			name: "the next minute is a new tick",
			task: convoy.SnapshotUsage, at: tick,
			other: convoy.SnapshotUsage, when: tick.Add(time.Minute),
			same: false,
		},
		{
			name: "a firing delayed past its minute is a new tick",
			task: convoy.SnapshotUsage, at: tick,
			other: convoy.SnapshotUsage, when: tick.Add(61 * time.Second),
			same: false,
		},
		{
			name: "different tasks never share a tick",
			task: convoy.SnapshotUsage, at: tick,
			other: convoy.RetentionPolicies, when: tick,
			same: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CronJobID(tc.task, tc.at)
			require.Equal(t, tc.same, got == CronJobID(tc.other, tc.when))
		})
	}
}

func TestCronJobIDCarriesTheSharedPrefix(t *testing.T) {
	id := CronJobID(convoy.SnapshotUsage, time.Now())
	require.Equal(t, CronJobIDPrefix, id[:len(CronJobIDPrefix)])
	require.Contains(t, id, string(convoy.SnapshotUsage))
}

func TestPriorityCyclePreservesWeights(t *testing.T) {
	cycle := PriorityCycle(map[string]int{
		"critical": 5,
		"default":  2,
		"low":      1,
	})

	require.Len(t, cycle, 8)
	require.Equal(t, 5, countQueue(cycle, "critical"))
	require.Equal(t, 2, countQueue(cycle, "default"))
	require.Equal(t, 1, countQueue(cycle, "low"))
}

func countQueue(names []string, target string) int {
	count := 0
	for _, name := range names {
		if name == target {
			count++
		}
	}
	return count
}
