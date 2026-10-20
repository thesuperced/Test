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

	"github.com/Alia5/SISR/config"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestNativeKeyboardRoutingLeavesKeyboardTargetAlone(t *testing.T) {
	for _, open := range []bool{true, false} {
		t.Run(strconv.FormatBool(open), func(t *testing.T) {
			var address string
			requests := make(chan cdpRequest, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/json" {
					fmt.Fprintf(w, `[{"title":"SharedJSContext","webSocketDebuggerUrl":"%s/shared"},{"title":"Steam Input On-screen Keyboard","webSocketDebuggerUrl":"%s/keyboard"}]`, address, address)
					return
				}
				if r.URL.Path != "/shared" {
					t.Error("touched native keyboard")
					http.Error(w, "wrong target", 500)
					return
				}
				ws, err := websocket.Accept(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer ws.CloseNow()
				var request cdpRequest
				if err := wsjson.Read(r.Context(), ws, &request); err != nil {
					t.Error(err)
					return
				}
				requests <- request
				_ = wsjson.Write(r.Context(), ws, map[string]any{"id": 1, "result": map[string]any{}})
			}))
			defer server.Close()
			address = "ws" + strings.TrimPrefix(server.URL, "http")
			_, p, _ := net.SplitHostPort(server.Listener.Addr().String())
			port, _ := strconv.Atoi(p)
			if err := RouteNativeKeyboard(context.Background(), &config.Steam{CEFRemoteDebugPort: uint16(port)}, "15506786540112887808", open); err != nil {
				t.Fatal(err)
			}
			req := <-requests
			expression := req.Params["expression"].(string)
			if req.Method != "Runtime.evaluate" || !strings.Contains(expression, "15506786540112887808") {
				t.Fatal(req)
			}
			if strings.Contains(expression, "SetOverlayState(") != open {
				t.Fatal(expression)
			}
		})
	}
}

func TestKeyboardRoutingRejectsInvalidTargets(t *testing.T) {
	if err := RouteNativeKeyboard(context.Background(), nil, `";bad()`, true); err == nil {
		t.Fatal("invalid game ID accepted")
	}
	if err := evaluateTarget(context.Background(), "ws://example.com/x", "anything"); err == nil {
		t.Fatal("remote target accepted")
	}
}
