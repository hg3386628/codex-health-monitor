package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"
)

const (
	probeModel      = "gpt-5.6-luna"
	probeURL        = "https://chatgpt.com/backend-api/codex/responses"
	maxHistory      = 100
	stateFileName   = "state.json"
	historyFileName = "history.json"

	// jitterMaxMinutes is the largest random delay, in minutes, added on top
	// of the configured interval so scheduled checks do not repeat at an
	// exactly fixed period.
	jitterMaxMinutes = 5
)

// intervalJitter returns a random duration in [0, jitterMaxMinutes] minutes.
// It is a variable so tests can pin it if needed.
var intervalJitter = func() time.Duration {
	return time.Duration(rand.IntN(jitterMaxMinutes*60+1)) * time.Second
}

var ErrRunInProgress = errors.New("health check already running")

type Host interface {
	ListAuthFiles(context.Context) ([]AuthFile, error)
	GetAuth(context.Context, string) (json.RawMessage, error)
	HTTPDo(context.Context, HostHTTPRequest) (HostHTTPResponse, error)
	Log(context.Context, string, string, map[string]any)
}

type realHost struct{}

type AuthFile struct {
	ID          string `json:"id"`
	AuthIndex   string `json:"auth_index"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Type        string `json:"type"`
	Email       string `json:"email"`
	Disabled    bool   `json:"disabled"`
	Unavailable bool   `json:"unavailable"`
}

type HostHTTPRequest struct {
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Headers map[string][]string `json:"headers,omitempty"`
	Body    []byte              `json:"body,omitempty"`
}

type HostHTTPResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
}

func (r *HostHTTPResponse) UnmarshalJSON(data []byte) error {
	var raw struct {
		StatusCode      int                 `json:"StatusCode"`
		StatusCodeSnake int                 `json:"status_code"`
		Headers         map[string][]string `json:"Headers"`
		HeadersLower    map[string][]string `json:"headers"`
		Body            []byte              `json:"Body"`
		BodyLower       []byte              `json:"body"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.StatusCode = raw.StatusCode
	if r.StatusCode == 0 {
		r.StatusCode = raw.StatusCodeSnake
	}
	r.Headers = raw.Headers
	if r.Headers == nil {
		r.Headers = raw.HeadersLower
	}
	r.Body = raw.Body
	if r.Body == nil {
		r.Body = raw.BodyLower
	}
	return nil
}

func (realHost) ListAuthFiles(ctx context.Context) ([]AuthFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var response struct {
		Files []AuthFile `json:"files"`
	}
	if err := callHost("host.auth.list", map[string]any{}, &response); err != nil {
		return nil, err
	}
	return response.Files, nil
}

func (realHost) GetAuth(ctx context.Context, authIndex string) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var response struct {
		JSON json.RawMessage `json:"json"`
		Auth json.RawMessage `json:"auth"`
		Data json.RawMessage `json:"data"`
	}
	if err := callHost("host.auth.get", map[string]any{"auth_index": authIndex}, &response); err != nil {
		return nil, err
	}
	if len(response.JSON) > 0 {
		return response.JSON, nil
	}
	if len(response.Auth) > 0 {
		return response.Auth, nil
	}
	if len(response.Data) > 0 {
		return response.Data, nil
	}
	return nil, errors.New("empty auth document")
}

func (realHost) HTTPDo(ctx context.Context, request HostHTTPRequest) (HostHTTPResponse, error) {
	if err := ctx.Err(); err != nil {
		return HostHTTPResponse{}, err
	}
	var response HostHTTPResponse
	if err := callHost("host.http.do", request, &response); err != nil {
		return HostHTTPResponse{}, err
	}
	return response, nil
}

func (realHost) Log(ctx context.Context, level, message string, fields map[string]any) {
	if ctx.Err() != nil {
		return
	}
	_ = callHost("host.log", map[string]any{"level": level, "message": message, "fields": fields}, nil)
}

type ScheduleConfig struct {
	Enabled      bool   `json:"enabled"`
	Mode         string `json:"schedule_mode"`
	IntervalMin  int    `json:"interval_min"`
	DailyTimes   string `json:"daily_times"`
	Timezone     string `json:"timezone"`
	TimeoutSec   int    `json:"timeout_sec"`
	TargetEmails string `json:"target_emails"`
}

