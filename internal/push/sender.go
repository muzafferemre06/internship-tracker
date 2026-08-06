package push

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"crypto/hkdf"

	"github.com/muzaffer/internship-tracker/internal/store"
)

type Message struct {
	Payload []byte
	Topic   string
	TTL     int
}

type SendResult struct {
	StatusCode int
	RetryAfter *time.Time
}

type Sender interface {
	Send(ctx context.Context, subscription store.PushSubscription, message Message) (SendResult, error)
}

type HTTPSender struct {
	config Config
	client *http.Client
	now    func() time.Time
	random io.Reader
}

func NewHTTPSender(config Config, client *http.Client) (*HTTPSender, error) {
	validated, err := ValidateConfig(config)
	if err != nil {
		return nil, err
	}
	if !validated.Enabled {
		return nil, errors.New("Web Push sender is disabled")
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &HTTPSender{config: validated, client: client, now: time.Now, random: rand.Reader}, nil
}

func (s *HTTPSender) Send(
	ctx context.Context,
	subscription store.PushSubscription,
	message Message,
) (SendResult, error) {
	if err := ValidateSubscription(subscription.Endpoint, subscription.P256DH, subscription.Auth); err != nil {
		return SendResult{}, err
	}
	if len(message.Payload) == 0 || len(message.Payload) > 3000 {
		return SendResult{}, errors.New("push payload must be between 1 and 3000 bytes")
	}
	if len(message.Topic) == 0 || len(message.Topic) > 32 {
		return SendResult{}, errors.New("push topic must be between 1 and 32 characters")
	}
	body, err := encryptPayload(subscription, message.Payload, s.random)
	if err != nil {
		return SendResult{}, err
	}
	authorization, err := s.authorization(subscription.Endpoint)
	if err != nil {
		return SendResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, subscription.Endpoint, bytes.NewReader(body))
	if err != nil {
		return SendResult{}, fmt.Errorf("create Web Push request: %w", err)
	}
	ttl := message.TTL
	if ttl <= 0 {
		ttl = 86400
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Encoding", "aes128gcm")
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("TTL", strconv.Itoa(ttl))
	request.Header.Set("Urgency", "high")
	request.Header.Set("Topic", message.Topic)

	response, err := s.client.Do(request)
	if err != nil {
		return SendResult{}, fmt.Errorf("send Web Push request: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	result := SendResult{StatusCode: response.StatusCode}
	result.RetryAfter = parseRetryAfter(response.Header.Get("Retry-After"), s.now().UTC())
	return result, nil
}

func (s *HTTPSender) authorization(endpoint string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse push endpoint audience: %w", err)
	}
	header, _ := json.Marshal(map[string]string{"typ": "JWT", "alg": "ES256"})
	claims, _ := json.Marshal(map[string]any{
		"aud": parsed.Scheme + "://" + parsed.Host,
		"exp": s.now().UTC().Add(12 * time.Hour).Unix(),
		"sub": s.config.Subject,
	})
	unsigned := encodeRawURL(header) + "." + encodeRawURL(claims)
	privateBytes, _ := decodeBase64URL(s.config.PrivateKey)
	x, y := elliptic.P256().ScalarBaseMult(privateBytes)
	privateKey := &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, D: new(big.Int).SetBytes(privateBytes)}
	digest := sha256.Sum256([]byte(unsigned))
	r, signatureS, err := ecdsa.Sign(s.random, privateKey, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign VAPID token: %w", err)
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	signatureS.FillBytes(signature[32:])
	token := unsigned + "." + encodeRawURL(signature)
	return "vapid t=" + token + ", k=" + s.config.PublicKey, nil
}

func encryptPayload(subscription store.PushSubscription, plaintext []byte, random io.Reader) ([]byte, error) {
	sender, err := ecdh.P256().GenerateKey(random)
	if err != nil {
		return nil, fmt.Errorf("generate push message key: %w", err)
	}
	salt := make([]byte, 16)
	if _, err := io.ReadFull(random, salt); err != nil {
		return nil, fmt.Errorf("generate push salt: %w", err)
	}
	return encryptPayloadWithKey(subscription, plaintext, sender.Bytes(), salt)
}

func encryptPayloadWithKey(
	subscription store.PushSubscription,
	plaintext []byte,
	senderPrivate []byte,
	salt []byte,
) ([]byte, error) {
	receiverBytes, err := decodeBase64URL(subscription.P256DH)
	if err != nil {
		return nil, fmt.Errorf("decode receiver push key: %w", err)
	}
	authSecret, err := decodeBase64URL(subscription.Auth)
	if err != nil {
		return nil, fmt.Errorf("decode push auth secret: %w", err)
	}
	receiver, err := ecdh.P256().NewPublicKey(receiverBytes)
	if err != nil {
		return nil, fmt.Errorf("parse receiver push key: %w", err)
	}
	sender, err := ecdh.P256().NewPrivateKey(senderPrivate)
	if err != nil {
		return nil, fmt.Errorf("parse sender push key: %w", err)
	}
	sharedSecret, err := sender.ECDH(receiver)
	if err != nil {
		return nil, fmt.Errorf("derive push message secret: %w", err)
	}
	senderPublic := sender.PublicKey().Bytes()
	info := append([]byte("WebPush: info\x00"), receiverBytes...)
	info = append(info, senderPublic...)
	ikm, err := hkdf.Key(sha256.New, sharedSecret, authSecret, string(info), 32)
	if err != nil {
		return nil, fmt.Errorf("derive push input key: %w", err)
	}
	if len(salt) != 16 {
		return nil, errors.New("push salt must be 16 bytes")
	}
	contentKey, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, fmt.Errorf("derive push content key: %w", err)
	}
	nonce, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, fmt.Errorf("derive push nonce: %w", err)
	}
	block, err := aes.NewCipher(contentKey)
	if err != nil {
		return nil, fmt.Errorf("create push cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create push AEAD: %w", err)
	}
	record := append(append([]byte(nil), plaintext...), 2)
	ciphertext := aead.Seal(nil, nonce, record, nil)
	const recordSize = 4096
	result := make([]byte, 0, 16+4+1+len(senderPublic)+len(ciphertext))
	result = append(result, salt...)
	size := make([]byte, 4)
	binary.BigEndian.PutUint32(size, recordSize)
	result = append(result, size...)
	result = append(result, byte(len(senderPublic)))
	result = append(result, senderPublic...)
	result = append(result, ciphertext...)
	return result, nil
}

func parseRetryAfter(value string, now time.Time) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		retryAt := now.Add(time.Duration(seconds) * time.Second)
		return &retryAt
	}
	if retryAt, err := http.ParseTime(value); err == nil {
		retryAt = retryAt.UTC()
		return &retryAt
	}
	return nil
}

func encodeRawURL(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}
