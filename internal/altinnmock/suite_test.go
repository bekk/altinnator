package altinnmock_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAltinnMock(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Altinn Mock Suite")
}
