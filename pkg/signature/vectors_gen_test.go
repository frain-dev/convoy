package signature

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// vector is a single shared webhook-signature verification test case.
//
// testdata/signature-vectors.json is generated from this package's own signing
// code so every language SDK (convoy-go, convoy-python, convoy.js, convoy.rb,
// convoy-php, convoy-java) verifies against one canonical set. Regenerate with:
//
//	CONVOY_WRITE_VECTORS=1 go test ./pkg/signature -run TestGenerateSignatureVectors
//
// Time handling: valid advanced cases use a large tolerance so the age check
// always passes, isolating HMAC correctness from clock policy. Expiry cases keep
// the same old timestamp with a 300s tolerance so the age check fails.
type vector struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Advanced    bool   `json:"advanced"`
	Hash        string `json:"hash"`
	Encoding    string `json:"encoding"`
	Secret      string `json:"secret"`
	Payload     string `json:"payload"`
	Header      string `json:"header"`
	Tolerance   int64  `json:"tolerance"`
	Valid       bool   `json:"valid"`
}

const (
	vectorsPayload   = `{"event":"charge.success","amount":1000,"currency":"NGN"}`
	vectorsSecret    = "convoy-webhook-secret"
	vectorsWrongA    = "wrong-secret-a"
	vectorsWrongB    = "wrong-secret-b"
	vectorsTimestamp = "1700000000"
	toleranceValid   = int64(3153600000) // ~100 years
	toleranceExpiry  = int64(300)
)

var vectorsFile = filepath.Join("testdata", "signature-vectors.json")

func TestGenerateSignatureVectors(t *testing.T) {
	if os.Getenv("CONVOY_WRITE_VECTORS") == "" {
		t.Skip("set CONVOY_WRITE_VECTORS=1 to regenerate testdata/signature-vectors.json")
	}

	vectors := buildSignatureVectors(t)

	buf, err := marshalSignatureVectors(vectors)
	if err != nil {
		t.Fatalf("marshal vectors: %v", err)
	}

	if err = os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}

	if err = os.WriteFile(vectorsFile, buf, 0o644); err != nil {
		t.Fatalf("write %s: %v", vectorsFile, err)
	}

	t.Logf("wrote %d vectors to %s", len(vectors), vectorsFile)
}

// TestSignatureVectorsUpToDate is an always-on drift guard: it rebuilds the
// vectors from the current signing code and fails if the committed JSON is
// stale, so a change to signing (or the generator) cannot merge with vectors
// the SDKs would then verify against incorrectly.
func TestSignatureVectorsUpToDate(t *testing.T) {
	want, err := marshalSignatureVectors(buildSignatureVectors(t))
	if err != nil {
		t.Fatalf("marshal vectors: %v", err)
	}

	got, err := os.ReadFile(vectorsFile)
	if err != nil {
		t.Fatalf("read %s: %v (regenerate with CONVOY_WRITE_VECTORS=1 go test ./pkg/signature -run TestGenerateSignatureVectors)", vectorsFile, err)
	}

	if !bytes.Equal(want, got) {
		t.Fatalf("%s is stale; regenerate with CONVOY_WRITE_VECTORS=1 go test ./pkg/signature -run TestGenerateSignatureVectors", vectorsFile)
	}
}

func marshalSignatureVectors(vectors []vector) ([]byte, error) {
	buf, err := json.MarshalIndent(vectors, "", "  ")
	if err != nil {
		return nil, err
	}

	return append(buf, '\n'), nil
}

