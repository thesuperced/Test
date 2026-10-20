package cef

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/Alia5/SISR/cmd"
	"github.com/Alia5/SISR/sdl"
	"github.com/Alia5/SISR/sdl/extras"
	"github.com/Alia5/SISR/webview"
	"github.com/danielgtaylor/huma/v2"
)

func registerOverlayStateChanged(a huma.API, c *cmd.SISRContext) {
	huma.Register(a, huma.Operation{
		Method:      http.MethodPost,
		Path:        "/api/v1/steam/cef/overlay-state-changed",
		Description: "Callback for CEF Injectee when Steam overlay is opened/closed",
		Tags:        []string{"steam", "cef", "callback"},
	}, overlayStateChanged(c))
}

type OverlayChangedRequest struct {
	Body OverlayChanged
}

type OverlayChanged struct {
	Open bool `json:"open"`
}

func overlayStateChanged(c *cmd.SISRContext) func(ctx context.Context, req *OverlayChangedRequest) (*struct{}, error) {

	var state overlaySnapshot

	return func(ctx context.Context, req *OverlayChangedRequest) (*struct{}, error) {
		c.Config.Lock()
		kbmEnabled := c.Config.KeyboardMouseEmulation
		c.Config.Unlock()

		err, dispatchErr := cmd.ScheduleWindowDispatch(
			ctx,
			c.WindowDispatcher,
			func(w *sdl.Window, wv webview.WebView) error {
				// Notifications can be duplicated (including after reinjection).
				// Preserve the original state until the matching close edge.
				if !state.transition(req.Body.Open, wv.Visible(), extras.GetWindowHitTest(w)) {
					return nil
				}

				slog.Debug("Overlay state changed",
					"previousWebViewVisibility", state.visibility,
					"previousCursorHitTest", state.hitTest,
					"newOverlayOpenState", req.Body.Open,
				)

				if req.Body.Open {
					wv.SetVisible(false)
					err := extras.SetCursorHitTest(w, true)
					if err != nil {
						slog.Error("Failed setting window cursor hittest", "error", err)
					}
				} else {
					if state.visibility {
						wv.SetVisible(true)
					}
					targetHitTest := state.hitTest
					if kbmEnabled {
						targetHitTest = true
					}
					err := extras.SetCursorHitTest(w, targetHitTest)
					if err != nil {
						slog.Error("Failed setting window cursor hittest", "error", err)
					}
				}
				return nil
			})
		if dispatchErr != nil {
			return nil, dispatchErr
		}
		if err != nil {
			return nil, err
		}

		return nil, nil
	}
}
