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
	"net/http"
	"time"
	"unsafe"

	"github.com/duu261/opencode-go-quota/internal/cliproxyconfig"
	"github.com/duu261/opencode-go-quota/internal/pool"
)

const (
	abiVersion = 1
	pluginID   = "opencode-go-quota"
	pluginName = "OpenCode Go Quota"

	resourcePath       = "/status"
	fullResourcePath   = "/v0/resource/plugins/" + pluginID + resourcePath
	quotaRoutePath     = "/plugins/" + pluginID + "/quotas"
	fullQuotaRoutePath = "/v0/management" + quotaRoutePath
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
				Version:          "0.2.1",
				Author:           "Duu",
				GitHubRepository: "https://github.com/duu261/opencode-go-quota",
				ConfigFields: []configField{
					{Name: "config_path", Type: "string", Description: "Protected CLIProxyAPI config file path."},
					{Name: "provider_names", Type: "array", Description: "Additional OpenAI-compatible provider names to include."},
					{Name: "max_concurrency", Type: "integer", Description: "Concurrent quota requests, 1-16."},
					{Name: "timeout_seconds", Type: "integer", Description: "Whole-report timeout, 1-60 seconds."},
				},
			},
			Capabilities: registrationCapabilities{ManagementAPI: true},
		})
	case "management.register":
		return okEnvelope(managementRegistration{
			Routes: []managementRoute{{
				Method:      http.MethodGet,
				Path:        quotaRoutePath,
				Description: "Authenticated OpenCode Go quota pool data.",
			}},
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
	case "", resourcePath, fullResourcePath:
		return okEnvelope(htmlResponse(http.StatusOK, quotaPageHTML))
	default:
		return okEnvelope(htmlResponse(http.StatusNotFound, "<h1>Not found</h1>"))
	}
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
