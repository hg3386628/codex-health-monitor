package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
    void* ptr;
    size_t len;
} cliproxy_buffer;

typedef struct {
    uint32_t abi_version;
    void* host_ctx;
    int (*call)(void*, const char*, const uint8_t*, size_t, cliproxy_buffer*);
    void (*free_buffer)(void*, size_t);
} cliproxy_host_api;

typedef struct {
    uint32_t abi_version;
    int (*call)(char*, uint8_t*, size_t, cliproxy_buffer*);
    void (*free_buffer)(void*, size_t);
    void (*shutdown)(void);
} cliproxy_plugin_api;

typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);

extern int cliproxyPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void cliproxyPluginFree(void*, size_t);
extern void cliproxyPluginShutdown(void);

static const cliproxy_host_api* stored_host;

static void store_host_api(const cliproxy_host_api* host) {
    stored_host = host;
}

static int call_host_api(const char* method, const uint8_t* request, size_t request_len, cliproxy_buffer* response) {
    if (stored_host == NULL || stored_host->call == NULL) {
        return 1;
    }
    return stored_host->call(stored_host->host_ctx, method, request, request_len, response);
}

static void free_host_buffer(void* ptr, size_t len) {
    if (stored_host != NULL && stored_host->free_buffer != NULL && ptr != NULL) {
        stored_host->free_buffer(ptr, len);
    }
}
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"unsafe"
)

const (
	pluginName = "codex-health-monitor"
	abiVersion = 1
)

