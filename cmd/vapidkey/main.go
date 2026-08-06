package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "vapid key generation failed:", err)
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("vapidkey", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	privateOutput := flags.String("private-output", "", "owner-only output file for the VAPID private key")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*privateOutput) == "" {
		return errors.New("usage: vapidkey -private-output PATH")
	}

	privateKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate P-256 key: %w", err)
	}
	privateValue := base64.RawURLEncoding.EncodeToString(privateKey.Bytes()) + "\n"
	privateFile, err := os.OpenFile(*privateOutput, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create private key file: %w", err)
	}
	if _, err := io.WriteString(privateFile, privateValue); err != nil {
		_ = privateFile.Close()
		_ = os.Remove(*privateOutput)
		return fmt.Errorf("write private key file: %w", err)
	}
	if err := privateFile.Close(); err != nil {
		_ = os.Remove(*privateOutput)
		return fmt.Errorf("close private key file: %w", err)
	}

	publicValue := base64.RawURLEncoding.EncodeToString(privateKey.PublicKey().Bytes())
	if _, err := fmt.Fprintf(output, "WEB_PUSH_PUBLIC_KEY=%s\n", publicValue); err != nil {
		return fmt.Errorf("write public key: %w", err)
	}
	return nil
}
