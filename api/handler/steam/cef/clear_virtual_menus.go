package cef

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Alia5/SISR/cmd"
	"github.com/Alia5/SISR/steam"
	"github.com/danielgtaylor/huma/v2"
)

func registerClearVirtualMenus(a huma.API, c *cmd.SISRContext) {
	huma.Register(a, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/steam/cef/clear-virtual-menus",
		Description: "Close stale Steam virtual-menu CEF targets after an action-set change",
		Tags:        []string{"steam", "cef", "callback"},
	}, clearVirtualMenus(c))
}

func clearVirtualMenus(c *cmd.SISRContext) func(context.Context, *struct{}) (*struct{}, error) {
	return func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		c.Config.Lock()
		steamCfg := *c.Config.Steam
		c.Config.Unlock()

		closeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := steam.CloseVirtualMenuTabs(closeCtx, &steamCfg); err != nil {
			slog.Debug("Failed to clear stale Steam virtual menus", "error", err)
			return nil, err
		}
		slog.Debug("Cleared stale Steam virtual menus")
		return nil, nil
	}
}
