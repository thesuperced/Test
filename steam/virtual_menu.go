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
	ID     int    `json:"id"`
	Method string `json:"method"`
}

// CloseVirtualMenuTabs closes only Steam's virtual-menu CEF targets. Steam
// creates new targets when a menu is activated again.
func CloseVirtualMenuTabs(ctx context.Context, cfg *config.Steam) error {
	tabs, err := GetCEFTabs(ctx, cfg)
	if err != nil {
		return err
	}
	return closeVirtualMenuTabs(ctx, tabs)
}

func closeVirtualMenuTabs(ctx context.Context, tabs []TabInfo) error {
	var closeErrs []error
	for _, tab := range tabs {
		if !isVirtualMenuTab(tab) {
			continue
		}
		if err := closeCEFTarget(ctx, tab.WebSocketDebuggerURL); err != nil {
			closeErrs = append(closeErrs, fmt.Errorf("close virtual menu target: %w", err))
		}
	}
	return errors.Join(closeErrs...)
}

func isVirtualMenuTab(tab TabInfo) bool {
	return strings.HasPrefix(strings.ToLower(tab.Title), virtualMenuTargetPrefix)
}

func closeCEFTarget(ctx context.Context, debuggerURL string) error {
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
	return wsjson.Write(ctx, conn, closeTargetRequest{ID: 1, Method: "Page.close"})
}