type AccountResult struct {
	Email        string    `json:"email"`
	AuthIndex    string    `json:"auth_index"`
	AccountID    string    `json:"account_id"`
	Model        string    `json:"model"`
	Status       string    `json:"status"`
	Healthy      bool      `json:"healthy"`
	HTTPStatus   int       `json:"http_status"`
	LatencyMS    int64     `json:"latency_ms"`
	CheckedAt    time.Time `json:"checked_at"`
	ErrorCode    string    `json:"error_code,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

type AccountView struct {
	Email        string    `json:"email"`
	AuthIndex    string    `json:"auth_index"`
	AccountID    string    `json:"account_id,omitempty"`
	Status       string    `json:"status"`
	Healthy      bool      `json:"healthy"`
	HTTPStatus   int       `json:"http_status"`
	LatencyMS    int64     `json:"latency_ms"`
	CheckedAt    time.Time `json:"checked_at,omitempty"`
	ErrorCode    string    `json:"error_code,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	Disabled     bool      `json:"disabled"`
	Unavailable  bool      `json:"unavailable"`
}

type RunRecord struct {
	ID           string          `json:"id"`
	Trigger      string          `json:"trigger"`
	StartedAt    time.Time       `json:"started_at"`
	FinishedAt   time.Time       `json:"finished_at"`
	Total        int             `json:"total"`
	Healthy      int             `json:"healthy"`
	Unhealthy    int             `json:"unhealthy"`
	ErrorCode    string          `json:"error_code,omitempty"`
	ErrorMessage string          `json:"error_message,omitempty"`
	Accounts     []AccountResult `json:"accounts"`
}

