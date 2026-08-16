package opencode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrUnauthorized = errors.New("usage unavailable: unauthorized")

type Window struct {
	Status   string    `json:"status"`
	Percent  float64   `json:"percent"`
	ResetsAt time.Time `json:"resetsAt"`
}

type Usage struct {
	Rolling Window `json:"rolling"`
	Weekly  Window `json:"weekly"`
	Monthly Window `json:"monthly"`
}

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func (c Client) Fetch(ctx context.Context, apiKey string) (Usage, error) {
	if strings.TrimSpace(apiKey) == "" {
		return Usage{}, errors.New("API key is required")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+"/v1/usage", nil)
	if err != nil {
		return Usage{}, fmt.Errorf("create usage request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "opencode-go-quota/0.1.0")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return Usage{}, fmt.Errorf("request usage: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized {
		return Usage{}, ErrUnauthorized
	}
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return Usage{}, fmt.Errorf("usage endpoint returned HTTP %d", response.StatusCode)
	}

	var payload struct {
		Usage Usage `json:"usage"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil {
		return Usage{}, fmt.Errorf("decode usage response: %w", err)
	}
	return payload.Usage, nil
}
