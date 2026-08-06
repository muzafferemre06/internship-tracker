package push

import (
	"bytes"
	"crypto/ecdh"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

type Config struct {
	Enabled    bool
	PublicKey  string
	PrivateKey string
	Subject    string
}

func ValidateConfig(config Config) (Config, error) {
	config.PublicKey = strings.TrimSpace(config.PublicKey)
	config.PrivateKey = strings.TrimSpace(config.PrivateKey)
	config.Subject = strings.TrimSpace(config.Subject)
	if !config.Enabled {
		if config.PublicKey != "" || config.PrivateKey != "" || config.Subject != "" {
			return Config{}, errors.New("WEB_PUSH_ENABLED must be true when Web Push values are configured")
		}
		return config, nil
	}
	if config.PublicKey == "" || config.PrivateKey == "" || config.Subject == "" {
		return Config{}, errors.New("enabled Web Push requires public key, private key and subject")
	}
	publicBytes, err := decodeBase64URL(config.PublicKey)
	if err != nil || len(publicBytes) != 65 || publicBytes[0] != 4 {
		return Config{}, errors.New("WEB_PUSH_PUBLIC_KEY must be an uncompressed P-256 public key")
	}
	privateBytes, err := decodeBase64URL(config.PrivateKey)
	if err != nil || len(privateBytes) != 32 {
		return Config{}, errors.New("WEB_PUSH_PRIVATE_KEY must be a 32-byte P-256 private key")
	}
	privateKey, err := ecdh.P256().NewPrivateKey(privateBytes)
	if err != nil {
		return Config{}, errors.New("WEB_PUSH_PRIVATE_KEY is not a valid P-256 private key")
	}
	if !bytes.Equal(privateKey.PublicKey().Bytes(), publicBytes) {
		return Config{}, errors.New("Web Push public and private keys do not match")
	}
	subjectURL, err := url.Parse(config.Subject)
	if err != nil || (subjectURL.Scheme != "mailto" && subjectURL.Scheme != "https") {
		return Config{}, errors.New("WEB_PUSH_SUBJECT must be a mailto: or https: URI")
	}
	if subjectURL.Scheme == "mailto" && strings.TrimSpace(subjectURL.Opaque) == "" {
		return Config{}, errors.New("WEB_PUSH_SUBJECT mailto URI must include an address")
	}
	if subjectURL.Scheme == "https" && subjectURL.Hostname() == "" {
		return Config{}, errors.New("WEB_PUSH_SUBJECT https URI must include a host")
	}
	return config, nil
}

func ValidateSubscription(endpoint, p256dh, auth string) error {
	if err := ValidateEndpoint(endpoint); err != nil {
		return err
	}
	publicKey, err := decodeBase64URL(strings.TrimSpace(p256dh))
	if err != nil || len(publicKey) != 65 || publicKey[0] != 4 {
		return errors.New("push p256dh key is invalid")
	}
	if _, err := ecdh.P256().NewPublicKey(publicKey); err != nil {
		return errors.New("push p256dh key is invalid")
	}
	authSecret, err := decodeBase64URL(strings.TrimSpace(auth))
	if err != nil || len(authSecret) != 16 {
		return errors.New("push auth secret is invalid")
	}
	return nil
}

func ValidateEndpoint(endpoint string) error {
	endpoint = strings.TrimSpace(endpoint)
	if len(endpoint) == 0 || len(endpoint) > 4096 {
		return errors.New("push endpoint length is invalid")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return errors.New("push endpoint must be an HTTPS URL without credentials or fragment")
	}
	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return errors.New("push endpoint cannot target localhost")
	}
	if address := net.ParseIP(hostname); address != nil && (address.IsLoopback() || address.IsPrivate() || address.IsLinkLocalUnicast() || address.IsUnspecified()) {
		return errors.New("push endpoint cannot target a private address")
	}
	return nil
}

func decodeBase64URL(value string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(value, "="))
	if err != nil {
		return nil, fmt.Errorf("decode base64url: %w", err)
	}
	return decoded, nil
}
