// Package registry queries the CircleCI orb registry for published versions.
package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/mi-wada/pinorb/internal/orb"
)

const defaultEndpoint = "https://circleci.com/graphql-unstable"

// Client fetches orb version lists from the CircleCI registry.
type Client struct {
	HTTP     *http.Client
	Endpoint string
	Token    string // optional Circle-Token; required for private orbs and to ease rate limits

	cache map[string][]orb.Version
}

// New returns a Client. token may be empty for public orbs.
func New(token string) *Client {
	return &Client{
		HTTP:     http.DefaultClient,
		Endpoint: defaultEndpoint,
		Token:    token,
		cache:    map[string][]orb.Version{},
	}
}

// The registry returns only the 10 most recent versions unless an explicit
// count is given, which would hide older minor lines (e.g. resolving "@5.1"
// when 5.3 exists). Request a generous upper bound instead.
const versionsQuery = `query($name: String!) { orb(name: $name) { versions(count: 1000) { version } } }`

type graphQLResponse struct {
	Data struct {
		Orb *struct {
			Versions []struct {
				Version string `json:"version"`
			} `json:"versions"`
		} `json:"orb"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// Versions returns the published versions of an orb (e.g. "circleci/aws-cli"),
// parsed and de-duplicated. Results are cached per Client.
func (c *Client) Versions(ctx context.Context, name string) ([]orb.Version, error) {
	if v, ok := c.cache[name]; ok {
		return v, nil
	}

	body, err := json.Marshal(map[string]any{
		"query":     versionsQuery,
		"variables": map[string]string{"name": name},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Circle-Token", c.Token)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("circleci registry returned %s: %s", resp.Status, truncate(raw, 200))
	}

	var gql graphQLResponse
	if err := json.Unmarshal(raw, &gql); err != nil {
		return nil, fmt.Errorf("decode registry response: %w", err)
	}
	if len(gql.Errors) > 0 {
		return nil, fmt.Errorf("registry error: %s", gql.Errors[0].Message)
	}
	if gql.Data.Orb == nil {
		return nil, fmt.Errorf("orb %q not found (it may be private; pass a token with --token or $CIRCLE_TOKEN)", name)
	}

	var versions []orb.Version
	for _, v := range gql.Data.Orb.Versions {
		if pv, ok := orb.ParseVersion(v.Version); ok {
			versions = append(versions, pv)
		}
	}
	c.cache[name] = versions
	return versions, nil
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "..."
	}
	return string(b)
}
