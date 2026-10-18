package steam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestIsVirtualMenuTab(t *testing.T) {
	for _, tc := range []struct {
		title string
		want  bool
	}{
		{"virtualmenus_uid123", true},
		{"VirtualMenus_UID456", true},
		{"Steam Input On-screen Keyboard", false},
		{"SharedJSContext", false},
		{"not_virtualmenus_uid123", false},
	} {
		if got := isVirtualMenuTab(TabInfo{Title: tc.title}); got != tc.want {
			t.Errorf("title %q: got %v, want %v", tc.title, got, tc.want)
		}
	}
}

func TestCloseVirtualMenuTabs(t *testing.T) {
	requests := make(chan closeTargetRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept websocket: %v", err)
			return
		}
		defer conn.CloseNow() //nolint:errcheck
		var request closeTargetRequest
		if err := wsjson.Read(r.Context(), conn, &request); err != nil {
			t.Errorf("read close request: %v", err)
			return
		}
		requests <- request
	}))
	defer server.Close()

	debuggerURL := "ws" + strings.TrimPrefix(server.URL, "http")
	tabs := []TabInfo{
		{Title: "SharedJSContext", WebSocketDebuggerURL: "ws://example.com/ignored"},
		{Title: "virtualmenus_uid123", WebSocketDebuggerURL: debuggerURL},
	}
	if err := closeVirtualMenuTabs(context.Background(), tabs); err != nil {
		t.Fatal(err)
	}
	select {
	case request := <-requests:
		if request.ID != 1 || request.Method != "Page.close" {
			t.Fatalf("unexpected request: %+v", request)
		}
	case <-time.After(time.Second):
		t.Fatal("virtual menu target was not closed")
	}
}

func TestCloseCEFTargetRejectsRemoteHost(t *testing.T) {
	err := closeCEFTarget(context.Background(), "ws://example.com/devtools/page/1")
	if err == nil || !strings.Contains(err.Error(), "non-loopback") {
		t.Fatalf("got %v, want non-loopback error", err)
	}
}
