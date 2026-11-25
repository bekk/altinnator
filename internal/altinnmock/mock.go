package altinnmock

import (
	"errors"
	"net/http"
)

// Mock plays the parts of Altinn that Altinnator calls, with the resources kept in memory
type Mock struct {
	mux      *http.ServeMux
	registry *registry
}

func New() *Mock {
	m := &Mock{mux: http.NewServeMux(), registry: newRegistry()}
	resources := &resourceRegistry{registry: m.registry}

	m.mux.HandleFunc("GET "+resourcePath+"/{id}", resources.handleGet)
	return m
}

func (m *Mock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mux.ServeHTTP(w, r)
}

// Resource is the newest version of the resource, as Altinn returns it, with versionId as an int64
func (m *Mock) Resource(id string) (map[string]any, bool) {
	return m.registry.get(id)
}

// SetResource makes the resource the newest version of the one with its identifier, creating it if there is none,
// like a change made in Altinn Studio. None of Altinn's checks are made
func (m *Mock) SetResource(resource map[string]any) error {
	if _, ok := resource["identifier"].(string); !ok {
		return errors.New("the resource has no identifier")
	}
	m.registry.put(deepCopy(resource))
	return nil
}