var pluginVersion = "0.1.10"

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Retryable  bool   `json:"retryable,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
}

type registration struct {
	SchemaVersion int                  `json:"schema_version"`
	Capabilities  capabilities         `json:"capabilities"`
	Metadata      registrationMetadata `json:"metadata"`
}

type capabilities struct {
	ManagementAPI bool `json:"management_api"`
}

type registrationMetadata struct {
	Name             string        `json:"Name"`
	Version          string        `json:"Version"`
	Author           string        `json:"Author"`
	GitHubRepository string        `json:"GitHubRepository"`
	ConfigFields     []configField `json:"ConfigFields"`
}

type configField struct {
	Name        string   `json:"Name"`
	Type        string   `json:"Type"`
	EnumValues  []string `json:"EnumValues,omitempty"`
	Description string   `json:"Description,omitempty"`
}

type managementRegistration struct {
	Routes    []managementRoute    `json:"routes"`
	Resources []managementResource `json:"resources"`
}

type managementRoute struct {
	Method string `json:"Method"`
	Path   string `json:"Path"`
}

type managementResource struct {
	Path        string `json:"Path"`
	Menu        string `json:"Menu"`
	Description string `json:"Description"`
}

type managementRequest struct {
	Method         string              `json:"method"`
	Path           string              `json:"path"`
	Headers        map[string][]string `json:"headers,omitempty"`
	Query          map[string][]string `json:"query,omitempty"`
	Body           []byte              `json:"body,omitempty"`
	HostCallbackID string              `json:"host_callback_id,omitempty"`
}

type managementResponse struct {
	StatusCode int                 `json:"StatusCode"`
	Headers    map[string][]string `json:"Headers,omitempty"`
	Body       []byte              `json:"Body,omitempty"`
}

var (
	runtimeMu     sync.RWMutex
	pluginRuntime *Runtime
)

//export cliproxy_plugin_init
func cliproxy_plugin_init(host *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if host == nil || plugin == nil || uint32(host.abi_version) != abiVersion {
		return 1
	}
	C.store_host_api(host)
	plugin.abi_version = C.uint32_t(abiVersion)
	plugin.call = C.cliproxy_plugin_call_fn(C.cliproxyPluginCall)
	plugin.free_buffer = C.cliproxy_plugin_free_fn(C.cliproxyPluginFree)
	plugin.shutdown = C.cliproxy_plugin_shutdown_fn(C.cliproxyPluginShutdown)
	runtimeMu.Lock()
	if pluginRuntime != nil {
		pluginRuntime.Stop()
	}
	pluginRuntime = NewRuntime(realHost{}, defaultDataDir())
	runtimeMu.Unlock()
	return 0
}

//export cliproxyPluginCall
func cliproxyPluginCall(method *C.char, request *C.uint8_t, requestLen C.size_t, response *C.cliproxy_buffer) C.int {
	if response != nil {
		response.ptr = nil
		response.len = 0
	}
	if method == nil {
		writePluginResponse(response, mustMarshal(failure("invalid_method", "method is required")))
		return 1
	}
	requestBytes := []byte("{}")
	if request != nil && requestLen > 0 {
		requestBytes = C.GoBytes(unsafe.Pointer(request), C.int(requestLen))
	}
	result, err := dispatch(C.GoString(method), requestBytes)
	if err != nil {
		writePluginResponse(response, mustMarshal(failure("plugin_error", err.Error())))
		return 1
	}
	writePluginResponse(response, mustMarshal(success(result)))
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
func cliproxyPluginShutdown() {
	runtimeMu.Lock()
	if pluginRuntime != nil {
		pluginRuntime.Stop()
		pluginRuntime = nil
	}
	runtimeMu.Unlock()
}

func main() {}

func dispatch(method string, request []byte) (any, error) {
	switch method {
	case "plugin.register":
		var payload struct {
			ConfigYAML []byte `json:"config_yaml"`
		}
		_ = json.Unmarshal(request, &payload)
		rt := currentRuntime()
		if rt == nil {
			return nil, errors.New("plugin runtime is not initialized")
		}
		if err := rt.Configure(string(payload.ConfigYAML), false); err != nil {
			return nil, fmt.Errorf("invalid plugin configuration: %w", err)
		}
		return registrationPayload(), nil
	case "plugin.reconfigure":
		var payload struct {
			ConfigYAML []byte `json:"config_yaml"`
		}
		if err := json.Unmarshal(request, &payload); err != nil {
			return nil, errors.New("invalid reconfigure request")
		}
		rt := currentRuntime()
		if rt == nil {
			return nil, errors.New("plugin runtime is not initialized")
		}
		if err := rt.Configure(string(payload.ConfigYAML), true); err != nil {
			return nil, fmt.Errorf("invalid plugin configuration: %w", err)
		}
		return registrationPayload(), nil
	case "management.register":
		return managementRegistrationPayloadForID(pluginIDFromManagementRequest(request)), nil
	case "management.handle":
		var req managementRequest
		if err := json.Unmarshal(request, &req); err != nil {
			return nil, errors.New("invalid management request")
		}
		return handleManagement(req), nil
	default:
		return nil, fmt.Errorf("unsupported method %q", method)
	}
}

func registrationPayload() registration {
	return registration{
		SchemaVersion: 1,
		Capabilities:  capabilities{ManagementAPI: true},
		Metadata: registrationMetadata{
			Name:             pluginName,
			Version:          pluginVersion,
			Author:           "Cai Feng",
			GitHubRepository: "https://github.com/hg3386628/codex-health-monitor",
			ConfigFields: []configField{
				{Name: "schedule_mode", Type: "enum", EnumValues: []string{"interval", "daily_times"}, Description: "Scheduling mode"},
				{Name: "interval_min", Type: "integer", Description: "Interval in minutes (5-10080)"},
				{Name: "daily_times", Type: "string", Description: "Comma-separated HH:mm values"},
				{Name: "timezone", Type: "string", Description: "IANA timezone"},
				{Name: "timeout_sec", Type: "integer", Description: "Per-account timeout in seconds"},
				{Name: "target_emails", Type: "string", Description: "Optional comma-separated account emails"},
			},
		},
	}
}

func managementRegistrationPayload() managementRegistration {
	return managementRegistrationPayloadForID(pluginName)
}

func managementRegistrationPayloadForID(pluginID string) managementRegistration {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		pluginID = pluginName
	}
	routePrefix := "/plugins/" + pluginID
	return managementRegistration{
		Routes: []managementRoute{
			{Method: http.MethodGet, Path: routePrefix + "/status"},
			{Method: http.MethodGet, Path: routePrefix + "/accounts"},
			{Method: http.MethodGet, Path: routePrefix + "/history"},
			{Method: http.MethodPost, Path: routePrefix + "/run"},
			{Method: http.MethodGet, Path: routePrefix + "/schedule"},
			{Method: http.MethodPost, Path: routePrefix + "/schedule"},
		},
		Resources: []managementResource{
			{Path: "/panel", Menu: "Codex Health Monitor", Description: "Codex account status, history, and scheduling."},
		},
	}
}

func pluginIDFromManagementRequest(request []byte) string {
	var payload struct {
		ResourceBasePath      string `json:"ResourceBasePath"`
		ResourceBasePathSnake string `json:"resource_base_path"`
		ResourceBasePathCamel string `json:"resourceBasePath"`
	}
	if err := json.Unmarshal(request, &payload); err != nil {
		return pluginName
	}
	basePath := strings.TrimSpace(payload.ResourceBasePath)
	if basePath == "" {
		basePath = strings.TrimSpace(payload.ResourceBasePathSnake)
	}
	if basePath == "" {
		basePath = strings.TrimSpace(payload.ResourceBasePathCamel)
	}
	const prefix = "/v0/resource/plugins/"
	basePath = strings.TrimRight(basePath, "/")
	if !strings.HasPrefix(basePath, prefix) {
		return pluginName
	}
	pluginID := strings.TrimPrefix(basePath, prefix)
	if pluginID == "" || strings.Contains(pluginID, "/") || strings.ContainsAny(pluginID, "?# \t\r\n") {
		return pluginName
	}
	return pluginID
}

func handleManagement(req managementRequest) managementResponse {
	rt := currentRuntime()
	if rt == nil {
		return jsonResponse(http.StatusServiceUnavailable, map[string]any{"error": "plugin runtime is not initialized"})
	}
	path := normalizeManagementPath(req.Path)
	switch {
	case req.Method == http.MethodGet && isPanelPath(path):
		return managementResponse{
			StatusCode: http.StatusOK,
			Headers:    map[string][]string{"Content-Type": {"text/html; charset=utf-8"}, "Cache-Control": {"no-store"}},
			Body:       []byte(panelHTML),
		}
	case req.Method == http.MethodGet && path == "/status":
		return jsonResponse(http.StatusOK, rt.Status())
	case req.Method == http.MethodGet && path == "/accounts":
		accounts, err := rt.Accounts()
		if err != nil {
			return jsonResponse(http.StatusBadGateway, map[string]any{"error": "unable to list Codex accounts"})
		}
		return jsonResponse(http.StatusOK, map[string]any{"accounts": accounts})
	case req.Method == http.MethodGet && path == "/history":
		return jsonResponse(http.StatusOK, map[string]any{"history": rt.History()})
	case req.Method == http.MethodPost && path == "/run":
		var input struct {
			Wait bool `json:"wait"`
		}
		if len(req.Body) > 0 && json.Unmarshal(req.Body, &input) != nil {
			return jsonResponse(http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
		}
		done, err := rt.StartRun("manual")
		if errors.Is(err, ErrRunInProgress) {
			return jsonResponse(http.StatusConflict, map[string]any{"error": "a health check is already running"})
		}
		if err != nil {
			return jsonResponse(http.StatusInternalServerError, map[string]any{"error": "unable to start health check"})
		}
		if input.Wait {
			<-done
			return jsonResponse(http.StatusOK, rt.Status())
		}
		return jsonResponse(http.StatusAccepted, map[string]any{"started": true})
	case req.Method == http.MethodGet && path == "/schedule":
		return jsonResponse(http.StatusOK, rt.ScheduleStatus())
	case req.Method == http.MethodPost && path == "/schedule":
		var schedule ScheduleConfig
		if err := json.Unmarshal(req.Body, &schedule); err != nil {
			return jsonResponse(http.StatusBadRequest, map[string]any{"error": "invalid schedule JSON"})
		}
		if err := rt.UpdateSchedule(schedule); err != nil {
			return jsonResponse(http.StatusBadRequest, map[string]any{"error": err.Error()})
		}
		return jsonResponse(http.StatusOK, rt.ScheduleStatus())
	default:
		return jsonResponse(http.StatusNotFound, map[string]any{"error": "route not found"})
	}
}

// knownManagementPrefixes lists host-side prefixes whose next path segment is
// always the runtime plugin ID. The ID comes from the shared library file name,
// so it is not necessarily pluginName (codex-health-monitor-linux-arm64.so
// yields the ID "codex-health-monitor-linux-arm64"). Dropping that whole
// segment keeps routing correct however the library was named on disk.
var knownManagementPrefixes = []string{
	"/v0/resource/plugins/",
	"/v0/management/plugins/",
	"/plugins/",
	"/v0/management/",
}

// normalizeManagementPath reduces a host-supplied request path to the plugin
// relative path, for example "/status" or "/panel".
func normalizeManagementPath(rawPath string) string {
	path := strings.TrimSpace(rawPath)
	if cut := strings.IndexAny(path, "?#"); cut >= 0 {
		path = path[:cut]
	}
	for _, prefix := range knownManagementPrefixes {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		rest := strings.TrimPrefix(path, prefix)
		if slash := strings.Index(rest, "/"); slash >= 0 {
			path = rest[slash:]
		} else {
			path = "/"
		}
		break
	}
	path = strings.TrimRight(path, "/")
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

// isPanelPath reports whether a normalized path should serve the panel page.
// "/0" covers hosts or UI routes that address the resource by menu index.
func isPanelPath(path string) bool {
	switch path {
	case "/", "/panel", "/index.html", "/0":
		return true
	default:
		return false
	}
}

func currentRuntime() *Runtime {
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()
	return pluginRuntime
}

func success(value any) envelope {
	raw, _ := json.Marshal(value)
	return envelope{OK: true, Result: raw}
}

func failure(code, message string) envelope {
	return envelope{OK: false, Error: &envelopeError{Code: code, Message: message}}
}

func mustMarshal(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		raw = []byte(`{"ok":false,"error":{"code":"encode_error","message":"failed to encode plugin response"}}`)
	}
	return raw
}

func writePluginResponse(response *C.cliproxy_buffer, raw []byte) {
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

func jsonResponse(status int, value any) managementResponse {
	raw, err := json.Marshal(value)
	if err != nil {
		status = http.StatusInternalServerError
		raw = []byte(`{"error":"failed to encode response"}`)
	}
	return managementResponse{
		StatusCode: status,
		Headers:    map[string][]string{"Content-Type": {"application/json; charset=utf-8"}, "Cache-Control": {"no-store"}},
		Body:       raw,
	}
}

func callHost(method string, request any, result any) error {
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	methodC := C.CString(method)
	requestC := C.CString(string(raw))
	defer C.free(unsafe.Pointer(methodC))
	defer C.free(unsafe.Pointer(requestC))
	var responseBuffer C.cliproxy_buffer
	var requestPtr *C.uint8_t
	if len(raw) > 0 {
		requestPtr = (*C.uint8_t)(unsafe.Pointer(requestC))
	}
	callCode := C.call_host_api(methodC, requestPtr, C.size_t(len(raw)), &responseBuffer)
	var responseRaw []byte
	if responseBuffer.ptr != nil && responseBuffer.len > 0 {
		responseRaw = C.GoBytes(responseBuffer.ptr, C.int(responseBuffer.len))
	}
	if responseBuffer.ptr != nil {
		C.free_host_buffer(responseBuffer.ptr, responseBuffer.len)
	}
	if len(responseRaw) == 0 {
		return errors.New("host callback returned no response")
	}
	var response envelope
	if err := json.Unmarshal(responseRaw, &response); err != nil {
		return errors.New("host callback returned invalid JSON")
	}
	if !response.OK {
		if response.Error != nil && response.Error.Message != "" {
			return errors.New(response.Error.Message)
		}
		return errors.New("host callback failed")
	}
	if callCode != 0 {
		return errors.New("host callback returned a failure code")
	}
	if result == nil || len(response.Result) == 0 {
		return nil
	}
	return json.Unmarshal(response.Result, result)
}
