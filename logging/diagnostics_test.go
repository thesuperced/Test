package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestDiagnosticConcurrentRecordsAndLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "diagnostic.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	diagnosticLog.file, diagnosticLog.bytes = f, 0
	defer func() { diagnosticLog.file = nil; _ = f.Close() }()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); Diagnostic("test", map[string]any{"open": true}) }()
	}
	wg.Wait()
	diagnosticLog.bytes = diagnosticLimit - 1
	Diagnostic("at_limit", nil)
	Diagnostic("must_not_be_written", nil)
	read, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	decoder := json.NewDecoder(read)
	for i := 0; i < 21; i++ {
		var record map[string]any
		if err := decoder.Decode(&record); err != nil {
			t.Fatal(err)
		}
		if i == 20 && record["event"] != "log_limit_reached" {
			t.Fatal(record)
		}
	}
	var extra any
	if decoder.Decode(&extra) == nil {
		t.Fatal("wrote beyond cap")
	}
}
