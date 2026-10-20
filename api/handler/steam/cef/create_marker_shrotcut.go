package cef

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Alia5/SISR/cefpayloads"
	"github.com/Alia5/SISR/cmd"
	"github.com/Alia5/SISR/helper"
	"github.com/Alia5/SISR/steam"
	"github.com/danielgtaylor/huma/v2"
)

func registerCreateMarkerShortcut(a huma.API, c *cmd.SISRContext) {
	huma.Register(a, huma.Operation{
		Method: http.MethodPost,
		Path:   "/api/v1/steam/cef/create-marker-shortcut",
		Tags:   []string{"steam", "cef", "inject"},
	}, createMarkerShortcut(c))
}

func createMarkerShortcut(c *cmd.SISRContext) func(ctx context.Context, req *struct{}) (*struct{}, error) {
	return func(ctx context.Context, req *struct{}) (*struct{}, error) {
		c.Config.Lock()
		steamCfg := *c.Config.Steam
		c.Config.Unlock()

		SISRPath, err := helper.GetOwnExecutablePath()
		if err != nil {
			slog.Error("Failed to get SISR executable path", "error", err)
			return nil, err
		}

		steamAppID := os.Getenv("SteamAppId")
		markerAppID := os.Getenv("SISR_MARKER_ID")
		if (steamAppID != "" && steamAppID != "0") || (markerAppID != "" && markerAppID != "0") {
			slog.Debug("Marker shortcut already exists OR launched via Steam, skipping creation", "appId", markerAppID)
			return nil, nil
		}

		// Steam's process can be running several seconds before SharedJSContext,
		// which owns AddShortcut, is available after a restart. Wait for that
		// actual capability instead of treating process startup as readiness.
		ready := false
		for range 30 {
			tabs, tabsErr := steam.GetCEFTabs(ctx, &steamCfg)
			if tabsErr == nil {
				for _, tab := range tabs {
					if tab.Title == "SharedJSContext" {
						ready = true
						break
					}
				}
			}
			if ready {
				break
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Second):
			}
		}
		if !ready {
			return nil, fmt.Errorf("Steam UI is not ready for marker creation; switch Steam to Online Mode and wait for the Library to load")
		}

		_, err = cefpayloads.NewCreateMarkerShortcut(&steamCfg).
			Execute(
				ctx,
				&cefpayloads.CreateMarkerShortcutArgs{
					SISRPath: filepath.ToSlash(SISRPath),
				},
			)
		if err != nil {
			slog.Error("failed to create marker shortcut", "error", err)
		}

		return nil, err
	}
}
