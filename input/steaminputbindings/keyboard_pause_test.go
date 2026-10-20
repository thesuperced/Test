package steaminputbindings

import (
	"context"
	"reflect"
	"testing"
)

func TestKeyboardOwnershipAndExplicitModeChange(t *testing.T) {
	for _, changeMode := range []bool{false, true} {
		e := testEnforcer()
		var urls []string
		e.openURL = func(_ context.Context, url string) error { urls = append(urls, url); return nil }
		ctx := context.Background()
		if err := e.pauseForKeyboard(ctx, 1); err != nil {
			t.Fatal(err)
		}
		_ = e.pauseForKeyboard(ctx, 1) // subsequent open polls must not reset repeatedly
		if changeMode {
			_ = e.ForceInputAppID(77)
		}
		applied, err := e.reapplyOwnAppID(ctx, 1)
		if err != nil || applied == changeMode {
			t.Fatalf("applied %v error %v", applied, err)
		}
		want := []string{"steam://forceinputappid/0", "steam://forceinputappid/42"}
		if changeMode {
			want[1] = "steam://forceinputappid/77"
		}
		if !reflect.DeepEqual(urls, want) {
			t.Fatalf("got %v want %v", urls, want)
		}
	}
}
