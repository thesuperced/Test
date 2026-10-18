package steam

import (
	"context"
	"errors"
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
	return keyboardTargetPresent(tabs), nil
}

func keyboardTargetPresent(tabs []TabInfo) bool {
	for _, tab := range tabs {
		if tab.Title == keyboardTargetTitle {
			return true
		}
	}
	return false
}
