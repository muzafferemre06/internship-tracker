package main

import (
	"bytes"
	"crypto/ecdh"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCreatesMatchingOwnerOnlyKeyPairWithoutOverwrite(t *testing.T) {
	privatePath := filepath.Join(t.TempDir(), "web_push_private_key")
	var output bytes.Buffer
	if err := run([]string{"-private-output", privatePath}, &output); err != nil {
		t.Fatalf("generate VAPID key pair: %v", err)
	}

	info, err := os.Stat(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected private key mode %o", info.Mode().Perm())
	}
	privateEncoded, err := os.ReadFile(privatePath)
	if err != nil {
		t.Fatal(err)
	}
	privateBytes, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(string(privateEncoded)))
	if err != nil {
		t.Fatalf("decode private key: %v", err)
	}
	privateKey, err := ecdh.P256().NewPrivateKey(privateBytes)
	if err != nil {
		t.Fatalf("parse private key: %v", err)
	}

	publicEncoded := strings.TrimSpace(strings.TrimPrefix(output.String(), "WEB_PUSH_PUBLIC_KEY="))
	publicBytes, err := base64.RawURLEncoding.DecodeString(publicEncoded)
	if err != nil {
		t.Fatalf("decode public key: %v", err)
	}
	if !bytes.Equal(publicBytes, privateKey.PublicKey().Bytes()) {
		t.Fatal("public key does not match private key")
	}

	if err := run([]string{"-private-output", privatePath}, &bytes.Buffer{}); err == nil {
		t.Fatal("existing private key file must not be overwritten")
	}
}

func TestRunRequiresPrivateOutput(t *testing.T) {
	if err := run(nil, &bytes.Buffer{}); err == nil {
		t.Fatal("missing private output path must fail")
	}
}
