package testenv

import (
	"context"
	"fmt"
	"testing"
)

func TestSentinelContainer(t *testing.T) {
	ctx := context.Background()

	container, factory, err := NewTestRedisSentinel(ctx)
	if err != nil {
		t.Fatalf("failed to create sentinel container: %v", err)
	}
	defer container.Terminate(ctx)

	client, sentinelAddr, err := factory(t, 0)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	fmt.Printf("Successfully created failover client pointing to %s\n", sentinelAddr)

	err = client.Ping(ctx).Err()
	if err != nil {
		t.Fatalf("failed to ping master through sentinel: %v", err)
	}

	fmt.Println("Ping successful!")
}