func buildSignatureVectors(t *testing.T) []vector {
	t.Helper()

	body := encodedBody(t, vectorsPayload)

	var out []vector

	// Matrix: {simple, advanced} x {SHA256, SHA512} x {hex, base64}.
	for _, advanced := range []bool{false, true} {
		for _, hash := range []string{"SHA256", "SHA512"} {
			for _, enc := range []string{"hex", "base64"} {
				mode := "simple"
				if advanced {
					mode = "advanced"
				}

				out = append(out, vector{
					Name:        fmt.Sprintf("%s_%s_%s_valid", mode, strings.ToLower(hash), enc),
					Description: fmt.Sprintf("%s mode, %s, %s: valid signature", mode, hash, enc),
					Advanced:    advanced,
					Hash:        hash,
					Encoding:    enc,
					Secret:      vectorsSecret,
					Payload:     body,
					Header:      computeHeader(t, advanced, hash, enc, vectorsSecret),
					Tolerance:   toleranceValid,
					Valid:       true,
				})
			}
		}
	}

	// Adversarial cases built on advanced SHA256 hex.
	sch := Scheme{Secret: []string{vectorsSecret}, Hash: "SHA256", Encoding: "hex"}
	signed := []byte(vectorsTimestamp + "," + body)
	rightSig := computeSig(t, sch, vectorsSecret, signed)
	wrongSigA := computeSig(t, sch, vectorsWrongA, signed)
	wrongSigB := computeSig(t, sch, vectorsWrongB, signed)

	edge := func(name, desc, header string, tolerance int64, valid bool) vector {
		return vector{
			Name:        name,
			Description: desc,
			Advanced:    true,
			Hash:        "SHA256",
			Encoding:    "hex",
			Secret:      vectorsSecret,
			Payload:     body,
			Header:      header,
			Tolerance:   tolerance,
			Valid:       valid,
		}
	}

	out = append(out,
		edge("advanced_tamper", "tampered signature must be rejected",
			"t="+vectorsTimestamp+",v1="+flipHex(rightSig), toleranceValid, false),
		edge("advanced_expired", "valid signature past tolerance must be rejected",
			"t="+vectorsTimestamp+",v1="+rightSig, toleranceExpiry, false),
		edge("advanced_t_not_first", "timestamp not first must still verify (key-based parse)",
			"v1="+rightSig+",t="+vectorsTimestamp, toleranceValid, true),
		edge("advanced_multi_v1_one_valid", "multiple v1, one correct, must verify",
			"t="+vectorsTimestamp+",v1="+wrongSigA+",v1="+rightSig, toleranceValid, true),
		edge("advanced_multi_v1_none_valid", "multiple v1, none correct, must be rejected",
			"t="+vectorsTimestamp+",v1="+wrongSigA+",v1="+wrongSigB, toleranceValid, false),
		edge("advanced_t_infinity", "non-finite timestamp must be rejected",
			"t=Infinity,v1="+rightSig, toleranceValid, false),
		edge("advanced_numeric_signature", "numeric signature value must not be parsed as timestamp",
			"t="+vectorsTimestamp+",v1=1234567890", toleranceValid, false),
		edge("advanced_missing_timestamp", "advanced header without t must be rejected",
			"v1="+rightSig+",v2="+rightSig, toleranceValid, false),
	)

	// Simple mode tamper.
	out = append(out, vector{
		Name:        "simple_tamper",
		Description: "simple mode tampered signature must be rejected",
		Advanced:    false,
		Hash:        "SHA256",
		Encoding:    "hex",
		Secret:      vectorsSecret,
		Payload:     body,
		Header:      flipHex(computeHeader(t, false, "SHA256", "hex", vectorsSecret)),
		Tolerance:   toleranceValid,
		Valid:       false,
	})

	return out
}

func encodedBody(t *testing.T, payload string) string {
	t.Helper()

	s := &Signature{Payload: json.RawMessage(payload)}
	buf, err := s.encodePayload()
	if err != nil {
		t.Fatalf("encode payload: %v", err)
	}

	return string(buf)
}

func computeHeader(t *testing.T, advanced bool, hash, enc, secret string) string {
	t.Helper()

	s := &Signature{
		Payload:             json.RawMessage(vectorsPayload),
		Advanced:            advanced,
		Schemes:             []Scheme{{Secret: []string{secret}, Hash: hash, Encoding: enc}},
		generateTimestampFn: func() string { return vectorsTimestamp },
	}

	h, err := s.ComputeHeaderValue()
	if err != nil {
		t.Fatalf("compute header: %v", err)
	}

	return h
}

func computeSig(t *testing.T, sch Scheme, secret string, buf []byte) string {
	t.Helper()

	s := &Signature{}
	sig, err := s.generateSignature(sch, secret, buf)
	if err != nil {
		t.Fatalf("generate signature: %v", err)
	}

	return sig
}

// flipHex flips the last nibble of a hex signature to produce a value that is
// still valid hex but no longer matches.
func flipHex(sig string) string {
	if sig == "" {
		return sig
	}

	repl := byte('0')
	if sig[len(sig)-1] == '0' {
		repl = '1'
	}

	return sig[:len(sig)-1] + string(repl)
}
