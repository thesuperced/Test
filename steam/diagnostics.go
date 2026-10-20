package steam

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/Alia5/SISR/config"
	"github.com/Alia5/SISR/logging"
)

// WatchDiagnostics only observes target metadata. It never evaluates JavaScript,
// closes a target, or reads text from a keyboard or application window.
func WatchDiagnostics(ctx context.Context, cfg config.Steam) {
	last := ""
	port := cfg.CEFRemoteDebugPort
	if port == defaultCEFRemoteDebugPort {
		port = 0
	}
	var nextDiscovery, lastRecord time.Time
	for ctx.Err() == nil {
		probeCtx, cancel := context.WithTimeout(ctx, time.Second)
		if port == 0 {
			port = GetCefDebugPort(probeCtx)
			nextDiscovery = time.Now().Add(5 * time.Second)
		}
		tabs, err := getCEFTabsAtPort(probeCtx, port)
		if err != nil && !time.Now().Before(nextDiscovery) && (cfg.CEFRemoteDebugPort == 0 || cfg.CEFRemoteDebugPort == defaultCEFRemoteDebugPort) {
			port = 0
		}
		cancel()
		type target struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		}
		targets := []target{}
		for _, tab := range tabs {
			kind := ""
			switch {
			case tab.Title == keyboardTargetTitle:
				kind = "keyboard"
			case isVirtualMenuTab(tab):
				kind = "virtual_menu"
			case tab.Title == "SharedJSContext":
				kind = "shared_context"
			case strings.Contains(strings.ToLower(tab.Title), "overlay"):
				kind = "overlay"
			}
			if kind != "" {
				targets = append(targets, target{tab.ID, kind})
			}
		}
		sort.Slice(targets, func(i, j int) bool { return targets[i].ID < targets[j].ID })
		snapshot, _ := json.Marshal(targets)
		state := string(snapshot)
		if err != nil {
			state = "unavailable"
		}
		if state != last || time.Since(lastRecord) >= 10*time.Second {
			logging.Diagnostic("cef_targets", map[string]any{"available": err == nil, "targets": targets})
			last = state
			lastRecord = time.Now()
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}
