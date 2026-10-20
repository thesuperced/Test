package steaminputbindings

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/Alia5/SISR/logging"
)

const (
	keyboardPollInterval  = 250 * time.Millisecond
	keyboardRecoveryDelay = 300 * time.Millisecond
	keyboardResetDelay    = 100 * time.Millisecond
	keyboardMaxAttempts   = 3
)

// WatchKeyboard reapplies the currently selected own layout after the Steam
// keyboard closes. A failed observation invalidates the cycle: an unavailable
// or restarting CEF endpoint is not evidence that the keyboard closed.
func (e *enforcer) WatchKeyboard(ctx context.Context, isOpen func(context.Context) (bool, error)) {
	var state keyboardRecovery
	lastOpen := false
	cefUnavailable := false
	for ctx.Err() == nil {
		generation, enabled := e.keyboardRecoveryState()
		delay := keyboardPollInterval
		if !enabled {
			state = keyboardRecovery{}
		} else {
			open, err := isOpen(ctx)
			now := time.Now()
			if err != nil {
				state = keyboardRecovery{}
				if !cefUnavailable && ctx.Err() == nil {
					slog.Debug("Keyboard recovery: CEF observation unavailable")
				}
				cefUnavailable = true
			} else if ctx.Err() == nil {
				if open != lastOpen {
					logging.Diagnostic("keyboard_observed", map[string]any{"open": open, "generation": generation})
					lastOpen = open
				}
				cefUnavailable = false
				if state.observe(now, open, generation) {
					forceCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
					applied, err := e.reapplyOwnAppID(forceCtx, generation)
					logging.Diagnostic("keyboard_layout_recovery", map[string]any{"applied": applied, "success": err == nil, "generation": generation})
					cancel()
					state.attempts++
					if err != nil {
						slog.Debug("Keyboard layout recovery failed", "attempt", state.attempts, "error", err)
					}
					if applied || err == nil || state.attempts >= keyboardMaxAttempts {
						state = keyboardRecovery{}
					} else {
						state.due = time.Now().Add(time.Duration(state.attempts) * keyboardRecoveryDelay)
					}
				}
				if !state.due.IsZero() {
					delay = min(delay, max(0, time.Until(state.due)))
				}
			}
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// Each explicit enforcement change advances the generation, including an
// off/on toggle between polls. Old keyboard observations cannot restore it.
func (e *enforcer) keyboardRecoveryState() (uint64, bool) {
	e.mtx.Lock()
	defer e.mtx.Unlock()
	return e.generation, !e.noSteam && e.ownAppID != 0 && e.forcedAppID == e.ownAppID
}

func (e *enforcer) reapplyOwnAppID(ctx context.Context, generation uint64) (bool, error) {
	e.mtx.Lock()
	if ctx.Err() != nil || e.noSteam || e.ownAppID == 0 || e.forcedAppID != e.ownAppID || e.generation != generation {
		e.mtx.Unlock()
		return false, nil
	}
	// Re-sending the same app ID can be ignored by Steam while a keyboard
	// overlay still owns controller focus. Explicitly release the forced layout
	// first, as SISR's successful shutdown cleanup does, then restore it.
	if err := e.openURL(ctx, "steam://forceinputappid/0"); err != nil {
		e.mtx.Unlock()
		return false, err
	}
	e.mtx.Unlock()
	timer := time.NewTimer(keyboardResetDelay)
	select {
	case <-ctx.Done():
		timer.Stop()
		return false, ctx.Err()
	case <-timer.C:
	}
	e.mtx.Lock()
	defer e.mtx.Unlock()
	if ctx.Err() != nil || e.noSteam || e.ownAppID == 0 || e.forcedAppID != e.ownAppID || e.generation != generation {
		return false, nil
	}
	if err := e.openURL(ctx, "steam://forceinputappid/"+strconv.FormatUint(uint64(e.ownAppID), 10)); err != nil {
		return false, err
	}
	slog.Debug("Reapplied SteamInput layout after keyboard closed")
	return true, nil
}

type keyboardRecovery struct {
	generation uint64
	open       bool
	due        time.Time
	attempts   int
}

func (s *keyboardRecovery) observe(now time.Time, open bool, generation uint64) bool {
	if s.generation != generation {
		*s = keyboardRecovery{generation: generation}
	}
	if open {
		s.open = true
		s.due = time.Time{}
		s.attempts = 0
		return false
	}
	if !s.open {
		return false
	}
	if s.due.IsZero() {
		s.due = now.Add(keyboardRecoveryDelay)
	}
	return !now.Before(s.due)
}
