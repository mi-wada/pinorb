package registry

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New("")
	c.Endpoint = srv.URL
	return c
}

func TestVersionsParsesAndSkipsNonNumeric(t *testing.T) {
	var gotQuery, gotToken string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("Circle-Token")
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Query string `json:"query"`
		}
		json.Unmarshal(body, &req)
		gotQuery = req.Query
		io.WriteString(w, `{"data":{"orb":{"versions":[
			{"version":"5.3.0"},{"version":"5.1.1"},{"version":"volatile"},{"version":"dev:abc"}
		]}}}`)
	})
	c.Token = "secret"

	vs, err := c.Versions(context.Background(), "circleci/node")
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(vs) != 2 {
		t.Fatalf("got %d parsable versions, want 2 (non-numeric skipped)", len(vs))
	}
	// Guard against the truncation regression: the query must request a large
	// version count, not the registry's default of 10.
	if !strings.Contains(gotQuery, "count:") {
		t.Errorf("query missing count argument: %q", gotQuery)
	}
	if gotToken != "secret" {
		t.Errorf("Circle-Token header = %q, want %q", gotToken, "secret")
	}
}

func TestVersionsCaches(t *testing.T) {
	calls := 0
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		io.WriteString(w, `{"data":{"orb":{"versions":[{"version":"1.0.0"}]}}}`)
	})
	for range 3 {
		if _, err := c.Versions(context.Background(), "circleci/x"); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Errorf("server called %d times, want 1 (cached)", calls)
	}
}

func TestVersionsPrivateOrbError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":{"orb":null}}`)
	})
	_, err := c.Versions(context.Background(), "acme-org/secret")
	if err == nil || !strings.Contains(err.Error(), "private") {
		t.Fatalf("want private-orb hint, got %v", err)
	}
}
