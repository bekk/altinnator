package names

import (
	"fmt"
	"hash/fnv"

	"github.com/kartverket/altinnator/api/v1alpha1"
)

type nameBase struct {
	altinnResourceName string
}

func newNameBase(ar v1alpha1.AltinnResource) nameBase {
	return nameBase{
		altinnResourceName: ar.Name,
	}
}

// ShortHash returns the first 8 hex characters of an FNV-32a hash of s.
// Useful for producing short, stable, Kubernetes-safe name suffixes.
func ShortHash(s string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return fmt.Sprintf("%08x", h.Sum32())
}

func WithShortHashSuffix(s string) string {
	return fmt.Sprintf("%s-%s", s, ShortHash(s))
}

type DummyNamer struct {
	nameBase
}

func NewDummyNamer(ar v1alpha1.AltinnResource) DummyNamer {
	return DummyNamer{newNameBase(ar)}
}

func (n DummyNamer) ConfigMapName() string {
	return WithShortHashSuffix(n.altinnResourceName)
}
