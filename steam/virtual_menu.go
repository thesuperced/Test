package steam

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Alia5/SISR/config"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const virtualMenuTargetPrefix = "virtualmenus_uid"

type closeTargetRequest struct {
	ID     int                    `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// SetVirtualMenuVisibility hides or restores Steam's virtual-menu CEF targets
// without closing them. Steam reuses these targets when an action set with a
// virtual menu becomes active again.
func SetVirtualMenuVisibility(ctx context.Context, cfg *config.Steam, visible bool) error {
	tabs, err := GetCEFTabs(ctx, cfg)
	if err != nil {
		return err
	}
	return setVirtualMenuTabsVisibility(ctx, tabs, visible)
}

func setVirtualMenuTabsVisibility(ctx context.Context, tabs []TabInfo, visible bool) error {
	var visibilityErrs []error
	for _, tab := range tabs {
		if !isVirtualMenuTab(tab) {
			continue
		}
		if err := setCEFTargetVisibility(ctx, tab.WebSocketDebuggerURL, visible); err != nil {
			visibilityErrs = append(visibilityErrs, fmt.Errorf("set virtual menu target visibility: %w", err))
		}
	}
	return errors.Join(visibilityErrs...)
}

func isVirtualMenuTab(tab TabInfo) bool {
	return strings.HasPrefix(strings.ToLower(tab.Title), virtualMenuTargetPrefix)
}

func setCEFTargetVisibility(ctx context.Context, debuggerURL string, visible bool) error {
	u, err := url.Parse(debuggerURL)
	if err != nil {
		return err
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return fmt.Errorf("unexpected debugger URL scheme %q", u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return fmt.Errorf("refusing non-loopback debugger host %q", host)
	}

	conn, _, err := websocket.Dial(ctx, debuggerURL, &websocket.DialOptions{HTTPHeader: http.Header{}})
	if err != nil {
		return err
	}
	defer conn.CloseNow() //nolint:errcheck
	expression := "document.documentElement.style.setProperty('visibility','hidden','important')"
	if visible {
		expression = "document.documentElement.style.removeProperty('visibility')"
	}
	return wsjson.Write(ctx, conn, closeTargetRequest{
		ID:     1,
		Method: "Runtime.evaluate",
		Params: map[string]interface{}{"expression": expression},
	})
}
