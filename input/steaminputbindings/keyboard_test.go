package steaminputbindings

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func testEnforcer() *enforcer {
	return &enforcer{ownAppID: 42, forcedAppID: 42, generation: 1, openURL: func(context.Context, string) error { return nil }}
}

func TestKeyboardRecoveryTransitions(t *testing.T) {
	now := time.Now()
	var s keyboardRecovery
	observe := func(ms int, open bool, generation uint64, want bool) {
		t.Helper()
		if got := s.observe(now.Add(time.Duration(ms)*time.Millisecond), open, generation); got != want {
			t.Fatalf("at %dms open=%v generation=%d: got %v want %v", ms, open, generation, got, want)
		}
	}
	observe(0, false, 1, false) // absent at startup
	observe(250, true, 1, false)
	observe(500, true, 1, false)
	observe(750, false, 1, false)
	observe(1049, false, 1, false)
	observe(1050, false, 1, true)
	s = keyboardRecovery{} // consumed cycle
	observe(1300, false, 1, false)
	observe(1500, true, 1, false)
	observe(1750, false, 1, false)
	observe(1900, true, 1, false) // reopen cancels debounce
	observe(2050, true, 1, false)
	observe(2250, false, 1, false)
	observe(2500, false, 2, false) // mode changed while pending
	observe(2800, false, 2, false)
}

func TestWatchKeyboard(t *testing.T) {
	for _, scenario := range []string{"cycles", "reopen", "cef failure", "disabled", "other app", "off on", "retry exhausted", "retry succeeds", "cancel pending", "no steam", "missing appid"} {
		t.Run(scenario, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				e := testEnforcer()
				var open atomic.Bool
				var observationFailed atomic.Bool
				var forceFailed atomic.Bool
				var calls atomic.Int64
				var polls atomic.Int64
				e.openURL = func(context.Context, string) error {
					calls.Add(1)
					if forceFailed.Load() {
						return errors.New("launch failed")
					}
					return nil
				}
				if scenario == "no steam" {
					e.noSteam = true
				}
				if scenario == "missing appid" {
					e.ownAppID = 0
				}
				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan struct{})
				go func() {
					defer close(done)
					e.WatchKeyboard(ctx, func(context.Context) (bool, error) {
						polls.Add(1)
						if observationFailed.Load() {
							return false, errors.New("CEF disconnected")
						}
						return open.Load(), nil
					})
				}()
				synctest.Wait()
				advance := func(d time.Duration) { time.Sleep(d); synctest.Wait() }
				wantCalls := func(want int) {
					t.Helper()
					if calls.Load() != int64(want) {
						t.Fatalf("calls=%d, want %d", calls.Load(), want)
					}
				}
				advance(time.Second)
				wantCalls(0) // never recover merely because initially closed
				open.Store(true)
				advance(time.Second)
				wantCalls(0) // typing remains uninterrupted
				open.Store(false)
				advance(250 * time.Millisecond) // first observed close
				switch scenario {
				case "reopen":
					open.Store(true)
					advance(time.Second)
					wantCalls(0)
					open.Store(false)
					advance(time.Second)
					wantCalls(2)
				case "cef failure":
					observationFailed.Store(true)
					advance(time.Second)
					wantCalls(0)
					observationFailed.Store(false)
					advance(time.Second)
					wantCalls(0) // reconnect closed does not resurrect stale recovery
					open.Store(true)
					advance(250 * time.Millisecond)
					open.Store(false)
					advance(time.Second)
					wantCalls(2)
				case "disabled", "other app", "off on":
					appID := uint32(0)
					if scenario == "other app" {
						appID = 77
					}
					if err := e.ForceInputAppID(appID); err != nil {
						t.Fatal(err)
					}
					if scenario == "off on" {
						if err := e.ForceOwnAppID(); err != nil {
							t.Fatal(err)
						}
					}
					calls.Store(0)
					advance(2 * time.Second)
					wantCalls(0)
				case "retry exhausted", "retry succeeds":
					forceFailed.Store(true)
					advance(300 * time.Millisecond)
					wantCalls(1)
					if scenario == "retry succeeds" {
						forceFailed.Store(false)
					}
					advance(5 * time.Second)
					want := 3
					if scenario == "retry succeeds" {
						want = 3
					}
					wantCalls(want)
					advance(time.Minute)
					wantCalls(want)
				case "cancel pending":
					cancel()
					advance(time.Second)
					wantCalls(0)
				case "no steam", "missing appid":
					advance(time.Second)
					wantCalls(0)
					if polls.Load() != 0 {
						t.Errorf("disabled watcher made %d CEF requests", polls.Load())
					}
				default:
					advance(299 * time.Millisecond)
					wantCalls(0)
					advance(101 * time.Millisecond)
					wantCalls(2)
					advance(time.Second)
					wantCalls(2)
					open.Store(true)
					advance(250 * time.Millisecond)
					open.Store(false)
					advance(time.Second)
					wantCalls(4)
				}
				cancel()
				<-done
				before := polls.Load()
				advance(time.Second)
				if polls.Load() != before {
					t.Fatal("watcher continued after shutdown")
				}
			})
		})
	}
}

