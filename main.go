package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	void* ptr;
	size_t len;
} cliproxy_buffer;

typedef int (*cliproxy_host_call_fn)(void*, const char*, const uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_host_free_fn)(void*, size_t);

typedef struct {
	uint32_t abi_version;
	void* host_ctx;
	cliproxy_host_call_fn call;
	cliproxy_host_free_fn free_buffer;
} cliproxy_host_api;

typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);

typedef struct {
	uint32_t abi_version;
	cliproxy_plugin_call_fn call;
	cliproxy_plugin_free_fn free_buffer;
	cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;

extern int cliproxyPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void cliproxyPluginFree(void*, size_t);
extern void cliproxyPluginShutdown(void);
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
	"unsafe"

	"github.com/duu261/opencode-go-pool/internal/accounts"
	"github.com/duu261/opencode-go-pool/internal/cliproxyconfig"
	"github.com/duu261/opencode-go-pool/internal/opencode"
	"github.com/duu261/opencode-go-pool/internal/pool"
)

const (
	abiVersion = 1
	pluginID   = "opencode-go-quota"
	pluginName = "OpenCode Go Pool"

	resourcePath       = "/status"
	fullResourcePath   = "/v0/resource/plugins/" + pluginID + resourcePath
	quotaRoutePath     = "/plugins/" + pluginID + "/quotas"
	fullQuotaRoutePath = "/v0/management" + quotaRoutePath
	accountsRoutePath  = "/plugins/" + pluginID + "/accounts"
	fullAccountsPath   = "/v0/management" + accountsRoutePath
)

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type registration struct {
	SchemaVersion uint32                   `json:"schema_version"`
	Metadata      metadata                 `json:"metadata"`
	Capabilities  registrationCapabilities `json:"capabilities"`
}

type metadata struct {
	Name             string        `json:"Name"`
	Version          string        `json:"Version"`
	Author           string        `json:"Author"`
	GitHubRepository string        `json:"GitHubRepository"`
	ConfigFields     []configField `json:"ConfigFields"`
}

type configField struct {
	Name        string `json:"Name"`
	Type        string `json:"Type"`
	Description string `json:"Description"`
}

type registrationCapabilities struct {
	ManagementAPI bool `json:"management_api"`
}

type managementRegistration struct {
	Routes    []managementRoute    `json:"routes,omitempty"`
	Resources []managementResource `json:"resources"`
}

type managementRoute struct {
	Method      string `json:"Method"`
	Path        string `json:"Path"`
	Description string `json:"Description"`
}

type managementResource struct {
	Path        string `json:"Path"`
	Menu        string `json:"Menu"`
	Description string `json:"Description"`
}

type managementRequest struct {
	Method string
	Path   string
	Query  map[string][]string
	Body   []byte
}

type managementResponse struct {
	StatusCode int         `json:"StatusCode"`
	Headers    http.Header `json:"Headers"`
	Body       []byte      `json:"Body"`
}

func main() {}

//export cliproxy_plugin_init
func cliproxy_plugin_init(host *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if plugin == nil {
		return 1
	}
	_ = host
	plugin.abi_version = C.uint32_t(abiVersion)
	plugin.call = C.cliproxy_plugin_call_fn(C.cliproxyPluginCall)
	plugin.free_buffer = C.cliproxy_plugin_free_fn(C.cliproxyPluginFree)
	plugin.shutdown = C.cliproxy_plugin_shutdown_fn(C.cliproxyPluginShutdown)
	return 0
}

//export cliproxyPluginCall
func cliproxyPluginCall(method *C.char, request *C.uint8_t, requestLen C.size_t, response *C.cliproxy_buffer) C.int {
	if response != nil {
		response.ptr = nil
		response.len = 0
	}
	if method == nil {
		writeResponse(response, errorEnvelope("invalid_method", "method is required"))
		return 1
	}
	var requestBytes []byte
	if request != nil && requestLen > 0 {
		requestBytes = C.GoBytes(unsafe.Pointer(request), C.int(requestLen))
	}
	raw, err := handleMethod(C.GoString(method), requestBytes)
	if err != nil {
		writeResponse(response, errorEnvelope("plugin_error", err.Error()))
		return 1
	}
	writeResponse(response, raw)
	return 0
}

//export cliproxyPluginFree
func cliproxyPluginFree(ptr unsafe.Pointer, length C.size_t) {
	if ptr != nil {
		C.free(ptr)
	}
	_ = length
}

//export cliproxyPluginShutdown
func cliproxyPluginShutdown() {}

