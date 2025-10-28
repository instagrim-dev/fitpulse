//go:build integration

package testsupport

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestStartDgraphLoadsSchema(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, endpoint := StartDgraph(ctx, t)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	payload := []byte(`{"query":"schema(pred: [last_seen_at]) { predicate type }"}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/query", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("build schema request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("execute schema request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("unexpected status: %s", resp.Status)
	}

	var body struct {
		Data struct {
			Schema []struct {
				Predicate string `json:"predicate"`
				Type      string `json:"type"`
			} `json:"schema"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode schema response: %v", err)
	}

	for _, entry := range body.Data.Schema {
		if entry.Predicate == "last_seen_at" && entry.Type == "datetime" {
			return
		}
	}
	t.Fatalf("expected last_seen_at predicate in schema response: %+v", body.Data.Schema)
}
