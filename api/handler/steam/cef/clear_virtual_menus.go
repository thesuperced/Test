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

type virtualMenuVisibilityInput struct {
	Body struct {
		Visible bool `json:"visible" doc:"Whether Steam virtual-menu targets should be visible"`
	}
}

func registerVirtualMenuVisibility(a huma.API, c *cmd.SISRContext) {
	huma.Register(a, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/steam/cef/virtual-menu-visibility",
		Description: "Hide or restore Steam virtual-menu CEF targets after an action-set change",
		Tags:        []string{"steam", "cef", "callback"},
	}, setVirtualMenuVisibility(c))
}

func setVirtualMenuVisibility(c *cmd.SISRContext) func(context.Context, *virtualMenuVisibilityInput) (*struct{}, error) {
	return func(ctx context.Context, input *virtualMenuVisibilityInput) (*struct{}, error) {
		c.Config.Lock()
		steamCfg := *c.Config.Steam
		c.Config.Unlock()

		closeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := steam.SetVirtualMenuVisibility(closeCtx, &steamCfg, input.Body.Visible); err != nil {
			slog.Debug("Failed to change Steam virtual-menu visibility", "visible", input.Body.Visible, "error", err)
			return nil, err
		}
		slog.Debug("Changed Steam virtual-menu visibility", "visible", input.Body.Visible)
		return nil, nil
	}
}
