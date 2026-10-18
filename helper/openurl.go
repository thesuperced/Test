package helper

import (
	"context"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"
)

func OpenURL(url string) error {
	return OpenURLContext(context.Background(), url)
}

// OpenURLContext opens a URL using the platform handler, stopping the handler
// process if the caller cancels or times out.
func OpenURLContext(ctx context.Context, url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32.exe"
		args = []string{"url.dll,FileProtocolHandler", strings.ReplaceAll(url, "&", "^&")}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}

	slog.Debug("Opening URL", "url", url)
	e := exec.CommandContext(ctx, cmd, args...)
	err := e.Start()
	if err != nil {
		return err
	}
	err = e.Wait()
	if err != nil {
		return err
	}

	return nil
}
