package dataplanestats

import (
	"encoding/json"
	"strings"
	"testing"
)

// The store keeps the snapshot as one JSON document and the handler renders the
// decoded struct, so JSON is the whole path from publisher to browser. A plane
// that measured no interval and a plane that moved nothing are different facts,
// and this is the encode that has to keep them apart.
//
// A plane with no previous reading to difference omits the throughput gauges. An
// omitted gauge list must not decode into a list of zeroes.
func TestUnmeasuredIntervalPublishesNoThroughputGauges(t *testing.T) {
	payload, err := json.Marshal(Snapshot{Replica: "agent-1", Running: true})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(payload), "throughput") {
		t.Fatalf("a snapshot with no measured interval must carry no throughput gauge, got %s", payload)
	}

	var decoded Snapshot
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.Gauges) != 0 {
		t.Fatalf("absent gauges decoded as present, which reads as a plane that moved nothing: %+v", decoded.Gauges)
	}
}

// A genuinely idle interval is reported, not omitted: all four gauges are there
// and three of them are zero. Metric.Value has no omitempty, which is what makes
// the zero survive, so this fails the moment it grows one.
func TestIdleIntervalIsPublishedAsZero(t *testing.T) {
	payload, err := json.Marshal(Snapshot{
		Replica: "agent-1",
		Running: true,
		Gauges: []Metric{
			{Name: GaugeThroughputWindowMS, Value: 4954},
			{Name: GaugeThroughputAccepted, Value: 0},
			{Name: GaugeThroughputDelivered, Value: 0},
			{Name: GaugeThroughputFailed, Value: 0},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if !strings.Contains(string(payload), `"value":0`) {
		t.Fatalf("an idle interval must send its zeroes rather than omit them, got %s", payload)
	}

	var decoded Snapshot
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	values := map[string]int64{}
	for _, gauge := range decoded.Gauges {
		values[gauge.Name] = gauge.Value
	}

	// The window is what turns the counts into a rate, so it has to survive
	// exactly: a sampler reports real elapsed milliseconds, not its period.
	if values[GaugeThroughputWindowMS] != 4954 {
		t.Fatalf("window length must survive the round trip exactly: %+v", decoded.Gauges)
	}
	if _, ok := values[GaugeThroughputAccepted]; !ok {
		t.Fatal("a measured zero must arrive as a present zero, not as an absent gauge")
	}
	if values[GaugeThroughputAccepted] != 0 || values[GaugeThroughputDelivered] != 0 {
		t.Fatalf("idle interval decoded as flow: %+v", decoded.Gauges)
	}
}

// A leftover row from a restarted replica is stale, and its name is often a
// container id that sorts ahead of the live replacement. The page has to open
// on the replica that is still reporting.
func TestOrderReplicasPutsStaleLast(t *testing.T) {
	replicas := []Replica{
		{Snapshot: Snapshot{Replica: "aaa"}, Stale: true},
		{Snapshot: Snapshot{Replica: "zzz"}, Stale: false},
		{Snapshot: Snapshot{Replica: "mmm"}, Stale: false},
	}

	OrderReplicas(replicas)

	got := make([]string, len(replicas))
	for i, replica := range replicas {
		got[i] = replica.Replica
	}
	want := []string{"mmm", "zzz", "aaa"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("live then stale, then by name: got %v want %v", got, want)
	}
}
