package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

const completedOK = "data: {\"type\":\"response.created\"}\n\n" +
	"data: {\"type\":\"response.output_text.delta\",\"delta\":\"O\"}\n\n" +
	"data: {\"type\":\"response.output_text.delta\",\"delta\":\"K\"}\n\n" +
	"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n" +
	"data: [DONE]\n\n"

type fakeHost struct {
	mu           sync.Mutex
	files        []AuthFile
	auth         map[string]json.RawMessage
	requests     []HostHTTPRequest
	logs         []string
	status       int
	body         []byte
	httpErr      error
	listErr      error
	block        chan struct{}
	requestReady chan struct{}
}

func (h *fakeHost) ListAuthFiles(context.Context) ([]AuthFile, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]AuthFile(nil), h.files...), h.listErr
}

func (h *fakeHost) GetAuth(_ context.Context, authIndex string) (json.RawMessage, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	raw, ok := h.auth[authIndex]
	if !ok {
		return nil, errors.New("not found")
	}
	return append(json.RawMessage(nil), raw...), nil
}

func (h *fakeHost) HTTPDo(ctx context.Context, request HostHTTPRequest) (HostHTTPResponse, error) {
	h.mu.Lock()
	h.requests = append(h.requests, request)
	ready := h.requestReady
	block := h.block
	status := h.status
	body := append([]byte(nil), h.body...)
	httpErr := h.httpErr
	h.mu.Unlock()
	if ready != nil {
		select {
		case ready <- struct{}{}:
		default:
		}
	}
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return HostHTTPResponse{}, ctx.Err()
		}
	}
	if httpErr != nil {
		return HostHTTPResponse{}, httpErr
	}
	if status == 0 {
		status = 200
	}
	if body == nil {
		body = []byte(completedOK)
	}
	return HostHTTPResponse{StatusCode: status, Body: body}, nil
}

func (h *fakeHost) Log(_ context.Context, level, message string, fields map[string]any) {
	raw, _ := json.Marshal(map[string]any{"level": level, "message": message, "fields": fields})
	h.mu.Lock()
	h.logs = append(h.logs, string(raw))
	h.mu.Unlock()
}

