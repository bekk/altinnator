package auth

import (
	"context"
	"net/http"
	"time"
)

const (
	refreshLeeway = 30 * time.Second
	fetchTimeout  = 10 * time.Second
)

type Config struct {
	MaskinportenTokenURL string
	PlatformBaseURL      string
	ClientID             string
	Key                  Key
}

type Provider struct {
	maskinporten       *maskinportenClient
	exchanger          *altinnExchanger
	maskinportenTokens *tokenCache
	altinnTokens       *tokenCache
}

func NewProvider(config Config, httpClient *http.Client) (*Provider, error) {
	return newProvider(config, httpClient, time.Now)
}

func newProvider(config Config, httpClient *http.Client, now func() time.Time) (*Provider, error) {
	maskinporten, err := newMaskinportenClient(config.MaskinportenTokenURL, config.ClientID, config.Key, httpClient, now)
	if err != nil {
		return nil, err
	}
	return &Provider{
		maskinporten:       maskinporten,
		exchanger:          newAltinnExchanger(config.PlatformBaseURL, httpClient),
		maskinportenTokens: &tokenCache{now: now, leeway: refreshLeeway, timeout: fetchTimeout},
		altinnTokens:       &tokenCache{now: now, leeway: refreshLeeway, timeout: fetchTimeout},
	}, nil
}

func (p *Provider) Token(ctx context.Context) (Token, error) {
	return p.altinnTokens.get(ctx, func(ctx context.Context) (Token, error) {
		maskinportenToken, err := p.maskinportenTokens.get(ctx, p.maskinporten.fetch)
		if err != nil {
			return Token{}, err
		}
		token, err := p.exchanger.exchange(ctx, maskinportenToken)
		if err != nil {
			p.maskinportenTokens.clear(maskinportenToken.value)
			return Token{}, err
		}
		return token, nil
	})
}

func (p *Provider) Refused(token Token) {
	p.altinnTokens.clear(token.value)
}
