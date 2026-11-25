package altinn_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAltinn(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Altinn Suite")
}