func TestWatchKeyboardCancelRequest(t *testing.T) {
	e := testEnforcer()
	ctx, cancel := context.WithCancel(context.Background())
	entered := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		e.WatchKeyboard(ctx, func(ctx context.Context) (bool, error) {
			close(entered)
			<-ctx.Done()
			return false, ctx.Err()
		})
	}()
	<-entered
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("request was not cancelled")
	}
}

func TestReapplyOwnAppIDGuards(t *testing.T) {
	for _, mode := range []string{"disabled", "other app", "generation", "cancelled", "no steam", "missing appid", "enabled"} {
		t.Run(mode, func(t *testing.T) {
			e := testEnforcer()
			calls := 0
			e.openURL = func(context.Context, string) error { calls++; return nil }
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "disabled":
				e.forcedAppID = 0
			case "other app":
				e.forcedAppID = 99
			case "generation":
				e.generation++
			case "cancelled":
				cancel()
			case "no steam":
				e.noSteam = true
			case "missing appid":
				e.ownAppID = 0
				e.forcedAppID = 0
			}
			applied, err := e.reapplyOwnAppID(ctx, 1)
			want := mode == "enabled"
			if err != nil || applied != want || (calls == 2) != want {
				t.Fatalf("applied=%v calls=%d err=%v", applied, calls, err)
			}
		})
	}
}

func TestRecoverySerializesWithModeChanges(t *testing.T) {
	e := testEnforcer()
	e.openURL = func(context.Context, string) error { return nil }
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			generation, _ := e.keyboardRecoveryState()
			_, _ = e.reapplyOwnAppID(context.Background(), generation)
		})
		wg.Go(func() {
			_ = e.ForceInputAppID(0)
			_ = e.ForceOwnAppID()
		})
	}
	wg.Wait()
	if err := e.ForceInputAppID(0); err != nil {
		t.Fatal(err)
	}
	generation, _ := e.keyboardRecoveryState()
	applied, err := e.reapplyOwnAppID(context.Background(), generation)
	if applied || err != nil {
		t.Fatalf("recovery overrode final disable: %v %v", applied, err)
	}
}

func TestWatchKeyboardCancelForce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := testEnforcer()
		entered := make(chan struct{})
		e.openURL = func(ctx context.Context, url string) error {
			if url != "steam://forceinputappid/0" {
				t.Errorf("unexpected URL %q", url)
			}
			close(entered)
			<-ctx.Done()
			return ctx.Err()
		}
		observations := 0
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			e.WatchKeyboard(ctx, func(context.Context) (bool, error) {
				observations++
				return observations == 1, nil
			})
		}()
		synctest.Wait()
		time.Sleep(550 * time.Millisecond)
		synctest.Wait()
		select {
		case <-entered:
		default:
			t.Fatal("recovery did not start")
		}
		cancel()
		<-done
	})
}