func authJSON(index int, accountID string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"access_token":"secret-token-%d","account_id":%q,"email":"user-%d@example.com"}`, index, accountID, index))
}

func newConfiguredRuntime(t *testing.T, host Host) *Runtime {
	t.Helper()
	runtime := NewRuntime(host, t.TempDir())
	if err := runtime.Configure("", false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Stop)
	return runtime
}

func waitForRun(t *testing.T, runtime *Runtime) RunRecord {
	t.Helper()
	done, err := runtime.StartRun("manual")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("health check did not complete")
	}
	history := runtime.History()
	if len(history) == 0 {
		t.Fatal("health check did not produce history")
	}
	return history[0]
}

func TestRunFiltersCodexAndPinsEveryAuthIndex(t *testing.T) {
	host := &fakeHost{
		files: []AuthFile{
			{AuthIndex: "idx-9", Type: "anthropic", Provider: "codex", Email: "excluded@example.com"},
			{AuthIndex: "idx-3", Type: "codex", Email: "three@example.com"},
			{AuthIndex: "idx-1", Type: "codex", Email: "one@example.com"},
			{AuthIndex: "idx-2", Type: "codex", Email: "two@example.com"},
		},
		auth: map[string]json.RawMessage{
			"idx-1": authJSON(1, "shared-account"),
			"idx-2": authJSON(2, "shared-account"),
			"idx-3": authJSON(3, "different-account"),
		},
	}
	runtime := newConfiguredRuntime(t, host)
	record := waitForRun(t, runtime)
	if record.Total != 3 || record.Healthy != 3 || record.Unhealthy != 0 {
		t.Fatalf("unexpected totals: %+v", record)
	}
	host.mu.Lock()
	requests := append([]HostHTTPRequest(nil), host.requests...)
	host.mu.Unlock()
	if len(requests) != 3 {
		t.Fatalf("got %d requests, want 3", len(requests))
	}
	var tokens []string
	for _, request := range requests {
		if request.URL != probeURL {
			t.Errorf("unexpected URL %q", request.URL)
		}
		tokens = append(tokens, request.Headers["Authorization"][0])
		var payload map[string]any
		if err := json.Unmarshal(request.Body, &payload); err != nil {
			t.Fatal(err)
		}
		if payload["model"] != probeModel {
			t.Errorf("model = %v, want %s", payload["model"], probeModel)
		}
		if payload["stream"] != true {
			t.Error("stream must be true")
		}
	}
	sort.Strings(tokens)
	wantTokens := []string{"Bearer secret-token-1", "Bearer secret-token-2", "Bearer secret-token-3"}
	for index := range wantTokens {
		if tokens[index] != wantTokens[index] {
			t.Fatalf("tokens = %v, want %v", tokens, wantTokens)
		}
	}
	if record.Accounts[0].AccountID == "" || record.Accounts[1].AccountID == "" {
		t.Fatal("account IDs should be reported")
	}
}

func TestParseCompletedResponse(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{name: "deltas with terminal event", body: completedOK, want: "OK"},
		{name: "terminal response output", body: `data: {"type":"response.completed","response":{"output":[{"type":"message","content":[{"type":"output_text","text":"OK"}]}]}}` + "\n\n", want: "OK"},
		{name: "duplicate done representations", body: `data: {"type":"response.output_text.delta","delta":"OK"}` + "\n\n" +
			`data: {"type":"response.output_text.done","text":"OK"}` + "\n\n" +
			`data: {"type":"response.output_item.done","item":{"type":"message","content":[{"type":"output_text","text":"OK"}]}}` + "\n\n" +
			`data: {"type":"response.completed","response":{"status":"completed"}}` + "\n\n", want: "OK"},
		{name: "single JSON terminal response", body: `{"type":"response.completed","response":{"output":[{"content":[{"type":"output_text","text":"OK"}]}]}}`, want: "OK"},
		{name: "missing terminal event", body: `data: {"type":"response.output_text.delta","delta":"OK"}` + "\n\n", wantErr: true},
		{name: "missing output", body: `data: {"type":"response.completed","response":{"status":"completed"}}` + "\n\n", wantErr: true},
		{name: "failed event", body: `data: {"type":"response.failed"}` + "\n\n" + `data: {"type":"response.completed"}` + "\n\n", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseCompletedResponse([]byte(test.body))
			if test.wantErr && err == nil {
				t.Fatalf("expected an error, got %q", got)
			}
			if !test.wantErr && (err != nil || got != test.want) {
				t.Fatalf("got %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestUnexpectedOutputIsResponseError(t *testing.T) {
	host := &fakeHost{
		files: []AuthFile{{AuthIndex: "idx-1", Type: "codex", Email: "one@example.com"}},
		auth:  map[string]json.RawMessage{"idx-1": authJSON(1, "account")},
		body: []byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"NO\"}\n\n" +
			"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n"),
	}
	record := waitForRun(t, newConfiguredRuntime(t, host))
	if record.Accounts[0].ErrorCode != "unexpected_output" || record.Accounts[0].Healthy {
		t.Fatalf("unexpected result: %+v", record.Accounts[0])
	}
}

func TestHTTPFailureClassification(t *testing.T) {
	tests := []struct {
		status int
		code   string
	}{
		{401, "unauthorized"},
		{402, "payment_required"},
		{403, "forbidden"},
		{429, "rate_limited"},
		{500, "upstream_error"},
		{503, "upstream_error"},
	}
	for _, test := range tests {
		t.Run(fmt.Sprint(test.status), func(t *testing.T) {
			host := &fakeHost{
				files:  []AuthFile{{AuthIndex: "idx-1", Type: "codex"}},
				auth:   map[string]json.RawMessage{"idx-1": authJSON(1, "account")},
				status: test.status,
			}
			record := waitForRun(t, newConfiguredRuntime(t, host))
			result := record.Accounts[0]
			if result.ErrorCode != test.code || result.HTTPStatus != test.status || result.Healthy {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func TestNetworkAndTimeoutClassification(t *testing.T) {
	networkHost := &fakeHost{
		files:   []AuthFile{{AuthIndex: "idx-1", Type: "codex"}},
		auth:    map[string]json.RawMessage{"idx-1": authJSON(1, "account")},
		httpErr: errors.New("dial failed"),
	}
	network := waitForRun(t, newConfiguredRuntime(t, networkHost)).Accounts[0]
	if network.ErrorCode != "network_error" {
		t.Fatalf("network result: %+v", network)
	}

	timeoutHost := &fakeHost{
		files: []AuthFile{{AuthIndex: "idx-1", Type: "codex"}},
		auth:  map[string]json.RawMessage{"idx-1": authJSON(1, "account")},
		block: make(chan struct{}),
	}
	runtime := newConfiguredRuntime(t, timeoutHost)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	result := runtime.probeAccount(ctx, timeoutHost.files[0], 30)
	if result.ErrorCode != "timeout" {
		t.Fatalf("timeout result: %+v", result)
	}
}

func TestSecretsDoNotEnterPersistenceOrLogs(t *testing.T) {
	dir := t.TempDir()
	host := &fakeHost{
		files: []AuthFile{{AuthIndex: "idx-7", Type: "codex", Email: "safe@example.com"}},
		auth:  map[string]json.RawMessage{"idx-7": authJSON(7, "safe-account")},
	}
	runtime := NewRuntime(host, dir)
	if err := runtime.Configure("", false); err != nil {
		t.Fatal(err)
	}
	defer runtime.Stop()
	waitForRun(t, runtime)
	var combined strings.Builder
	for _, name := range []string{stateFileName, historyFileName} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		combined.Write(raw)
	}
	host.mu.Lock()
	combined.WriteString(strings.Join(host.logs, "\n"))
	host.mu.Unlock()
	text := combined.String()
	for _, forbidden := range []string{"secret-token-7", "Authorization", "access_token", "refresh_token", "id_token"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("persisted data or logs contain forbidden value %q", forbidden)
		}
	}
}

func TestScheduleCalculationAndNormalization(t *testing.T) {
	now := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	interval := defaultSchedule()
	interval.IntervalMin = 45
	next, err := nextRunAfter(interval, now)
	if err != nil {
		t.Fatal(err)
	}
	base := now.Add(45 * time.Minute)
	if next.Before(base) || next.After(base.Add(jitterMaxMinutes*time.Minute)) {
		t.Fatalf("interval next = %v, want within [%v, %v]", next, base, base.Add(jitterMaxMinutes*time.Minute))
	}

	daily := defaultSchedule()
	daily.Mode = "daily_times"
	daily.DailyTimes = "18:00,09:00,09:00,13:00"
	normalized, err := normalizeSchedule(daily)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.DailyTimes != "09:00,13:00,18:00" {
		t.Fatalf("daily_times = %q", normalized.DailyTimes)
	}
	next, err = nextRunAfter(normalized, now)
	if err != nil {
		t.Fatal(err)
	}
	location, _ := time.LoadLocation("Asia/Shanghai")
	want := time.Date(2026, 8, 27, 9, 0, 0, 0, location)
	if !next.Equal(want) {
		t.Fatalf("daily next = %v, want %v", next, want)
	}
}

func TestIntervalScheduleAddsRandomJitter(t *testing.T) {
	now := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	schedule := defaultSchedule()
	schedule.IntervalMin = 30
	base := now.Add(30 * time.Minute)
	seen := make(map[time.Duration]bool)
	for i := 0; i < 100; i++ {
		next, err := nextRunAfter(schedule, now)
		if err != nil {
			t.Fatal(err)
		}
		offset := next.Sub(base)
		if offset < 0 || offset > jitterMaxMinutes*time.Minute {
			t.Fatalf("next = %v, jitter offset = %v, want within [0, %v]", next, offset, jitterMaxMinutes*time.Minute)
		}
		seen[offset] = true
	}
	if len(seen) < 10 {
		t.Fatalf("jitter produced only %d distinct offsets across 100 runs, expected meaningful randomization", len(seen))
	}

	// daily_times mode keeps exact wall-clock times and must not jitter.
	daily := defaultSchedule()
	daily.Mode = "daily_times"
	daily.DailyTimes = "09:00"
	location, _ := time.LoadLocation(daily.Timezone)
	at := time.Date(2026, 8, 27, 8, 0, 0, 0, location)
	want := time.Date(2026, 8, 27, 9, 0, 0, 0, location)
	for i := 0; i < 10; i++ {
		next, err := nextRunAfter(daily, at)
		if err != nil {
			t.Fatal(err)
		}
		if !next.Equal(want) {
			t.Fatalf("daily next = %v, want %v (daily_times must not jitter)", next, want)
		}
	}
}

func TestDailyScheduleMovesToNextDayAndRejectsTooManyTimes(t *testing.T) {
	schedule := defaultSchedule()
	schedule.Mode = "daily_times"
	schedule.DailyTimes = "09:00,13:00"
	location, _ := time.LoadLocation(schedule.Timezone)
	now := time.Date(2026, 8, 27, 14, 0, 0, 0, location)
	next, err := nextRunAfter(schedule, now)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 8, 28, 9, 0, 0, 0, location)
	if !next.Equal(want) {
		t.Fatalf("next = %v, want %v", next, want)
	}
	schedule.DailyTimes = "00:00,01:00,02:00,03:00,04:00,05:00,06:00,07:00,08:00,09:00,10:00,11:00,12:00"
	if _, err := normalizeSchedule(schedule); err == nil {
		t.Fatal("expected an error for more than 12 daily times")
	}
}

func TestSingleFlightRejectsOverlappingRuns(t *testing.T) {
	host := &fakeHost{
		files:        []AuthFile{{AuthIndex: "idx-1", Type: "codex"}},
		auth:         map[string]json.RawMessage{"idx-1": authJSON(1, "account")},
		block:        make(chan struct{}),
		requestReady: make(chan struct{}, 1),
	}
	runtime := newConfiguredRuntime(t, host)
	done, err := runtime.StartRun("manual")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-host.requestReady:
	case <-time.After(time.Second):
		t.Fatal("first run did not reach HTTP request")
	}
	if _, err := runtime.StartRun("scheduled"); !errors.Is(err, ErrRunInProgress) {
		t.Fatalf("overlapping run error = %v", err)
	}
	close(host.block)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("first run did not finish")
	}
}

func TestHistoryIsCappedAtOneHundredRuns(t *testing.T) {
	host := &fakeHost{}
	runtime := newConfiguredRuntime(t, host)
	for index := 0; index < 105; index++ {
		done, err := runtime.StartRun("manual")
		if err != nil {
			t.Fatal(err)
		}
		<-done
	}
	if got := len(runtime.History()); got != maxHistory {
		t.Fatalf("history length = %d, want %d", got, maxHistory)
	}
}

func setAuthFileDisabled(t *testing.T, host *fakeHost, authIndex string, disabled bool) {
	t.Helper()
	host.mu.Lock()
	defer host.mu.Unlock()
	for index := range host.files {
		if host.files[index].AuthIndex == authIndex {
			host.files[index].Disabled = disabled
		}
	}
}

func viewByAuthIndex(views []AccountView, authIndex string) AccountView {
	for _, view := range views {
		if view.AuthIndex == authIndex {
			return view
		}
	}
	return AccountView{}
}

func TestAccountsViewReconcilesDisabledState(t *testing.T) {
	host := &fakeHost{
		files: []AuthFile{
			{AuthIndex: "idx-1", Type: "codex", Email: "one@example.com"},
			{AuthIndex: "idx-2", Type: "codex", Email: "two@example.com", Disabled: true},
		},
		auth: map[string]json.RawMessage{
			"idx-1": authJSON(1, "account-1"),
			"idx-2": authJSON(2, "account-2"),
		},
	}
	runtime := newConfiguredRuntime(t, host)
	record := waitForRun(t, runtime)
	if record.Total != 2 || record.Healthy != 1 || record.Unhealthy != 1 {
		t.Fatalf("unexpected run record: %+v", record)
	}

	// Re-enable idx-2 in CPA: the stale disabled result from the last run must
	// be retracted without waiting for the next scheduled check.
	setAuthFileDisabled(t, host, "idx-2", false)
	views, err := runtime.Accounts()
	if err != nil {
		t.Fatal(err)
	}
	if view := viewByAuthIndex(views, "idx-2"); view.Status != "not_checked" || view.Healthy || !view.CheckedAt.IsZero() {
		t.Fatalf("re-enabled view = %+v, want not_checked with retracted result", view)
	}
	if view := viewByAuthIndex(views, "idx-1"); view.Status != "healthy" || !view.Healthy {
		t.Fatalf("healthy view = %+v", view)
	}

	// Disabling an account flips its view to disabled immediately, even when
	// the last run reported it healthy.
	setAuthFileDisabled(t, host, "idx-1", true)
	views, err = runtime.Accounts()
	if err != nil {
		t.Fatal(err)
	}
	if view := viewByAuthIndex(views, "idx-1"); view.Status != "disabled" || view.Healthy || view.ErrorCode != "credential_disabled" {
		t.Fatalf("disabled view = %+v", view)
	}

	// After re-enabling and running again both accounts reflect current state.
	setAuthFileDisabled(t, host, "idx-1", false)
	record = waitForRun(t, runtime)
	if record.Healthy != 2 {
		t.Fatalf("expected both accounts healthy after re-enabling, got %+v", record)
	}
	views, err = runtime.Accounts()
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range views {
		if view.Status != "healthy" || !view.Healthy {
			t.Fatalf("view = %+v, want healthy", view)
		}
	}
}

func TestAccountsViewUnavailableIsProbedNotDisabled(t *testing.T) {
	// CPA's "unavailable" flag is transient (quota cooldown, post-restart
	// token loading, ...). It must NOT short-circuit the probe the way a
	// manually-disabled credential does; the health monitor should still
	// verify the credential independently and show the real result.
	host := &fakeHost{
		files: []AuthFile{
			{AuthIndex: "idx-1", Type: "codex", Email: "cool@example.com", Unavailable: true},
		},
		auth: map[string]json.RawMessage{
			"idx-1": authJSON(1, "account-1"),
		},
	}
	runtime := newConfiguredRuntime(t, host)
	record := waitForRun(t, runtime)
	if record.Total != 1 || record.Healthy != 1 || record.Unhealthy != 0 {
		t.Fatalf("unavailable credential should still be probed: %+v", record)
	}
	if got := record.Accounts[0]; got.Status != "healthy" || !got.Healthy || got.ErrorCode != "" {
		t.Fatalf("run result = %+v, want healthy with no error", got)
	}

	views, err := runtime.Accounts()
	if err != nil {
		t.Fatal(err)
	}
	view := viewByAuthIndex(views, "idx-1")
	if view.Status != "healthy" || !view.Healthy || !view.Unavailable || view.Disabled {
		t.Fatalf("view = %+v, want healthy with Unavailable flag set and not disabled", view)
	}
}

func TestConcurrentScheduleUpdatesAreSafe(t *testing.T) {
	host := &fakeHost{}
	runtime := newConfiguredRuntime(t, host)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			schedule := defaultSchedule()
			schedule.IntervalMin = 30 + (n % 10)
			if err := runtime.UpdateSchedule(schedule); err != nil {
				t.Errorf("UpdateSchedule failed: %v", err)
			}
		}(i)
	}
	wg.Wait()
	runtime.mu.RLock()
	next := runtime.state.NextRunAt
	runtime.mu.RUnlock()
	if next.IsZero() {
		t.Fatal("NextRunAt should be set after schedule updates")
	}
	// Stop must not hang even after concurrent restarts; a second Stop is a no-op.
	runtime.Stop()
	runtime.Stop()
}

func TestUnchangedReconfigurePreservesScheduledRun(t *testing.T) {
	previousJitter := intervalJitter
	intervalJitter = func() time.Duration { return 0 }
	t.Cleanup(func() { intervalJitter = previousJitter })

	runtime := NewRuntime(&fakeHost{}, t.TempDir())
	t.Cleanup(runtime.Stop)
	configYAML := `plugins:
  configs:
    codex-health-monitor:
      schedule_mode: interval
      interval_min: 30
      timezone: Asia/Shanghai
      timeout_sec: 30
