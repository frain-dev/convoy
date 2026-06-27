package handlers

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testSigningKey(b byte) []byte {
	k := make([]byte, signingKeyLen)
	for i := range k {
		k[i] = b
	}
	return k
}

// A cookie signed with the cluster key validates with that same key. This is
// the multi-replica case: every pod loads the same Redis-stored key, so a
// cookie minted on one pod verifies on another. The previous per-process random
// key failed this.
func TestQueueSessionCookie_SignAndVerifyRoundtrip(t *testing.T) {
	key := testSigningKey(0x11)
	value := signWithKey(key, time.Now().Add(queueMonitoringCookieTTL))

	_, ok := verifyWithKey(key, value)
	require.True(t, ok)
}

func TestQueueSessionCookie_RejectsDifferentKey(t *testing.T) {
	value := signWithKey(testSigningKey(0x11), time.Now().Add(queueMonitoringCookieTTL))

	_, ok := verifyWithKey(testSigningKey(0x22), value)
	require.False(t, ok)
}

func TestQueueSessionCookie_RejectsTamperedSignature(t *testing.T) {
	key := testSigningKey(0x11)
	value := signWithKey(key, time.Now().Add(queueMonitoringCookieTTL))

	_, ok := verifyWithKey(key, value+"00")
	require.False(t, ok)
}

func TestQueueSessionCookie_RejectsExpired(t *testing.T) {
	key := testSigningKey(0x11)
	value := signWithKey(key, time.Now().Add(-time.Minute))

	_, ok := verifyWithKey(key, value)
	require.False(t, ok)
}

func TestQueueSessionCookie_RejectsMalformed(t *testing.T) {
	_, ok := verifyWithKey(testSigningKey(0x11), "no-dot-here")
	require.False(t, ok)
}

func TestDecodeSigningKey(t *testing.T) {
	k, ok := decodeSigningKey(hex.EncodeToString(testSigningKey(0x11)))
	require.True(t, ok)
	require.Len(t, k, signingKeyLen)

	_, ok = decodeSigningKey("zzzz")
	require.False(t, ok)

	_, ok = decodeSigningKey(hex.EncodeToString([]byte{0x01, 0x02}))
	require.False(t, ok)
}
