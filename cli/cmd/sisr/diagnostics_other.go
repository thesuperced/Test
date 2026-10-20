//go:build !windows

package sisr

import "context"

func watchFocusDiagnostics(ctx context.Context) {}
