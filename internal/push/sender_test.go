package push

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/store"
)

func TestEncryptPayloadMatchesRFC8291Vector(t *testing.T) {
	decode := func(value string) []byte {
		result, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil {
			t.Fatalf("decode test vector: %v", err)
		}
		return result
	}
	subscription := store.PushSubscription{
		P256DH: "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4",
		Auth:   "BTBZMqHH6r4Tts7J_aSIgg",
	}
	result, err := encryptPayloadWithKey(
		subscription,
		[]byte("When I grow up, I want to be a watermelon"),
		decode("yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"),
		decode("DGv6ra1nlYgDCS1FRnbzlw"),
	)
	if err != nil {
		t.Fatalf("encrypt RFC vector: %v", err)
	}
	want := "DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27ml" +
		"mlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPT" +
		"pK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN"
	if got := base64.RawURLEncoding.EncodeToString(result); got != want {
		t.Fatalf("RFC 8291 ciphertext mismatch\nwant %s\n got %s", want, got)
	}
}

func TestVAPIDAuthorizationIsVerifiableES256JWT(t *testing.T) {
	privateBytes := make([]byte, 32)
	privateBytes[31] = 7
	privateKey, err := ecdh.P256().NewPrivateKey(privateBytes)
	if err != nil {
		t.Fatal(err)
	}
	config, err := ValidateConfig(Config{
		Enabled: true, PublicKey: encodeRawURL(privateKey.PublicKey().Bytes()),
		PrivateKey: encodeRawURL(privateBytes), Subject: "mailto:push@example.test",
	})
	if err != nil {
		t.Fatalf("validate test VAPID config: %v", err)
	}
	sender := &HTTPSender{config: config, now: func() time.Time {
		return time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	}, random: strings.NewReader(strings.Repeat("z", 256))}
	header, err := sender.authorization("https://push.example.test/messages/one")
	if err != nil {
		t.Fatalf("build VAPID authorization: %v", err)
	}
	if !strings.HasPrefix(header, "vapid t=") || !strings.Contains(header, ", k="+config.PublicKey) {
		t.Fatalf("unexpected VAPID authorization: %s", header)
	}
	token := strings.Split(strings.TrimPrefix(header, "vapid t="), ", k=")[0]
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected JWT: %s", token)
	}
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims struct {
		Audience string `json:"aud"`
		Expires  int64  `json:"exp"`
		Subject  string `json:"sub"`
	}
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Audience != "https://push.example.test" || claims.Subject != config.Subject || claims.Expires != sender.now().Add(12*time.Hour).Unix() {
		t.Fatalf("unexpected VAPID claims: %#v", claims)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != 64 {
		t.Fatalf("invalid raw ES256 signature: len=%d err=%v", len(signature), err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	publicBytes := privateKey.PublicKey().Bytes()
	x, y := elliptic.Unmarshal(elliptic.P256(), publicBytes)
	publicKey := ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
	if !ecdsa.Verify(&publicKey, digest[:], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:])) {
		t.Fatal("VAPID JWT signature could not be verified")
	}
}

func TestValidateConfigAndSubscriptionRejectUnsafeValues(t *testing.T) {
	if _, err := ValidateConfig(Config{Enabled: true}); err == nil {
		t.Fatal("expected incomplete enabled config to fail")
	}
	if err := ValidateEndpoint("http://push.example.test/message"); err == nil {
		t.Fatal("expected non-HTTPS endpoint to fail")
	}
	if err := ValidateEndpoint("https://127.0.0.1/message"); err == nil {
		t.Fatal("expected private endpoint to fail")
	}
}