`
	if err := runtime.Configure(configYAML, false); err != nil {
		t.Fatal(err)
	}

	var first time.Time
	deadline := time.Now().Add(time.Second)
	for first.IsZero() && time.Now().Before(deadline) {
		runtime.mu.RLock()
		first = runtime.state.NextRunAt
		runtime.mu.RUnlock()
		if first.IsZero() {
			time.Sleep(time.Millisecond)
		}
	}
	if first.IsZero() {
		t.Fatal("initial scheduler did not set NextRunAt")
	}

	if err := runtime.Configure(configYAML, true); err != nil {
		t.Fatal(err)
	}
	runtime.mu.RLock()
	second := runtime.state.NextRunAt
	runtime.mu.RUnlock()
	if !second.Equal(first) {
		t.Fatalf("unchanged reconfigure moved NextRunAt from %v to %v", first, second)
	}
}

func TestManagementRegistrationAndRoutes(t *testing.T) {
	registration := managementRegistrationPayload()
	if len(registration.Routes) != 6 || len(registration.Resources) != 1 {
		t.Fatalf("unexpected registration: %+v", registration)
	}
	if registration.Resources[0].Path != "/panel" || !strings.Contains(panelHTML, "gpt-5.6-luna") || !strings.Contains(panelHTML, "enc::v1::") {
		t.Fatal("panel resource is missing or incomplete")
	}
	for input, want := range map[string]string{
		"/plugins/codex-health-monitor/status":               "/status",
		"/v0/management/plugins/codex-health-monitor/status": "/status",
		"/v0/resource/plugins/codex-health-monitor/panel":    "/panel",
		// The runtime plugin ID comes from the library file name, so it can
		// carry a platform suffix. Routing must not depend on it.
		"/v0/resource/plugins/codex-health-monitor-linux-arm64/panel":    "/panel",
		"/v0/management/plugins/codex-health-monitor-linux-arm64/status": "/status",
		"/v0/resource/plugins/codex-health-monitor-v0.1.6/panel":         "/panel",
		"/v0/resource/plugins/codex-health-monitor/panel/":               "/panel",
		"/v0/resource/plugins/codex-health-monitor/panel?foo=bar":        "/panel",
		"history/": "/history",
	} {
		if got := normalizeManagementPath(input); got != want {
			t.Errorf("normalizeManagementPath(%q) = %q, want %q", input, got, want)
		}
	}

	host := &fakeHost{}
	runtime := newConfiguredRuntime(t, host)
	runtimeMu.Lock()
	previous := pluginRuntime
	pluginRuntime = runtime
	runtimeMu.Unlock()
	t.Cleanup(func() {
		runtimeMu.Lock()
		pluginRuntime = previous
		runtimeMu.Unlock()
	})
	response := handleManagement(managementRequest{Method: "GET", Path: "/plugins/codex-health-monitor/status"})
	if response.StatusCode != 200 || !strings.Contains(string(response.Body), probeModel) {
		t.Fatalf("unexpected status response: %+v", response)
	}
	response = handleManagement(managementRequest{Method: "GET", Path: "/plugins/codex-health-monitor/missing"})
	if response.StatusCode != 404 {
		t.Fatalf("missing route status = %d", response.StatusCode)
	}
	response = handleManagement(managementRequest{Method: "GET", Path: "/v0/resource/plugins/codex-health-monitor/panel"})
	if response.StatusCode != 200 || !strings.Contains(string(response.Body), "gpt-5.6-luna") {
		t.Fatalf("unexpected panel response: status=%d", response.StatusCode)
	}
	// A library installed under a platform-suffixed name must still serve the panel.
	response = handleManagement(managementRequest{Method: "GET", Path: "/v0/resource/plugins/codex-health-monitor-linux-arm64/panel"})
	if response.StatusCode != 200 || !strings.Contains(string(response.Body), "gpt-5.6-luna") {
		t.Fatalf("unexpected panel response for suffixed plugin id: status=%d", response.StatusCode)
	}
	response = handleManagement(managementRequest{Method: "GET", Path: "/v0/management/plugins/codex-health-monitor-linux-arm64/status"})
	if response.StatusCode != 200 || !strings.Contains(string(response.Body), probeModel) {
		t.Fatalf("unexpected status response for suffixed plugin id: status=%d", response.StatusCode)
	}
}

func TestManagementRegistrationUsesHostPluginID(t *testing.T) {
	for _, request := range []string{
		`{"ResourceBasePath":"/v0/resource/plugins/codex-health-monitor-linux-amd64"}`,
		`{"resource_base_path":"/v0/resource/plugins/codex-health-monitor-linux-amd64"}`,
		`{"resourceBasePath":"/v0/resource/plugins/codex-health-monitor-linux-amd64/"}`,
	} {
		value, err := dispatch("management.register", []byte(request))
		if err != nil {
			t.Fatalf("dispatch management.register failed: %v", err)
		}
		registration, ok := value.(managementRegistration)
		if !ok || len(registration.Routes) == 0 {
			t.Fatalf("unexpected registration value: %#v", value)
		}
		if got, want := registration.Routes[0].Path, "/plugins/codex-health-monitor-linux-amd64/status"; got != want {
			t.Fatalf("route path = %q, want %q", got, want)
		}
	}
	value, err := dispatch("management.register", []byte(`{"ResourceBasePath":"/invalid"}`))
	if err != nil {
		t.Fatal(err)
	}
	registration := value.(managementRegistration)
	if registration.Routes[0].Path != "/plugins/codex-health-monitor/status" {
		t.Fatalf("invalid resource base path should use the stable plugin ID: %+v", registration.Routes[0])
	}
}

func TestYAMLConfigurationParsing(t *testing.T) {
	raw := `plugins:
  enabled: true
  configs:
    another-plugin:
      schedule_mode: daily_times
    codex-health-monitor:
      enabled: true
      schedule_mode: daily_times
      interval_min: 60
      daily_times: "18:00,09:00"
      timezone: Asia/Shanghai
      timeout_sec: 25
      target_emails: "Two@example.com,one@example.com"
`
	parsed, found, err := parsePluginConfig(raw, defaultSchedule())
	if err != nil || !found {
		t.Fatalf("parse failed: found=%v err=%v", found, err)
	}
	normalized, err := normalizeSchedule(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Mode != "daily_times" || normalized.DailyTimes != "09:00,18:00" || normalized.TimeoutSec != 25 {
		t.Fatalf("unexpected parsed config: %+v", normalized)
	}
	if normalized.TargetEmails != "one@example.com,two@example.com" {
		t.Fatalf("target emails = %q", normalized.TargetEmails)
	}
}
