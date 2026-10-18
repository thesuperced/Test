package steam

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Alia5/SISR/config"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestKeyboardTargetPresent(t *testing.T) {
	for _, tc := range []struct {
		title string
		want  bool
	}{
		{"Steam Input On-screen Keyboard", true},
		{"virtualmenus_uid123", false},
		{"Steam Overlay", false},
		{"Steam Input On-screen Keyboard settings", false},
		{"", false},
	} {
		if got := keyboardTargetPresent([]TabInfo{{Title: tc.title}}); got != tc.want {
			t.Errorf("title=%q: %v", tc.title, got)
		}
	}
	if keyboardTargetPresent(nil) {
		t.Fatal("empty target list identified as keyboard")
	}
}

func TestCloseDuplicateKeyboardTargets(t *testing.T) {
	requests := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept websocket: %v", err)
			return
		}
		defer conn.CloseNow() //nolint:errcheck
		var request struct {
			Method string `json:"method"`
		}
		if err := wsjson.Read(r.Context(), conn, &request); err != nil {
			t.Errorf("read close request: %v", err)
			return
		}
		requests <- request.Method
	}))
	defer server.Close()

	debuggerURL := "ws" + strings.TrimPrefix(server.URL, "http")
	tabs := []TabInfo{
		{ID: "keep", Title: keyboardTargetTitle, WebSocketDebuggerURL: "ws://example.com/must-not-close"},
		{ID: "duplicate", Title: keyboardTargetTitle, WebSocketDebuggerURL: debuggerURL},
	}
	if err := closeDuplicateKeyboardTargets(context.Background(), tabs); err != nil {
		t.Fatal(err)
	}
	select {
	case method := <-requests:
		if method != "Page.close" {
			t.Fatalf("method=%q, want Page.close", method)
		}
	case <-time.After(time.Second):
		t.Fatal("duplicate keyboard target was not closed")
	}
}

func TestCloseKeyboardTargetRejectsRemoteHost(t *testing.T) {
	err := closeKeyboardTarget(context.Background(), "ws://example.com/devtools/page/1")
	if err == nil || !strings.Contains(err.Error(), "non-loopback") {
		t.Fatalf("got %v, want non-loopback error", err)
	}
}

func TestKeyboardObserverConfiguredPort(t *testing.T) {
	body := `[{"title":"Steam Input On-screen Keyboard"}]`
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json" {
			t.Errorf("path %q", r.URL.Path)
		}
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
	}))
	defer server.Close()
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	portNum, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	observer := NewKeyboardObserver(config.Steam{CEFRemoteDebugPort: uint16(portNum)})
	ctx := context.Background()
	if open, err := observer.IsOpen(ctx); !open || err != nil {
		t.Fatalf("open=%v err=%v", open, err)
	}
	body = `[{"title":"virtualmenus_uid123"}]`
	if open, err := observer.IsOpen(ctx); open || err != nil {
		t.Fatalf("open=%v err=%v", open, err)
	}
	status = http.StatusServiceUnavailable
	observer.nextDiscovery = time.Now().Add(-time.Second)
	if _, err := observer.IsOpen(ctx); err == nil {
		t.Fatal("HTTP failure was treated as a close")
	}
	if observer.port != 0 {
		t.Fatal("failed endpoint was not invalidated for rediscovery")
	}
	status = http.StatusOK
	body = `invalid JSON`
	if _, err := observer.IsOpen(ctx); err == nil {
		t.Fatal("malformed JSON was treated as a close")
	}
	body = `[{"title":"SharedJSContext"}]`
	if open, err := observer.IsOpen(ctx); open || err != nil {
		t.Fatalf("open=%v err=%v", open, err)
	}
	body = `[]`
	if _, err := observer.IsOpen(ctx); err == nil {
		t.Fatal("empty Steam target list was treated as a close")
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := observer.IsOpen(cancelCtx); err == nil {
		t.Fatal("cancelled request succeeded")
	}
}
