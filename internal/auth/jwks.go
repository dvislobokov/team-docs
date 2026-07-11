package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// jwksCache тянет и кеширует публичные RSA-ключи из JWKS-endpoint IdP
// (например Keycloak .../protocol/openid-connect/certs) и отдаёт keyfunc для
// проверки RS256-подписи. Ключи обновляются по TTL и при отсутствии нужного kid.
type jwksCache struct {
	url    string
	ttl    time.Duration
	client *http.Client

	mu     sync.RWMutex
	keys   map[string]*rsa.PublicKey
	expiry time.Time
}

func newJWKSCache(url string) *jwksCache {
	return &jwksCache{
		url:    url,
		ttl:    10 * time.Minute,
		client: &http.Client{Timeout: 10 * time.Second},
		keys:   map[string]*rsa.PublicKey{},
	}
}

func (j *jwksCache) keyfunc(token *jwt.Token) (any, error) {
	if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
		return nil, fmt.Errorf("jwks: unexpected signing method %v", token.Header["alg"])
	}
	kid, _ := token.Header["kid"].(string)
	return j.get(kid)
}

func (j *jwksCache) get(kid string) (*rsa.PublicKey, error) {
	j.mu.RLock()
	if time.Now().Before(j.expiry) {
		if k, ok := j.keys[kid]; ok {
			j.mu.RUnlock()
			return k, nil
		}
	}
	j.mu.RUnlock()

	// Нет ключа/просрочено — обновляем (kid могли ротировать).
	if err := j.refresh(); err != nil {
		return nil, err
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	if k, ok := j.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("jwks: signing key %q not found", kid)
}

func (j *jwksCache) refresh() error {
	resp, err := j.client.Get(j.url)
	if err != nil {
		return fmt.Errorf("jwks fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks fetch: status %d", resp.StatusCode)
	}

	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("jwks decode: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}

	j.mu.Lock()
	j.keys = keys
	j.expiry = time.Now().Add(j.ttl)
	j.mu.Unlock()
	return nil
}

// parseRSAPublicKey собирает rsa.PublicKey из base64url-компонентов n и e.
func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, err
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}
