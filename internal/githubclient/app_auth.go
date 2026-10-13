package githubclient

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AppAuth struct {
	appID          int64
	installationID int64
	privateKey     *rsa.PrivateKey
	http           *http.Client
	mu             sync.Mutex
	cachedToken    string
	tokenExpiry    time.Time
}

func NewAppAuth(appID, installationID, keyPath string) (*AppAuth, error) {
	if appID == "" || installationID == "" || keyPath == "" {
		return nil, fmt.Errorf("github app auth not configured")
	}
	aid, err := strconv.ParseInt(appID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid GITHUB_APP_ID: %w", err)
	}
	iid, err := strconv.ParseInt(installationID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid GITHUB_APP_INSTALLATION_ID: %w", err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read github app key: %w", err)
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM in github app private key")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		parsed, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("parse github app key: %w", err)
		}
		rsaKey, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("github app key is not RSA")
		}
		key = rsaKey
	}
	return &AppAuth{
		appID:          aid,
		installationID: iid,
		privateKey:     key,
		http:           &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (a *AppAuth) InstallationToken(ctx context.Context) (string, error) {
	if a == nil {
		return "", fmt.Errorf("github app auth not configured")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cachedToken != "" && time.Now().Before(a.tokenExpiry.Add(-2*time.Minute)) {
		return a.cachedToken, nil
	}
	jwtToken, err := a.createJWT()
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", a.installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := a.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("github installation token status %d", res.StatusCode)
	}
	var payload struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := jsonDecode(res.Body, &payload); err != nil {
		return "", err
	}
	a.cachedToken = payload.Token
	a.tokenExpiry = payload.ExpiresAt
	return payload.Token, nil
}

func (a *AppAuth) createJWT() (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": a.appID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(a.privateKey)
}
