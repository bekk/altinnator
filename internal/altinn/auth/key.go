package auth

import (
	"crypto/rsa"
	"errors"
	"strings"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-logr/logr"
)

type Key struct {
	jwk jose.JSONWebKey
}

var _ logr.Marshaler = Key{}

func ParseKey(jwk string) (Key, error) {
	var parsed jose.JSONWebKey
	if err := parsed.UnmarshalJSON([]byte(jwk)); err != nil {
		return Key{}, errors.New("the Maskinporten key is not a valid JWK")
	}
	private, ok := parsed.Key.(*rsa.PrivateKey)
	if !ok {
		return Key{}, errors.New("the Maskinporten key must be an RSA private key")
	}
	if err := private.Validate(); err != nil {
		return Key{}, errors.New("the Maskinporten key is not a valid RSA private key")
	}
	if strings.TrimSpace(parsed.KeyID) == "" {
		return Key{}, errors.New("the Maskinporten key must have a kid")
	}
	return Key{jwk: parsed}, nil
}

func (k Key) String() string { return "Key(kid=" + k.jwk.KeyID + ")" }

func (k Key) GoString() string { return k.String() }

func (k Key) MarshalLog() any { return k.String() }