func handleMethod(method string, request []byte) ([]byte, error) {
	switch method {
	case "plugin.register", "plugin.reconfigure":
		if err := configurePlugin(request); err != nil {
			return nil, err
		}
		return okEnvelope(registration{
			SchemaVersion: 1,
			Metadata: metadata{
				Name:             pluginName,
				Version:          "0.3.0",
				Author:           "Duu",
				GitHubRepository: "https://github.com/duu261/opencode-go-pool",
				ConfigFields: []configField{
					{Name: "config_path", Type: "string", Description: "Protected CLIProxyAPI config file path."},
					{Name: "accounts_path", Type: "string", Description: "Writable plaintext OpenCode account registry path."},
					{Name: "provider_names", Type: "array", Description: "Additional OpenAI-compatible provider names to include."},
					{Name: "max_concurrency", Type: "integer", Description: "Concurrent quota requests, 1-16."},
					{Name: "timeout_seconds", Type: "integer", Description: "Whole-report timeout, 1-60 seconds."},
				},
			},
			Capabilities: registrationCapabilities{ManagementAPI: true},
		})
	case "management.register":
		return okEnvelope(managementRegistration{
			Routes: []managementRoute{
				{Method: http.MethodGet, Path: quotaRoutePath, Description: "Authenticated OpenCode Go quota pool data."},
				{Method: http.MethodGet, Path: accountsRoutePath, Description: "Authenticated OpenCode Go account registry and quota data."},
				{Method: http.MethodPut, Path: accountsRoutePath, Description: "Replace the OpenCode Go account registry."},
			},
			Resources: []managementResource{{
				Path:        resourcePath,
				Menu:        pluginName,
				Description: "Read-only OpenCode Go quota pool status.",
			}},
		})
	case "management.handle":
		return handleManagement(request)
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func handleManagement(raw []byte) ([]byte, error) {
	var request managementRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
	}
	switch request.Path {
	case fullQuotaRoutePath, quotaRoutePath:
		return handleQuotaManagement()
	case fullAccountsPath, accountsRoutePath:
		return handleAccountsManagement(request)
	case "", resourcePath, fullResourcePath:
		return okEnvelope(htmlResponse(http.StatusOK, quotaPageHTML))
	default:
		return okEnvelope(htmlResponse(http.StatusNotFound, "<h1>Not found</h1>"))
	}
}

type accountView struct {
	accounts.Account
	KeyID       string          `json:"key_id"`
	PoolEnabled bool            `json:"pool_enabled"`
	Status      string          `json:"status"`
	Message     string          `json:"message,omitempty"`
	Usage       *opencode.Usage `json:"usage,omitempty"`
}

type accountSnapshot struct {
	GeneratedAt time.Time                 `json:"generated_at"`
	Revision    string                    `json:"revision"`
	Providers   []cliproxyconfig.Provider `json:"providers"`
	Accounts    []accountView             `json:"accounts"`
}

type accountRegistryRequest struct {
	Revision string              `json:"revision"`
	Accounts *[]accounts.Account `json:"accounts"`
}

func handleAccountsManagement(request managementRequest) ([]byte, error) {
	config := currentPluginConfig()
	switch request.Method {
	case "", http.MethodGet:
		return accountSnapshotResponse(config)
	case http.MethodPut:
		var payload accountRegistryRequest
		if err := json.Unmarshal(request.Body, &payload); err != nil {
			return okEnvelope(jsonResponse(http.StatusBadRequest, map[string]string{
				"error": "invalid_accounts", "message": "Account registry body must contain an accounts array.",
			}))
		}
		if payload.Accounts == nil {
			return okEnvelope(jsonResponse(http.StatusBadRequest, map[string]string{
				"error": "invalid_accounts", "message": "Account registry body must contain an accounts array.",
			}))
		}
		if payload.Revision == "" {
			return okEnvelope(jsonResponse(http.StatusBadRequest, map[string]string{
				"error": "missing_revision", "message": "Account registry revision is required.",
			}))
		}
		currentAccounts, currentRevision, err := accounts.LoadWithRevision(config.AccountsPath)
		if err != nil {
			return okEnvelope(jsonResponse(http.StatusInternalServerError, map[string]string{
				"error": "account_registry_load_failed", "message": err.Error(),
			}))
		}
		if payload.Revision != currentRevision {
			return okEnvelope(jsonResponse(http.StatusConflict, map[string]string{
				"error": "account_registry_conflict", "message": "Account registry changed in another tab. Refresh and retry.", "revision": currentRevision,
			}))
		}
		providers, err := cliproxyconfig.DiscoverProviders(config.ConfigPath, config.ProviderNames)
		if err != nil {
			return okEnvelope(jsonResponse(http.StatusInternalServerError, map[string]string{
				"error": "provider_discovery_failed", "message": "Unable to read CLIProxyAPI provider configuration.",
			}))
		}
		eligibleProviders := make(map[string]struct{}, len(providers))
		for _, provider := range providers {
			eligibleProviders[strings.ToLower(provider.Name)] = struct{}{}
		}
		for _, account := range *payload.Accounts {
			if _, eligible := eligibleProviders[strings.ToLower(strings.TrimSpace(account.ProviderName))]; !eligible {
				return okEnvelope(jsonResponse(http.StatusUnprocessableEntity, map[string]string{
					"error": "invalid_provider", "message": "Each account must use an eligible OpenCode provider.",
				}))
			}
		}
		nextByKey := make(map[string]accounts.Account, len(*payload.Accounts))
		for _, account := range *payload.Accounts {
			nextByKey[account.APIKey] = account
		}
		credentials, err := cliproxyconfig.Discover(config.ConfigPath, config.ProviderNames)
		if err != nil {
			return okEnvelope(jsonResponse(http.StatusInternalServerError, map[string]string{
				"error": "credential_discovery_failed", "message": "Unable to read CLIProxyAPI credential configuration.",
			}))
		}
		enabledKeys := make(map[string]struct{}, len(credentials))
		for _, credential := range credentials {
			if credential.Enabled {
				enabledKeys[credential.APIKey] = struct{}{}
			}
		}
		for _, account := range currentAccounts {
			if next, kept := nextByKey[account.APIKey]; kept {
				if !strings.EqualFold(account.ProviderName, next.ProviderName) {
					return okEnvelope(jsonResponse(http.StatusConflict, map[string]string{
						"error": "provider_reassignment", "message": "Account providers cannot be changed. Create a new parked account instead.",
					}))
				}
				continue
			}
			if _, enabled := enabledKeys[account.APIKey]; enabled {
				return okEnvelope(jsonResponse(http.StatusConflict, map[string]string{
					"error": "active_account_delete", "message": "Disable active accounts before deleting them.",
				}))
			}
		}
		newRevision, err := accounts.Replace(config.AccountsPath, *payload.Accounts, payload.Revision)
		if errors.Is(err, accounts.ErrRevisionConflict) {
			return okEnvelope(jsonResponse(http.StatusConflict, map[string]string{
				"error": "account_registry_conflict", "message": "Account registry changed in another tab. Refresh and retry.", "revision": newRevision,
			}))
		}
		if err != nil {
			return okEnvelope(jsonResponse(http.StatusUnprocessableEntity, map[string]string{
				"error": "account_registry_save_failed", "message": err.Error(),
			}))
		}
		return okEnvelope(jsonResponse(http.StatusOK, map[string]any{"status": "ok", "count": len(*payload.Accounts), "revision": newRevision}))
	default:
		return okEnvelope(jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"}))
	}
}

func accountSnapshotResponse(config pluginConfig) ([]byte, error) {
	registry, registryRevision, err := accounts.LoadWithRevision(config.AccountsPath)
	if err != nil {
		return okEnvelope(jsonResponse(http.StatusInternalServerError, map[string]string{
			"error": "account_registry_load_failed", "message": err.Error(),
		}))
	}
	credentials, err := cliproxyconfig.Discover(config.ConfigPath, config.ProviderNames)
	if err != nil {
		return okEnvelope(jsonResponse(http.StatusInternalServerError, map[string]string{
			"error": "credential_discovery_failed", "message": "Unable to read CLIProxyAPI credential configuration.",
		}))
	}
	providers, err := cliproxyconfig.DiscoverProviders(config.ConfigPath, config.ProviderNames)
	if err != nil {
		return okEnvelope(jsonResponse(http.StatusInternalServerError, map[string]string{
			"error": "provider_discovery_failed", "message": "Unable to read CLIProxyAPI provider configuration.",
		}))
	}
	providerBaseURL := make(map[string]string, len(providers))
	for _, provider := range providers {
		providerBaseURL[strings.ToLower(provider.Name)] = provider.BaseURL
	}
	quotaCredentials := append([]cliproxyconfig.Credential(nil), credentials...)
	quotaIndexByKey := make(map[string]int, len(quotaCredentials)+len(registry))
	for index, credential := range quotaCredentials {
		quotaIndexByKey[credential.APIKey] = index
	}
	for _, account := range registry {
		if index, exists := quotaIndexByKey[account.APIKey]; exists {
			quotaCredentials[index].Enabled = true
			continue
		}
		baseURL, exists := providerBaseURL[strings.ToLower(account.ProviderName)]
		if !exists {
			continue
		}
		quotaIndexByKey[account.APIKey] = len(quotaCredentials)
		quotaCredentials = append(quotaCredentials, cliproxyconfig.Credential{
			ProviderName: account.ProviderName,
			BaseURL:      baseURL,
			KeyID:        cliproxyconfig.KeyID(account.APIKey),
			APIKey:       account.APIKey,
			Enabled:      true,
		})
	}
	timeout := time.Duration(config.TimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	results := pool.Collect(ctx, quotaCredentials, &http.Client{Timeout: timeout}, config.MaxConcurrency)

	credentialByKey := make(map[string]cliproxyconfig.Credential, len(credentials))
	resultByKeyID := make(map[string]pool.Result, len(results))
	for _, credential := range credentials {
		credentialByKey[credential.APIKey] = credential
	}
	for _, result := range results {
		resultByKeyID[result.KeyID] = result
	}

	views := make([]accountView, 0, len(registry)+len(credentials))
	registered := make(map[string]struct{}, len(registry))
	for _, account := range registry {
		registered[account.APIKey] = struct{}{}
		view := accountView{Account: account, KeyID: cliproxyconfig.KeyID(account.APIKey), Status: pool.StatusDisabled}
		if credential, exists := credentialByKey[account.APIKey]; exists {
			view.KeyID = credential.KeyID
			view.PoolEnabled = credential.Enabled
			if view.ProviderName == "" {
				view.ProviderName = credential.ProviderName
			}
		}
		if result, ok := resultByKeyID[view.KeyID]; ok {
			view.Status = result.Status
			view.Message = result.Message
			view.Usage = result.Usage
		}
		views = append(views, view)
	}
	for _, credential := range credentials {
		if _, exists := registered[credential.APIKey]; exists {
			continue
		}
		view := accountView{
			Account:     accounts.Account{APIKey: credential.APIKey, ProviderName: credential.ProviderName},
			KeyID:       credential.KeyID,
			PoolEnabled: credential.Enabled,
		}
		if result, ok := resultByKeyID[credential.KeyID]; ok {
			view.Status = result.Status
			view.Message = result.Message
			view.Usage = result.Usage
		}
		views = append(views, view)
	}
	return okEnvelope(jsonResponse(http.StatusOK, accountSnapshot{GeneratedAt: time.Now().UTC(), Revision: registryRevision, Providers: providers, Accounts: views}))
}

type quotaSnapshot struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Results     []pool.Result `json:"results"`
}

func handleQuotaManagement() ([]byte, error) {
	config := currentPluginConfig()
	credentials, err := cliproxyconfig.Discover(config.ConfigPath, config.ProviderNames)
	if err != nil {
		return okEnvelope(jsonResponse(http.StatusInternalServerError, map[string]string{
			"error":   "credential_discovery_failed",
			"message": "Unable to read CLIProxyAPI credential configuration.",
		}))
	}
	timeout := time.Duration(config.TimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	results := pool.Collect(ctx, credentials, &http.Client{Timeout: timeout}, config.MaxConcurrency)
	return okEnvelope(jsonResponse(http.StatusOK, quotaSnapshot{
		GeneratedAt: time.Now().UTC(),
		Results:     results,
	}))
}

func htmlResponse(status int, body string) managementResponse {
	return managementResponse{
		StatusCode: status,
		Headers: http.Header{
			"Content-Type":            {"text/html; charset=utf-8"},
			"Cache-Control":           {"no-store"},
			"Content-Security-Policy": {"default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; frame-ancestors 'self'; form-action 'none'"},
			"Referrer-Policy":         {"no-referrer"},
			"X-Content-Type-Options":  {"nosniff"},
		},
		Body: []byte(body),
	}
}

func jsonResponse(status int, value any) managementResponse {
	body, err := json.Marshal(value)
	if err != nil {
		body = []byte(`{"error":"encode_failed"}`)
		status = http.StatusInternalServerError
	}
	return managementResponse{
		StatusCode: status,
		Headers: http.Header{
			"Content-Type":  {"application/json; charset=utf-8"},
			"Cache-Control": {"no-store"},
		},
		Body: body,
	}
}

func okEnvelope(result any) ([]byte, error) {
	rawResult, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return json.Marshal(envelope{OK: true, Result: rawResult})
}

func errorEnvelope(code, message string) []byte {
	raw, _ := json.Marshal(envelope{OK: false, Error: &envelopeError{Code: code, Message: message}})
	return raw
}

func writeResponse(response *C.cliproxy_buffer, raw []byte) {
	if response == nil || len(raw) == 0 {
		return
	}
	ptr := C.CBytes(raw)
	if ptr == nil {
		return
	}
	response.ptr = ptr
	response.len = C.size_t(len(raw))
}
