package steam

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/Alia5/SISR/config"
)

const keyboardTargetTitle = "Steam Input On-screen Keyboard"

// KeyboardObserver caches CEF port discovery, which can enumerate Windows
// processes. A disconnected endpoint is rediscovered at most every five seconds.
// IsOpen is used by one watcher goroutine; it does not read keyboard contents.
type KeyboardObserver struct {
	cfg           config.Steam
	port          uint16
	nextDiscovery time.Time
	keyboardCount int
}

func NewKeyboardObserver(cfg config.Steam) *KeyboardObserver {
	return &KeyboardObserver{cfg: cfg, keyboardCount: -1}
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
	if len(keyboardTabs) != o.keyboardCount {
		o.keyboardCount = len(keyboardTabs)
		slog.Info("Steam keyboard target count changed", "count", o.keyboardCount)
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
