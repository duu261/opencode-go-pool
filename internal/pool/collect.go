package pool

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/duu261/opencode-go-quota/internal/cliproxyconfig"
	"github.com/duu261/opencode-go-quota/internal/opencode"
)

const (
	StatusHealthy     = "healthy"
	StatusUnavailable = "unavailable"
	StatusDisabled    = "disabled"
	StatusError       = "error"
)

type Result struct {
	ProviderName string          `json:"provider_name"`
	KeyID        string          `json:"key_id"`
	Status       string          `json:"status"`
	Message      string          `json:"message,omitempty"`
	Usage        *opencode.Usage `json:"usage,omitempty"`
}

func Collect(ctx context.Context, credentials []cliproxyconfig.Credential, httpClient *http.Client, maxConcurrency int) []Result {
	if maxConcurrency < 1 {
		maxConcurrency = 4
	}
	if maxConcurrency > 16 {
		maxConcurrency = 16
	}

	results := make([]Result, len(credentials))
	semaphore := make(chan struct{}, maxConcurrency)
	var waitGroup sync.WaitGroup
	for index, credential := range credentials {
		results[index] = Result{ProviderName: credential.ProviderName, KeyID: credential.KeyID}
		if !credential.Enabled {
			results[index].Status = StatusDisabled
			results[index].Message = "Excluded from CLIProxy routing by weight"
			continue
		}

		waitGroup.Add(1)
		go func(index int, credential cliproxyconfig.Credential) {
			defer waitGroup.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				results[index].Status = StatusError
				results[index].Message = "Quota request cancelled"
				return
			}

			client := opencode.Client{BaseURL: credential.BaseURL, HTTPClient: httpClient}
			usage, err := client.Fetch(ctx, credential.APIKey)
			switch {
			case err == nil:
				results[index].Status = StatusHealthy
				results[index].Usage = &usage
			case errors.Is(err, opencode.ErrUnauthorized):
				results[index].Status = StatusUnavailable
				results[index].Message = "Usage endpoint rejected this key"
			default:
				results[index].Status = StatusError
				results[index].Message = err.Error()
			}
		}(index, credential)
	}
	waitGroup.Wait()
	return results
}
