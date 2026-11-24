package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/go-jose/go-jose/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAuth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auth Suite")
}

const testKeyID = "test-kid"

// privateKey signs the grants in the tests. It is made once, since making an RSA key is slow.
var privateKey *rsa.PrivateKey

var _ = BeforeSuite(func() {
	var err error
	privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
})

// jwkOf gives key as a JWK, like the Secret holds it. An empty kid leaves the kid out.
func jwkOf(key any, kid string) string {
	encoded, err := jose.JSONWebKey{Key: key, KeyID: kid}.MarshalJSON()
	Expect(err).NotTo(HaveOccurred())
	return string(encoded)
}