type persistedState struct {
	Version       int            `json:"version"`
	Schedule      ScheduleConfig `json:"schedule"`
	Latest        *RunRecord     `json:"latest,omitempty"`
	NextRunAt     time.Time      `json:"next_run_at,omitempty"`
	Running       bool           `json:"running"`
	ScheduleError string         `json:"schedule_error,omitempty"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type Runtime struct {
	host    Host
	dataDir string

	mu           sync.RWMutex
	persistMu    sync.Mutex
	schedMu      sync.Mutex
	state        persistedState
	history      []RunRecord
	running      bool
	runCancel    context.CancelFunc
	workerCancel context.CancelFunc
	workerDone   chan struct{}
	configured   bool
	stateLoaded  bool
}

func NewRuntime(host Host, dataDir string) *Runtime {
	r := &Runtime{
		host:    host,
		dataDir: dataDir,
		state: persistedState{
			Version:  1,
			Schedule: defaultSchedule(),
		},
	}
	r.load()
	return r
}

func defaultSchedule() ScheduleConfig {
	return ScheduleConfig{
		Enabled:     true,
		Mode:        "interval",
		IntervalMin: 30,
		Timezone:    "Asia/Shanghai",
		TimeoutSec:  30,
	}
}

func defaultDataDir() string {
	if value := strings.TrimSpace(os.Getenv("CODEX_HEALTH_MONITOR_DATA_DIR")); value != "" {
		return value
	}
	if info, err := os.Stat("/CLIProxyAPI/plugins"); err == nil && info.IsDir() {
		return "/CLIProxyAPI/plugins/codex-health-monitor"
	}
	return filepath.Join("plugins", "codex-health-monitor")
}

func (r *Runtime) Configure(configYAML string, force bool) error {
	r.mu.Lock()
	base := r.state.Schedule
	shouldApply := force || !r.stateLoaded
	r.mu.Unlock()

	if shouldApply && strings.TrimSpace(configYAML) != "" {
		parsed, found, err := parsePluginConfig(configYAML, base)
		if err != nil {
			return err
		}
		if found {
			base = parsed
		}
	}
	normalized, err := normalizeSchedule(base)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.state.Schedule = normalized
	r.configured = true
	r.state.UpdatedAt = time.Now().UTC()
	r.mu.Unlock()
	r.restartScheduler()
	r.persistState()
	return nil
}

func (r *Runtime) UpdateSchedule(input ScheduleConfig) error {
	normalized, err := normalizeSchedule(input)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.state.Schedule = normalized
	r.state.ScheduleError = ""
	r.state.UpdatedAt = time.Now().UTC()
	r.mu.Unlock()
	r.restartScheduler()
	r.persistState()
	return nil
}

func normalizeSchedule(input ScheduleConfig) (ScheduleConfig, error) {
	input.Enabled = true
	input.Mode = strings.TrimSpace(input.Mode)
	if input.Mode == "" {
		input.Mode = "interval"
	}
	if input.Mode != "interval" && input.Mode != "daily_times" {
		return ScheduleConfig{}, errors.New("schedule_mode must be interval or daily_times")
	}
	if input.IntervalMin == 0 {
		input.IntervalMin = 30
	}
	if input.IntervalMin < 5 || input.IntervalMin > 10080 {
		return ScheduleConfig{}, errors.New("interval_min must be between 5 and 10080")
	}
	if input.Timezone == "" {
		input.Timezone = "Asia/Shanghai"
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return ScheduleConfig{}, errors.New("timezone must be a valid IANA timezone")
	}
	if input.TimeoutSec == 0 {
		input.TimeoutSec = 30
	}
	if input.TimeoutSec < 5 || input.TimeoutSec > 120 {
		return ScheduleConfig{}, errors.New("timeout_sec must be between 5 and 120")
	}
	times, err := parseDailyTimes(input.DailyTimes)
	if err != nil {
		return ScheduleConfig{}, err
	}
	if input.Mode == "daily_times" && len(times) == 0 {
		return ScheduleConfig{}, errors.New("daily_times is required in daily_times mode")
	}
	input.DailyTimes = strings.Join(times, ",")
	input.TargetEmails = normalizeEmailList(input.TargetEmails)
	return input, nil
}

func parseDailyTimes(value string) ([]string, error) {
	seen := make(map[string]struct{})
	var result []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parsed, err := time.Parse("15:04", part)
		if err != nil || parsed.Format("15:04") != part {
			return nil, fmt.Errorf("invalid daily time %q; use HH:mm", part)
		}
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		result = append(result, part)
	}
	if len(result) > 12 {
		return nil, errors.New("daily_times supports at most 12 unique times")
	}
	sort.Strings(result)
	return result, nil
}

func normalizeEmailList(value string) string {
	seen := make(map[string]struct{})
	var emails []string
	for _, item := range strings.Split(value, ",") {
		email := strings.ToLower(strings.TrimSpace(item))
		if email == "" {
			continue
		}
		if _, exists := seen[email]; exists {
			continue
		}
		seen[email] = struct{}{}
		emails = append(emails, email)
	}
	sort.Strings(emails)
	return strings.Join(emails, ",")
}

func parsePluginConfig(raw string, base ScheduleConfig) (ScheduleConfig, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "{") {
		return parseJSONPluginConfig([]byte(trimmed), base)
	}
	known := map[string]func(string) error{
		"schedule_mode": func(v string) error { base.Mode = cleanYAMLScalar(v); return nil },
		"interval_min": func(v string) error {
			n, err := strconv.Atoi(cleanYAMLScalar(v))
			base.IntervalMin = n
			return err
		},
		"daily_times": func(v string) error { base.DailyTimes = cleanYAMLScalar(v); return nil },
		"timezone":    func(v string) error { base.Timezone = cleanYAMLScalar(v); return nil },
		"timeout_sec": func(v string) error {
			n, err := strconv.Atoi(cleanYAMLScalar(v))
			base.TimeoutSec = n
			return err
		},
		"target_emails": func(v string) error { base.TargetEmails = cleanYAMLScalar(v); return nil },
	}
	lines := strings.Split(raw, "\n")
	sectionIndent := -1
	foundSection := false
	foundField := false
	for _, line := range lines {
		plain := stripYAMLComment(line)
		if strings.TrimSpace(plain) == "" {
			continue
		}
		indent := len(plain) - len(strings.TrimLeft(plain, " \t"))
		key, value, ok := strings.Cut(strings.TrimSpace(plain), ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == pluginName {
			foundSection = true
			sectionIndent = indent
			continue
		}
		if foundSection && indent <= sectionIndent {
			foundSection = false
		}
		setter, knownField := known[key]
		if !knownField || (!foundSection && sectionIndent >= 0) {
			continue
		}
		if err := setter(value); err != nil {
			return ScheduleConfig{}, false, fmt.Errorf("invalid %s", key)
		}
		foundField = true
	}
	return base, foundField, nil
}

func parseJSONPluginConfig(raw []byte, base ScheduleConfig) (ScheduleConfig, bool, error) {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return ScheduleConfig{}, false, err
	}
	candidate := any(root)
	if plugins, ok := root["plugins"].(map[string]any); ok {
		if configs, ok := plugins["configs"].(map[string]any); ok {
			if section, exists := configs[pluginName]; exists {
				candidate = section
			}
		}
	}
	section, ok := candidate.(map[string]any)
	if !ok {
		return base, false, nil
	}
	rawSection, _ := json.Marshal(section)
	var patch struct {
		Mode         *string `json:"schedule_mode"`
		IntervalMin  *int    `json:"interval_min"`
		DailyTimes   *string `json:"daily_times"`
		Timezone     *string `json:"timezone"`
		TimeoutSec   *int    `json:"timeout_sec"`
		TargetEmails *string `json:"target_emails"`
	}
	if err := json.Unmarshal(rawSection, &patch); err != nil {
		return ScheduleConfig{}, false, err
	}
	found := false
	if patch.Mode != nil {
		base.Mode = *patch.Mode
		found = true
	}
	if patch.IntervalMin != nil {
		base.IntervalMin = *patch.IntervalMin
		found = true
	}
	if patch.DailyTimes != nil {
		base.DailyTimes = *patch.DailyTimes
		found = true
	}
	if patch.Timezone != nil {
		base.Timezone = *patch.Timezone
		found = true
	}
	if patch.TimeoutSec != nil {
		base.TimeoutSec = *patch.TimeoutSec
		found = true
	}
	if patch.TargetEmails != nil {
		base.TargetEmails = *patch.TargetEmails
		found = true
	}
	return base, found, nil
}

func stripYAMLComment(line string) string {
	var quote rune
	for index, char := range line {
		if (char == '\'' || char == '"') && quote == 0 {
			quote = char
			continue
		}
		if char == quote {
			quote = 0
			continue
		}
		if char == '#' && quote == 0 {
			return line[:index]
		}
	}
	return line
}

func cleanYAMLScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		return value[1 : len(value)-1]
	}
	return value
}

func nextRunAfter(schedule ScheduleConfig, now time.Time) (time.Time, error) {
	location, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		return time.Time{}, err
	}
	if schedule.Mode == "interval" {
		return now.Add(time.Duration(schedule.IntervalMin)*time.Minute + intervalJitter()), nil
	}
	times, err := parseDailyTimes(schedule.DailyTimes)
	if err != nil {
		return time.Time{}, err
	}
	localNow := now.In(location)
	for dayOffset := 0; dayOffset <= 1; dayOffset++ {
		day := localNow.AddDate(0, 0, dayOffset)
		for _, value := range times {
			clock, _ := time.Parse("15:04", value)
			candidate := time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), 0, 0, location)
			if candidate.After(localNow) {
				return candidate, nil
			}
		}
	}
	return time.Time{}, errors.New("unable to calculate next daily run")
}

func (r *Runtime) restartScheduler() {
	// schedMu serializes restart attempts so overlapping
	// reconfigurations cannot orphan a scheduler goroutine: each
	// caller observes and tears down the previous worker before
	// installing its own.
	r.schedMu.Lock()
	defer r.schedMu.Unlock()
	r.mu.Lock()
	oldCancel := r.workerCancel
	oldDone := r.workerDone
	r.workerCancel = nil
	r.workerDone = nil
	schedule := r.state.Schedule
	configured := r.configured
	r.mu.Unlock()
	if oldCancel != nil {
		oldCancel()
		if oldDone != nil {
			<-oldDone
		}
	}
	if !configured || !schedule.Enabled {
		r.mu.Lock()
		r.state.NextRunAt = time.Time{}
		r.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	r.mu.Lock()
	r.workerCancel = cancel
	r.workerDone = done
	r.mu.Unlock()
	go r.schedulerLoop(ctx, done)
}

func (r *Runtime) schedulerLoop(ctx context.Context, done chan struct{}) {
	defer close(done)
	for {
		r.mu.RLock()
		schedule := r.state.Schedule
		r.mu.RUnlock()
		next, err := nextRunAfter(schedule, time.Now())
		r.mu.Lock()
		if err != nil {
			r.state.ScheduleError = "unable to calculate next run"
			r.state.NextRunAt = time.Time{}
		} else {
			r.state.ScheduleError = ""
			r.state.NextRunAt = next.UTC()
		}
		r.state.UpdatedAt = time.Now().UTC()
		r.mu.Unlock()
		r.persistState()
		if err != nil {
			return
		}
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
			if _, runErr := r.StartRun("scheduled"); runErr != nil && !errors.Is(runErr, ErrRunInProgress) {
				r.host.Log(context.Background(), "error", "scheduled Codex health check could not start", map[string]any{"error_code": "start_failed"})
			}
		}
	}
}

func (r *Runtime) StartRun(trigger string) (<-chan struct{}, error) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return nil, ErrRunInProgress
	}
	r.running = true
	r.state.Running = true
	r.state.UpdatedAt = time.Now().UTC()
	ctx, cancel := context.WithCancel(context.Background())
	r.runCancel = cancel
	done := make(chan struct{})
	r.mu.Unlock()
	r.persistState()
	go func() {
		defer close(done)
		record := r.executeRun(ctx, trigger)
		r.mu.Lock()
		r.running = false
		r.runCancel = nil
		r.state.Running = false
		r.state.Latest = &record
		r.state.UpdatedAt = time.Now().UTC()
		r.history = append(r.history, record)
		if len(r.history) > maxHistory {
			r.history = append([]RunRecord(nil), r.history[len(r.history)-maxHistory:]...)
		}
		r.mu.Unlock()
		r.persistAll()
		if record.ErrorCode != "" {
			r.host.Log(context.Background(), "error", "Codex health check could not discover credentials", map[string]any{"error_code": record.ErrorCode})
		}
		r.host.Log(context.Background(), "info", "Codex health check completed", map[string]any{
			"run_id": record.ID, "trigger": trigger, "total": record.Total, "healthy": record.Healthy, "unhealthy": record.Unhealthy,
		})
	}()
	return done, nil
}

func (r *Runtime) executeRun(ctx context.Context, trigger string) RunRecord {
	started := time.Now().UTC()
	record := RunRecord{ID: fmt.Sprintf("%d", started.UnixNano()), Trigger: trigger, StartedAt: started}
	r.mu.RLock()
	schedule := r.state.Schedule
	r.mu.RUnlock()
	files, err := r.discoverAccounts(ctx, schedule.TargetEmails)
	if err != nil {
		record.ErrorCode = "account_discovery_failed"
		// Keep host details out of persisted state and logs; only expose a stable
		// actionable message to the management panel.
		record.ErrorMessage = "CPA could not list Codex credentials."
		record.FinishedAt = time.Now().UTC()
		return record
	}
	results := make([]AccountResult, len(files))
	var wg sync.WaitGroup
	for index, file := range files {
		wg.Add(1)
		go func(i int, account AuthFile) {
			defer wg.Done()
			results[i] = r.probeAccount(ctx, account, schedule.TimeoutSec)
		}(index, file)
	}
	wg.Wait()
	record.Accounts = results
	record.Total = len(results)
	for _, result := range results {
		if result.Healthy {
			record.Healthy++
		} else {
			record.Unhealthy++
		}
	}
	record.FinishedAt = time.Now().UTC()
	return record
}

func isCodexAuth(file AuthFile) bool {
	return strings.EqualFold(strings.TrimSpace(file.Type), "codex")
}

func (r *Runtime) discoverAccounts(ctx context.Context, targetEmails string) ([]AuthFile, error) {
	files, err := r.host.ListAuthFiles(ctx)
	if err != nil {
		return nil, err
	}
	targets := make(map[string]struct{})
	for _, email := range strings.Split(normalizeEmailList(targetEmails), ",") {
		if email != "" {
			targets[email] = struct{}{}
		}
	}
	var result []AuthFile
	for _, file := range files {
		if !isCodexAuth(file) {
			continue
		}
		if len(targets) > 0 {
			if _, ok := targets[strings.ToLower(strings.TrimSpace(file.Email))]; !ok {
				continue
			}
		}
		result = append(result, file)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Email == result[j].Email {
			return result[i].AuthIndex < result[j].AuthIndex
		}
		return result[i].Email < result[j].Email
	})
	return result, nil
}

type authMaterial struct {
	AccessToken string `json:"access_token"`
	AccountID   string `json:"account_id"`
	Email       string `json:"email"`
}

func (r *Runtime) probeAccount(parent context.Context, account AuthFile, timeoutSec int) AccountResult {
	started := time.Now()
	result := AccountResult{
		Email: account.Email, AuthIndex: account.AuthIndex, Model: probeModel, Status: "checking", CheckedAt: started.UTC(),
	}
	finish := func(status, code, message string, healthy bool, httpStatus int) AccountResult {
		result.Status = status
		result.Healthy = healthy
		result.HTTPStatus = httpStatus
		result.LatencyMS = time.Since(started).Milliseconds()
		result.ErrorCode = code
		result.ErrorMessage = message
		return result
	}
	// Only an explicitly disabled credential is skipped. CPA's "unavailable"
	// flag marks transient states (quota cooldown, token not loaded after a
	// restart, etc.) and must NOT short-circuit the probe: the whole point of
	// a health monitor is to verify the credential independently of CPA's
	// routing/cooldown state.
	if account.Disabled {
		return finish("disabled", "credential_disabled", "Credential is disabled in CPA.", false, 0)
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(timeoutSec)*time.Second)
	defer cancel()
	rawAuth, err := r.host.GetAuth(ctx, account.AuthIndex)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return finish("timeout", "timeout", "The account check timed out.", false, 0)
		}
		return finish("credential_error", "credential_read_failed", "CPA could not read this credential.", false, 0)
	}
	var material authMaterial
	if json.Unmarshal(rawAuth, &material) != nil || strings.TrimSpace(material.AccessToken) == "" {
		return finish("credential_error", "credential_invalid", "The credential does not contain a usable access token.", false, 0)
	}
	result.AccountID = material.AccountID
	if result.Email == "" {
		result.Email = material.Email
	}
	body, _ := json.Marshal(map[string]any{
		"model":        probeModel,
		"instructions": "Return exactly OK.",
		"input": []any{map[string]any{
			"type": "message", "role": "user",
			"content": []any{map[string]any{"type": "input_text", "text": "Reply with exactly OK"}},
		}},
		"stream":              true,
		"store":               false,
		"parallel_tool_calls": true,
		"include":             []string{"reasoning.encrypted_content"},
		"reasoning":           map[string]string{"effort": "low"},
	})
	request := HostHTTPRequest{
		Method: http.MethodPost,
		URL:    probeURL,
		Headers: map[string][]string{
			"Authorization":      {"Bearer " + material.AccessToken},
			"Content-Type":       {"application/json"},
			"Accept":             {"text/event-stream"},
			"Originator":         {"codex-tui"},
			"Chatgpt-Account-Id": {material.AccountID},
			"User-Agent":         {fmt.Sprintf("codex-tui/0.146.0 (Linux; %s)", runtime.GOARCH)},
			"Connection":         {"keep-alive"},
		},
		Body: body,
	}
	type responseResult struct {
		response HostHTTPResponse
		err      error
	}
	responseCh := make(chan responseResult, 1)
	go func() {
		response, requestErr := r.host.HTTPDo(ctx, request)
		responseCh <- responseResult{response: response, err: requestErr}
	}()
	var response HostHTTPResponse
	select {
	case <-ctx.Done():
		return finish("timeout", "timeout", "The account check timed out.", false, 0)
	case outcome := <-responseCh:
		if outcome.err != nil {
			if errors.Is(outcome.err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return finish("timeout", "timeout", "The account check timed out.", false, 0)
			}
			return finish("network_error", "network_error", "CPA could not reach the Codex upstream service.", false, 0)
		}
		response = outcome.response
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		status, code, message := classifyHTTPStatus(response.StatusCode)
		return finish(status, code, message, false, response.StatusCode)
	}
	output, err := parseCompletedResponse(response.Body)
	if err != nil {
		return finish("response_error", "response_format_error", "The upstream response did not contain a valid completed result.", false, response.StatusCode)
	}
	if strings.TrimSpace(output) != "OK" {
		return finish("response_error", "unexpected_output", "The completed response did not return the expected OK output.", false, response.StatusCode)
	}
	return finish("healthy", "", "", true, response.StatusCode)
}

func classifyHTTPStatus(status int) (string, string, string) {
	switch status {
	case http.StatusUnauthorized:
		return "unauthorized", "unauthorized", "The Codex credential was not authorized."
	case http.StatusPaymentRequired:
		return "payment_required", "payment_required", "The account quota or subscription requires attention."
	case http.StatusForbidden:
		return "forbidden", "forbidden", "The Codex account was refused by the upstream service."
	case http.StatusTooManyRequests:
		return "rate_limited", "rate_limited", "The Codex account is currently rate limited."
	}
	if status >= 500 {
		return "upstream_error", "upstream_error", "The Codex upstream service returned a server error."
	}
	return "request_error", "http_error", fmt.Sprintf("The Codex upstream service returned HTTP %d.", status)
}

func parseCompletedResponse(body []byte) (string, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return "", errors.New("empty response")
	}
	if trimmed[0] == '{' {
		var event map[string]any
		if json.Unmarshal(trimmed, &event) != nil {
			return "", errors.New("invalid response JSON")
		}
		if eventType(event) != "response.completed" {
			return "", errors.New("missing completed event")
		}
		text := completedEventText(event)
		if text == "" {
			return "", errors.New("missing output text")
		}
		return text, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var delta strings.Builder
	outputTextDone := ""
	outputItemDone := ""
	terminalText := ""
	completed := false
	failed := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return "", errors.New("invalid SSE event")
		}
		switch eventType(event) {
		case "response.output_text.delta":
			if value, ok := event["delta"].(string); ok {
				delta.WriteString(value)
			}
		case "response.output_text.done":
			if value, ok := event["text"].(string); ok {
				outputTextDone = value
			}
		case "response.output_item.done", "response.content_part.done":
			if text := extractOutputText(event); text != "" {
				outputItemDone = text
			}
		case "response.completed":
			completed = true
			terminalText = completedEventText(event)
		case "response.failed", "response.incomplete", "error":
			failed = true
		}
	}
	if err := scanner.Err(); err != nil {
		return "", errors.New("unable to read SSE response")
	}
	if failed || !completed {
		return "", errors.New("response did not complete")
	}
	output := terminalText
	if output == "" {
		output = outputTextDone
	}
	if output == "" {
		output = outputItemDone
	}
	if output == "" {
		output = delta.String()
	}
	if output == "" {
		return "", errors.New("missing output text")
	}
	return output, nil
}

func eventType(event map[string]any) string {
	value, _ := event["type"].(string)
	return value
}

func completedEventText(event map[string]any) string {
	if response, ok := event["response"].(map[string]any); ok {
		return extractOutputText(response)
	}
	return extractOutputText(event)
}

func extractOutputText(value any) string {
	var result strings.Builder
	var walk func(any)
	walk = func(current any) {
		switch item := current.(type) {
		case []any:
			for _, child := range item {
				walk(child)
			}
		case map[string]any:
			itemType, _ := item["type"].(string)
			if itemType == "output_text" {
				if text, ok := item["text"].(string); ok {
					result.WriteString(text)
					return
				}
			}
			for _, key := range []string{"output", "item", "content", "part"} {
				if child, exists := item[key]; exists {
					walk(child)
				}
			}
		}
	}
	walk(value)
	return result.String()
}

func (r *Runtime) Status() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	total, healthy, unhealthy := 0, 0, 0
	if r.state.Latest != nil {
		total = r.state.Latest.Total
		healthy = r.state.Latest.Healthy
		unhealthy = r.state.Latest.Unhealthy
	}
	return map[string]any{
		"name": pluginName, "version": pluginVersion, "model": probeModel,
		"running": r.running, "total": total, "healthy": healthy, "unhealthy": unhealthy,
		"latest": r.state.Latest, "schedule": r.state.Schedule, "next_run_at": r.state.NextRunAt,
		"schedule_error": r.state.ScheduleError,
	}
}

func (r *Runtime) ScheduleStatus() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return map[string]any{"schedule": r.state.Schedule, "next_run_at": r.state.NextRunAt, "schedule_error": r.state.ScheduleError}
}

func (r *Runtime) History() []RunRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := append([]RunRecord(nil), r.history...)
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}

func (r *Runtime) Accounts() ([]AccountView, error) {
	r.mu.RLock()
	targets := r.state.Schedule.TargetEmails
	var latest *RunRecord
	if r.state.Latest != nil {
		copyLatest := *r.state.Latest
		latest = &copyLatest
	}
	r.mu.RUnlock()
	files, err := r.discoverAccounts(context.Background(), targets)
	if err != nil {
		return nil, err
	}
	latestByIndex := make(map[string]AccountResult)
	if latest != nil {
		for _, result := range latest.Accounts {
			latestByIndex[result.AuthIndex] = result
		}
	}
	views := make([]AccountView, 0, len(files))
	for _, file := range files {
		// Disabled reflects ONLY an operator-manually-disabled credential
		// (auth file's "disabled": true). CPA's runtime "unavailable" flag is
		// transient (quota cooldown, post-restart token loading, etc.) and is
		// exposed separately as informational metadata; it never overrides the
		// real probe result shown to the user.
		view := AccountView{Email: file.Email, AuthIndex: file.AuthIndex, Status: "not_checked", Disabled: file.Disabled, Unavailable: file.Unavailable}
		if result, ok := latestByIndex[file.AuthIndex]; ok {
			view.AccountID = result.AccountID
			view.Status = result.Status
			view.Healthy = result.Healthy
			view.HTTPStatus = result.HTTPStatus
			view.LatencyMS = result.LatencyMS
			view.CheckedAt = result.CheckedAt
			view.ErrorCode = result.ErrorCode
			view.ErrorMessage = result.ErrorMessage
		}
		if view.Disabled {
			// The credential is manually disabled in CPA right now, so reflect
			// that immediately instead of a possibly stale check result.
			view.Status = "disabled"
			view.Healthy = false
			view.HTTPStatus = 0
			view.LatencyMS = 0
			view.ErrorCode = "credential_disabled"
			view.ErrorMessage = "Credential is disabled in CPA."
		} else if view.Status == "disabled" {
			// The last check ran while the credential was disabled, but it has
			// been re-enabled since. That result no longer reflects reality,
			// so retract it until the next run probes the credential again.
			view.Status = "not_checked"
			view.Healthy = false
			view.HTTPStatus = 0
			view.LatencyMS = 0
			view.CheckedAt = time.Time{}
			view.ErrorCode = ""
			view.ErrorMessage = ""
		}
		views = append(views, view)
	}
	return views, nil
}

func (r *Runtime) Stop() {
	r.mu.Lock()
	workerCancel := r.workerCancel
	workerDone := r.workerDone
	r.workerCancel = nil
	r.workerDone = nil
	runCancel := r.runCancel
	r.mu.Unlock()
	if workerCancel != nil {
		workerCancel()
		if workerDone != nil {
			<-workerDone
		}
	}
	if runCancel != nil {
		runCancel()
	}
}

func (r *Runtime) load() {
	if raw, err := os.ReadFile(filepath.Join(r.dataDir, stateFileName)); err == nil {
		var state persistedState
		if jsonErr := json.Unmarshal(raw, &state); jsonErr != nil {
			r.host.Log(context.Background(), "warn", "Codex health state could not be parsed, using defaults", map[string]any{"error_code": "state_parse_failed"})
		} else if normalized, normalizeErr := normalizeSchedule(state.Schedule); normalizeErr != nil {
			r.host.Log(context.Background(), "warn", "Codex health schedule in state is invalid, using defaults", map[string]any{"error_code": "state_schedule_invalid"})
		} else {
			state.Schedule = normalized
			state.Running = false
			r.state = state
			r.stateLoaded = true
		}
	}
	if raw, err := os.ReadFile(filepath.Join(r.dataDir, historyFileName)); err == nil {
		var history []RunRecord
		if jsonErr := json.Unmarshal(raw, &history); jsonErr != nil {
			r.host.Log(context.Background(), "warn", "Codex health history could not be parsed, starting empty", map[string]any{"error_code": "history_parse_failed"})
		} else {
			if len(history) > maxHistory {
				history = history[len(history)-maxHistory:]
			}
			r.history = history
		}
	}
}

func (r *Runtime) persistAll() {
	r.persistMu.Lock()
	defer r.persistMu.Unlock()
	r.mu.RLock()
	state := r.state
	state.Running = r.running
	history := append([]RunRecord(nil), r.history...)
	r.mu.RUnlock()
	if err := writeJSONAtomic(filepath.Join(r.dataDir, stateFileName), state); err != nil {
		r.host.Log(context.Background(), "error", "Codex health state could not be persisted", map[string]any{"error_code": "state_write_failed"})
	}
	if err := writeJSONAtomic(filepath.Join(r.dataDir, historyFileName), history); err != nil {
		r.host.Log(context.Background(), "error", "Codex health history could not be persisted", map[string]any{"error_code": "history_write_failed"})
	}
}

func (r *Runtime) persistState() {
	r.persistMu.Lock()
	defer r.persistMu.Unlock()
	r.mu.RLock()
	state := r.state
	state.Running = r.running
	r.mu.RUnlock()
	if err := writeJSONAtomic(filepath.Join(r.dataDir, stateFileName), state); err != nil {
		r.host.Log(context.Background(), "error", "Codex health state could not be persisted", map[string]any{"error_code": "state_write_failed"})
	}
}

func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(raw); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}
