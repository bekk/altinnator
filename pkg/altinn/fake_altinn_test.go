package altinn_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/kartverket/altinnator/internal/altinn/auth"
	. "github.com/onsi/ginkgo/v2"
)

type fakeTokenProvider struct {
	refused []string
}

func (f *fakeTokenProvider) Token(context.Context) (auth.Token, error) {
	return auth.NewToken("test-token", time.Now().Add(time.Hour)), nil
}

func (f *fakeTokenProvider) Refused(token auth.Token) {
	f.refused = append(f.refused, token.Reveal())
}

type writeRequest struct {
	method        string
	path          string
	authorization string
	contentType   string
	body          string
}

type fakeAltinn struct {
	server         *httptest.Server
	resourceStatus int
	resourceBody   string
	writeStatus    int
	reads          int
	writes         []writeRequest
}

func newFakeAltinn(resourceBody string) *fakeAltinn {
	fake := &fakeAltinn{resourceStatus: http.StatusOK, resourceBody: resourceBody, writeStatus: http.StatusOK}
	fake.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fake.reads++
			w.WriteHeader(fake.resourceStatus)
			_, _ = w.Write([]byte(fake.resourceBody))
			return
		}
		body, _ := io.ReadAll(r.Body)
		fake.writes = append(fake.writes, writeRequest{
			method:        r.Method,
			path:          r.URL.Path,
			authorization: r.Header.Get("Authorization"),
			contentType:   r.Header.Get("Content-Type"),
			body:          string(body),
		})
		w.WriteHeader(fake.writeStatus)
	}))
	DeferCleanup(fake.server.Close)
	return fake
}
