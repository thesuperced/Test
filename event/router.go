package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/Alia5/SISR/event/handler"
	"github.com/Alia5/SISR/logging"
	"github.com/Alia5/SISR/sdl"
)

type Router interface {
	RouteEvent(ctx context.Context, sdlEvent sdl.Event)
	hndlMap() map[sdl.EventType][]func(context.Context, sdl.Event) error
}

type router struct {
	handlers map[sdl.EventType][]func(context.Context, sdl.Event) error
}

func NewRouter() Router {
	r := &router{
		handlers: make(map[sdl.EventType][]func(context.Context, sdl.Event) error),
	}
	return r
}

func (r *router) RouteEvent(appCtx context.Context, ev sdl.Event) {
	switch ev.Base().Type {
	case sdl.EventTypeKeyDown, sdl.EventTypeKeyUp,
		sdl.EventTypeGamepadButtonDown, sdl.EventTypeGamepadButtonUp,
		sdl.EventTypeGamepadTouchpadDown, sdl.EventTypeGamepadTouchpadUp:
		// Do not record button identities, key codes, text, or touch positions.
		logging.Diagnostic("input_event", map[string]any{"kind": ev.Base().Type.String()})
	}
	ctx := context.WithValue(appCtx, "eventTime", ev.Base().Timestamp)
	handlers := r.handlers[ev.Base().Type]
	if len(handlers) > 0 {
		for _, handler := range handlers {
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						eventData, _ := json.Marshal(ev)
						slog.Error("Panic while handling event",
							"panic", rec,
							"event", string(eventData),
							"eventName", ev.Base().Type.String(),
							"stack", string(debug.Stack()),
						)
					}
				}()

				err := handler(ctx, ev)
				if err != nil {
					slog.Error("Failed to handle event", "error", err)
				}
			}()
		}
	}
}

func (r *router) hndlMap() map[sdl.EventType][]func(context.Context, sdl.Event) error {
	return r.handlers
}

func RegisterHandler[T sdl.Event](r Router, op handler.Operation[T]) {
	r.hndlMap()[op.Event] = append(r.hndlMap()[op.Event], func(ctx context.Context, sdlEvent sdl.Event) error {
		ev, ok := sdlEvent.(T)
		if !ok {
			return fmt.Errorf("unexpected event type %T for %v", sdlEvent, op.Event)
		}

		return op.Handler.Handle(ctx, ev)
	})
}
