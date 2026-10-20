package steam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/Alia5/SISR/config"
	"github.com/Alia5/SISR/logging"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// RouteNativeKeyboard hides only SISR's in-game keyboard overlay when the
// separate native keyboard target exists. It never closes the keyboard page.
func RouteNativeKeyboard(ctx context.Context, cfg *config.Steam, gameID string, open bool) error {
	if _, err := strconv.ParseUint(gameID, 10, 64); err != nil {
		return err
	}
	tabs, err := GetCEFTabs(ctx, cfg)
	if err != nil {
		return err
	}
	expression := fmt.Sprintf(`window.__sisrNativeKeyboardOpen=%t;window.__sisrKeyboardGameID=%q;`, open, gameID)
	if open {
		expression += `if(window.SteamClient?.Overlay?.SetOverlayState){window.SteamClient.Overlay.SetOverlayState(window.__sisrKeyboardGameID,0)}else{throw new Error("Overlay routing unavailable")}`
	}
	for _, tab := range tabs {
		if tab.Title != "SharedJSContext" {
			continue
		}
		err := evaluateTarget(ctx, tab.WebSocketDebuggerURL, expression)
		logging.Diagnostic("native_keyboard_route", map[string]any{"open": open, "success": err == nil, "game_id": gameID})
		return err
	}
	return errors.New("SharedJSContext unavailable")
}

func evaluateTarget(ctx context.Context, address, expression string) error {
	u, err := url.Parse(address)
	if err != nil {
		return err
	}
	if (u.Scheme != "ws" && u.Scheme != "wss") ||
		(u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") {
		return errors.New("CEF debugger must be loopback websocket")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, address, nil)
	if err != nil {
		return err
	}
	defer ws.CloseNow()
	if err := wsjson.Write(ctx, ws, cdpRequest{ID: 1, Method: "Runtime.evaluate", Params: map[string]interface{}{"expression": expression}}); err != nil {
		return err
	}
	for {
		var reply struct {
			ID     int             `json:"id"`
			Error  json.RawMessage `json:"error"`
			Result struct {
				Exception json.RawMessage `json:"exceptionDetails"`
			} `json:"result"`
		}
		if err := wsjson.Read(ctx, ws, &reply); err != nil {
			return err
		}
		if reply.ID != 1 {
			continue
		}
		if len(reply.Error) > 0 || len(reply.Result.Exception) > 0 {
			return errors.New("Steam rejected CEF evaluation")
		}
		return nil
	}
}
