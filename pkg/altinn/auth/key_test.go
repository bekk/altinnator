package auth

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ = Describe("ParseKey", func() {
	It("reads an RSA private key with a kid", func() {
		key, err := ParseKey(jwkOf(privateKey, testKeyID))

		Expect(err).NotTo(HaveOccurred())
		Expect(key.String()).To(Equal("Key(kid=test-kid)"))
	})

	// The messages are matched in full, so they cannot quote the key
	DescribeTable("refuses",
		func(jwk func() string, message string) {
			_, err := ParseKey(jwk())

			Expect(err).To(MatchError(message))
		},
		Entry("an empty key",
			func() string { return "" },
			"the Maskinporten key is not a valid JWK"),
		Entry("a blank key",
			func() string { return "  " },
			"the Maskinporten key is not a valid JWK"),
		Entry("a key that is not JSON",
			func() string { return "not json" },
			"the Maskinporten key is not a valid JWK"),
		Entry("an RSA public key",
			func() string { return jwkOf(&privateKey.PublicKey, testKeyID) },
			"the Maskinporten key must be an RSA private key"),
		Entry("an EC private key",
			func() string {
				ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				Expect(err).NotTo(HaveOccurred())
				return jwkOf(ecKey, testKeyID)
			},
			"the Maskinporten key must be an RSA private key"),
		Entry("a key without a kid",
			func() string { return jwkOf(privateKey, "") },
			"the Maskinporten key must have a kid"),
		Entry("a key with a blank kid",
			func() string { return jwkOf(privateKey, " ") },
			"the Maskinporten key must have a kid"),
	)
})

var _ = Describe("Key and Token", func() {
	var (
		key         Key
		privatePart string
		token       Token
	)

	BeforeEach(func() {
		jwk := jwkOf(privateKey, testKeyID)
		var fields map[string]string
		Expect(json.Unmarshal([]byte(jwk), &fields)).To(Succeed())
		privatePart = fields["d"]
		Expect(privatePart).NotTo(BeEmpty())

		var err error
		key, err = ParseKey(jwk)
		Expect(err).NotTo(HaveOccurred())
		token = Token{value: "secret-token", expiresAt: time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)}
	})

	It("do not show the key or the token when formatted", func() {
		config := Config{ClientID: "client", Key: key}
		for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x"} {
			for _, value := range []any{key, token, config, &key, &token, &config} {
				formatted := fmt.Sprintf(format, value)
				Expect(formatted).NotTo(ContainSubstring(privatePart), format)
				Expect(formatted).NotTo(ContainSubstring("secret-token"), format)
			}
		}
		Expect(token.String()).To(Equal("Token(***, expires 2026-11-23T12:00:00Z)"))
	})

	It("do not show the key or the token in the log", func() {
		var out bytes.Buffer
		logger := zap.New(zap.WriteTo(&out))

		logger.Info("loaded", "key", key, "token", token, "config", Config{ClientID: "client", Key: key})

		Expect(out.String()).To(ContainSubstring("Key(kid=test-kid)"))
		Expect(out.String()).NotTo(ContainSubstring(privatePart))
		Expect(out.String()).NotTo(ContainSubstring("secret-token"))
	})

	It("can be made with NewToken, and still hides its value", func() {
		made := NewToken("secret-token", time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC))

		Expect(made.Reveal()).To(Equal("secret-token"))
		Expect(made).To(Equal(token))
		Expect(made.String()).To(Equal("Token(***, expires 2026-11-23T12:00:00Z)"))
	})
})
