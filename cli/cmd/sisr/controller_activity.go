package sisr

import (
	"sync/atomic"

	"github.com/Alia5/SISR/sdl"
)

// controllerActivity tracks the physical action currently in progress. Layout
// restoration waits for the corresponding release event rather than a delay.
type controllerActivity struct{ active atomic.Int64 }

func (a *controllerActivity) observe(ev sdl.Event) {
	var delta int64
	switch ev.Base().Type {
	case sdl.EventTypeGamepadButtonDown, sdl.EventTypeGamepadTouchpadDown:
		delta = 1
	case sdl.EventTypeGamepadButtonUp, sdl.EventTypeGamepadTouchpadUp:
		delta = -1
	default:
		return
	}
	if delta > 0 {
		a.active.Add(delta)
		return
	}
	for {
		current := a.active.Load()
		if current <= 0 || a.active.CompareAndSwap(current, current-1) {
			return
		}
	}
}

func (a *controllerActivity) idle() bool { return a.active.Load() == 0 }
