package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccountDiscoveryFailureIsReportedAndPersisted(t *testing.T) {
	host := &fakeHost{listErr: errors.New("host failure containing secret-token")}
	rt := newConfiguredRuntime(t, host)
	record := waitForRun(t, rt)
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if record.ErrorCode != "account_discovery_failed" || record.ErrorMessage == "" {
		t.Fatalf("discovery failure was reported as an empty successful run: %s", raw)
	}
	if record.FinishedAt.IsZero() || record.Total != 0 || len(host.requests) != 0 {
		t.Fatalf("unexpected failed run: %+v", record)
	}
	for _, name := range []string{stateFileName, historyFileName} {
		data, err := os.ReadFile(filepath.Join(rt.dataDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "account_discovery_failed") || strings.Contains(string(data), "secret-token") {
			t.Errorf("%s did not persist a sanitized discovery failure", name)
		}
	}
	host.mu.Lock()
	logs := strings.Join(host.logs, "\n")
	host.mu.Unlock()
	if !strings.Contains(logs, `"level":"error"`) || strings.Contains(logs, "secret-token") {
		t.Fatal("discovery failure must produce a sanitized error log")
	}
	// Recovery must clear the error, including the legitimate empty-list case.
	host.mu.Lock()
	host.listErr = nil
	host.mu.Unlock()
	recovered := waitForRun(t, rt)
	raw, _ = json.Marshal(recovered)
	if strings.Contains(string(raw), "account_discovery_failed") {
		t.Fatalf("recovered run retained the old failure: %s", raw)
	}
}
