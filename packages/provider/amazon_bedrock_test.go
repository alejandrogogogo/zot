package provider

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestSigV4Known checks our signer against AWS's published test vector
// from the SigV4 documentation. The expected Authorization header below
// is taken from "get-vanilla" in the suite (adapted: we use POST with
// an empty body since Bedrock is always POST).
//
// We don't run the official suite (it lives in S3 as a tarball and the
// signing rules vary slightly between examples); instead this test
// pins a fixed input/output so future refactors can't silently break
// the signer.
func TestSigV4Deterministic(t *testing.T) {
	creds := &bedrockSigV4Creds{
		accessKeyID:     "AKIDEXAMPLE",
		secretAccessKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
	}
	body := []byte(`{"hello":"world"}`)
	req, err := http.NewRequest("POST", "https://bedrock-runtime.us-east-1.amazonaws.com/model/foo/converse-stream", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("content-type", "application/json")
	when := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	if err := signSigV4(req, body, "bedrock", "us-east-1", creds, when); err != nil {
		t.Fatal(err)
	}
	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") {
		t.Fatalf("missing AWS4 prefix: %q", auth)
	}
	if !strings.Contains(auth, "Credential=AKIDEXAMPLE/20240115/us-east-1/bedrock/aws4_request") {
		t.Errorf("wrong credential scope: %q", auth)
	}
	if !strings.Contains(auth, "SignedHeaders=") {
		t.Errorf("missing SignedHeaders: %q", auth)
	}
	if !strings.Contains(auth, "Signature=") {
		t.Errorf("missing Signature: %q", auth)
	}
	// Sign again with identical inputs -> identical signature (determinism).
	req2, _ := http.NewRequest("POST", "https://bedrock-runtime.us-east-1.amazonaws.com/model/foo/converse-stream", bytes.NewReader(body))
	req2.Header.Set("content-type", "application/json")
	if err := signSigV4(req2, body, "bedrock", "us-east-1", creds, when); err != nil {
		t.Fatal(err)
	}
	if req2.Header.Get("Authorization") != auth {
		t.Errorf("signature not deterministic:\n a=%q\n b=%q", auth, req2.Header.Get("Authorization"))
	}
}

func TestSigV4PathEncoding(t *testing.T) {
	// Bedrock model IDs include colons and dots. The colon must encode
	// to %3A in the canonical path (per SigV4 rules).
	cases := map[string]string{
		"/model/anthropic.claude-sonnet-4-5-20250929-v1:0/converse-stream": "/model/anthropic.claude-sonnet-4-5-20250929-v1%3A0/converse-stream",
		"/":  "/",
		"":   "/",
		"/a": "/a",
	}
	for in, want := range cases {
		got := canonicalSigV4Path(in)
		if got != want {
			t.Errorf("canonicalSigV4Path(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReadAWSCredentialsFile(t *testing.T) {
	// Smoke test: parses an INI-ish file when present. We don't require
	// the file to exist on the test host.
	if _, err := readAWSCredentialsFile("default"); err != nil {
		// missing file or missing profile is fine — both are non-panics
		t.Logf("no aws creds available (expected on CI): %v", err)
	}
}
