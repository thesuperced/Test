package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var diagnosticLog struct {
	sync.Mutex
	file  *os.File
	bytes int
}

const diagnosticLimit = 8 * 1024 * 1024

// StartDiagnostics uses a separate, bounded log: ordinary input logs, HTTP
// bodies, page contents, and keyboard text never enter this file.
func StartDiagnostics() (string, func(), error) {
	exe, err := os.Executable()
	if err != nil {
		return "", func() {}, err
	}
	dirs := []string{filepath.Join(filepath.Dir(exe), "SISR-diagnostics")}
	if cache, err := os.UserCacheDir(); err == nil {
		dirs = append(dirs, filepath.Join(cache, "SISR", "diagnostics"))
	}
	var f *os.File
	for _, dir := range dirs {
		if err = os.MkdirAll(dir, 0700); err != nil {
			continue
		}
		name := fmt.Sprintf("SISR-%s-%d.jsonl", time.Now().UTC().Format("20060102-150405.000"), os.Getpid())
		f, err = os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			break
		}
	}
	if err != nil {
		return "", func() {}, err
	}
	diagnosticLog.Lock()
	diagnosticLog.file, diagnosticLog.bytes = f, 0
	diagnosticLog.Unlock()
	return f.Name(), func() {
		Diagnostic("session_end", nil)
		diagnosticLog.Lock()
		defer diagnosticLog.Unlock()
		diagnosticLog.file = nil
		_ = f.Close()
	}, nil
}

func Diagnostic(event string, fields map[string]any) {
	diagnosticLog.Lock()
	defer diagnosticLog.Unlock()
	if diagnosticLog.file == nil || diagnosticLog.bytes >= diagnosticLimit {
		return
	}
	record := struct {
		Time   string         `json:"time"`
		Event  string         `json:"event"`
		Fields map[string]any `json:"fields,omitempty"`
	}{time.Now().UTC().Format(time.RFC3339Nano), event, fields}
	data, err := json.Marshal(record)
	if err != nil {
		return
	}
	if diagnosticLog.bytes+len(data)+1 >= diagnosticLimit {
		data = []byte(`{"event":"log_limit_reached","message":"Restart SISR for a new recording"}`)
		diagnosticLog.bytes = diagnosticLimit
	}
	n, _ := diagnosticLog.file.Write(append(data, '\n'))
	diagnosticLog.bytes += n
}
