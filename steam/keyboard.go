package steam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Alia5/SISR/config"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const keyboardTargetTitle = "Steam Input On-screen Keyboard"

// KeyboardObserver caches CEF port discovery, which can enumerate Windows
// processes. A disconnected endpoint is rediscovered at most every five seconds.
// IsOpen is used by one watcher goroutine; it does not read keyboard contents.
type KeyboardObserver struct {
	cfg           config.Steam
	port          uint16
	nextDiscovery time.Time
}

func NewKeyboardObserver(cfg config.Steam) *KeyboardObserver {
	return &KeyboardObserver{cfg: cfg}
}

func (o *KeyboardObserver) IsOpen(ctx context.Context) (bool, error) {
	if o.port == 0 {
		o.port = o.cfg.CEFRemoteDebugPort
		if o.port == 0 || o.port == defaultCEFRemoteDebugPort {
			o.port = GetCefDebugPort(ctx)
		}
		o.nextDiscovery = time.Now().Add(5 * time.Second)
	}
	tabs, err := getCEFTabsAtPort(ctx, o.port)
	if err != nil {
		if !time.Now().Before(o.nextDiscovery) {
			o.port = 0
		}
		return false, err
	}
	if len(tabs) == 0 {
		return false, errors.New("Steam CEF has no targets")
	}
	keyboardTabs := keyboardTargets(tabs)
	if len(keyboardTabs) > 1 {
		if err := closeDuplicateKeyboardTargets(ctx, keyboardTabs); err != nil {
			return true, fmt.Errorf("close duplicate Steam keyboard target: %w", err)
		}
	}
	return len(keyboardTabs) > 0, nil
}

func keyboardTargetPresent(tabs []TabInfo) bool {
	return len(keyboardTargets(tabs)) > 0
}

func keyboardTargets(tabs []TabInfo) []TabInfo {
	var targets []TabInfo
	for _, tab := range tabs {
		if tab.Title == keyboardTargetTitle {
			targets = append(targets, tab)
		}
	}
	return targets
}

// closeDuplicateKeyboardTargets preserves one keyboard and closes any extra
// CEF pages. Duplicate keyboards can otherwise consume controller focus after
// the visible keyboard is dismissed.
func closeDuplicateKeyboardTargets(ctx context.Context, tabs []TabInfo) error {
	if len(tabs) < 2 {
		return nil
	}
	for _, tab := range tabs[1:] {
		if err := closeKeyboardTarget(ctx, tab.WebSocketDebuggerURL); err != nil {
			return err
		}
	}
	return nil
}

func closeKeyboardTarget(ctx context.Context, debuggerURL string) error {
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
	return wsjson.Write(ctx, conn, struct {
		ID     int             `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params,omitempty"`
	}{ID: 1, Method: "Page.close"})
}
