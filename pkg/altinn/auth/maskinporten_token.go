package auth

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

const (
	ResourceWriteScope = "altinn:resourceregistry/resource.write"

	grantType = "urn:ietf:params:oauth:grant-type:jwt-bearer"

	grantLifetime = 60 * time.Second

	maxResponseBytes = 1 << 20
)

type maskinportenClient struct {
	tokenURL   string
	audience   string
	clientID   string
	signer     jose.Signer
	httpClient *http.Client
	now        func() time.Time
}

func newMaskinportenClient(
	tokenURL, clientID string, key Key, httpClient *http.Client, now func() time.Time,
) (*maskinportenClient, error) {
	parsed, err := url.Parse(tokenURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("the Maskinporten token URL %q is not an absolute URL", tokenURL)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key.jwk}, nil)
	if err != nil {
		return nil, errors.New("the Maskinporten key cannot sign with RS256")
	}
	return &maskinportenClient{
		tokenURL:   tokenURL,
		audience:   parsed.Scheme + "://" + parsed.Host + "/",
		clientID:   clientID,
		signer:     signer,
		httpClient: httpClient,
		now:        now,
	}, nil
}

type grantClaims struct {
	jwt.Claims
	Scope string `json:"scope"`
}

func (c *maskinportenClient) grant(now time.Time) (string, error) {
	claims := grantClaims{
		Issuer:   c.clientID,
		Audience: jwt.Audience{c.audience},
		ID:       rand.Text(),
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(grantLifetime)),
		Scope:    ResourceWriteScope,
	}
	grant, err := jwt.Signed(c.signer).Claims(claims).Serialize()
	if err != nil {
		return "", fmt.Errorf("signing the Maskinporten grant: %w", err)
	}
	return grant, nil
}

type maskinportenTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   *int64 `json:"expires_in"`
}

type maskinportenErrorResponse struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

func (c *maskinportenClient) fetch(ctx context.Context) (Token, error) {
	requestedAt := c.now()
	grant, err := c.grant(requestedAt)
	if err != nil {
		return Token{}, err
	}
	form := url.Values{"grant_type": {grantType}, "assertion": {grant}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, fmt.Errorf("creating the Maskinporten token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return Token{}, fmt.Errorf("calling Maskinporten: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return Token{}, fmt.Errorf("reading the response from Maskinporten: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return Token{}, maskinportenError(response.StatusCode, body)
	}

	var parsed maskinportenTokenResponse
	if json.Unmarshal(body, &parsed) != nil {
		return Token{}, errors.New("the response from Maskinporten is not JSON")
	}
	if parsed.AccessToken == "" {
		return Token{}, errors.New("the response from Maskinporten has no access_token")
	}
	if !bearerToken.MatchString(parsed.AccessToken) {
		return Token{}, errors.New("the access_token from Maskinporten is not a valid bearer token")
	}
	if parsed.ExpiresIn == nil {
		return Token{}, errors.New("the response from Maskinporten has no expires_in")
	}
	return Token{value: parsed.AccessToken, expiresAt: requestedAt.Add(time.Duration(*parsed.ExpiresIn) * time.Second)}, nil
}

func maskinportenError(status int, body []byte) error {
	var parsed maskinportenErrorResponse
	if json.Unmarshal(body, &parsed) == nil && parsed.Error != "" {
		return fmt.Errorf("the token request to Maskinporten failed with HTTP %d: %s: %s", status, parsed.Error, parsed.Description)
	}
	return fmt.Errorf("the token request to Maskinporten failed with HTTP %d", status)
}
