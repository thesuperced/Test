package cef

import "testing"

func TestDuplicateOpenPreservesDesktopState(t *testing.T) {
	var state overlaySnapshot
	if !state.transition(true, true, false) {
		t.Fatal("first open ignored")
	}
	if state.transition(true, false, true) {
		t.Fatal("duplicate open was applied")
	}
	if !state.transition(false, false, true) || !state.visibility || state.hitTest {
		t.Fatalf("original state lost: %+v", state)
	}
	if state.transition(false, false, false) {
		t.Fatal("duplicate close was applied")
	}
	if !state.transition(true, false, true) || state.visibility || !state.hitTest {
		t.Fatal("second session did not capture its own state")
	}
}
