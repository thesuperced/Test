package cef

// Accessed only from the SDL window dispatcher.
type overlaySnapshot struct {
	open       bool
	visibility bool
	hitTest    bool
}

func (s *overlaySnapshot) transition(open, visibility, hitTest bool) bool {
	if s.open == open {
		return false
	}
	s.open = open
	if open {
		s.visibility = visibility
		s.hitTest = hitTest
	}
	return true
}
