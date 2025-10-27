package outbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SchemaRegistryClient provides minimal interactions with Confluent Schema Registry.
type SchemaRegistryClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewSchemaRegistryClient constructs a client with sane defaults.
func NewSchemaRegistryClient(baseURL string) *SchemaRegistryClient {
	return &SchemaRegistryClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// EnsureSchema ensures a schema subject exists and returns the schema ID.
func (c *SchemaRegistryClient) EnsureSchema(ctx context.Context, subject string, schema string) (int, error) {
	if id, err := c.fetchLatest(ctx, subject); err == nil {
		return id, nil
	}

	return c.register(ctx, subject, schema)
}

func (c *SchemaRegistryClient) fetchLatest(ctx context.Context, subject string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/subjects/%s/versions/latest", c.baseURL, subject), nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return 0, fmt.Errorf("schema subject not found")
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("schema registry error: %s", body)
	}

	var payload struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, err
	}
	return payload.ID, nil
}

func (c *SchemaRegistryClient) register(ctx context.Context, subject string, schema string) (int, error) {
	body, err := json.Marshal(map[string]any{
		"schemaType": "JSON",
		"schema":     schema,
	})
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/subjects/%s/versions", c.baseURL, subject), bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("schema registry register error: %s", data)
	}

	var payload struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, err
	}
	return payload.ID, nil
}
