package auth

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"team-docs/internal/config"
)

// appleClientSecret генерирует client_secret для Sign in with Apple:
// ES256-JWT, подписанный приватным ключом .p8 (iss=teamId, sub=clientId).
// Apple допускает срок до 6 месяцев; генерируем короткоживущий на каждый обмен.
func appleClientSecret(c config.AppleClientSettings) (string, error) {
	keyPEM := strings.ReplaceAll(config.SecretString(c.PrivateKey), `\n`, "\n") // ключ мог прийти через env одной строкой
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return "", fmt.Errorf("apple: privateKey не похож на PEM (.p8)")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("apple: разбор ключа: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("apple: ожидался EC-ключ (ES256)")
	}

	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": c.TeamID,
		"iat": now.Unix(),
		"exp": now.Add(10 * time.Minute).Unix(),
		"aud": "https://appleid.apple.com",
		"sub": c.ClientID,
	})
	tok.Header["kid"] = c.KeyID
	return tok.SignedString(key)
}
