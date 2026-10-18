package steam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Alia5/SISR/config"
	"github.com/Alia5/SISR/helper"
)

const defaultCEFRemoteDebugPort = 8080

type TabInfo struct {
	Description          string `json:"description"`
	DevtoolsFrontendURL  string `json:"devtoolsFrontendUrl"`
	ID                   string `json:"id"`
	Title                string `json:"title"`
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func GetCEFTabs(ctx context.Context, cfg *config.Steam) ([]TabInfo, error) {

	remoteDebugPort := uint16(defaultCEFRemoteDebugPort)
	if cfg != nil && cfg.CEFRemoteDebugPort != 0 {
		remoteDebugPort = cfg.CEFRemoteDebugPort
	}

	if remoteDebugPort == defaultCEFRemoteDebugPort || remoteDebugPort == 0 {
		// find if user is running millennium and detect cef debug port
		port := GetCefDebugPort(ctx)
		if port != defaultCEFRemoteDebugPort {
			slog.Info("Likely running millennium...", "CEF Remote port", port)
			remoteDebugPort = port
		}
	}

	return getCEFTabsAtPort(ctx, remoteDebugPort)
}

func getCEFTabsAtPort(ctx context.Context, remoteDebugPort uint16) ([]TabInfo, error) {
	url := fmt.Sprintf("http://localhost:%d/json", remoteDebugPort)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from CEF remote debug endpoint: %d", resp.StatusCode)
	}

	var tabs []TabInfo
	if err := json.NewDecoder(resp.Body).Decode(&tabs); err != nil {
		return nil, err
	}

	return tabs, nil
}

func GetCefDebugPort(ctx context.Context) uint16 {
	if runtime.GOOS == "linux" {
		return getCefDebugPortLinux()
	}

	if runtime.GOOS == "windows" {
		return getCefDebugPortWindows(ctx)
	}

	slog.Error("CEF debug port detection is unsupported on this OS", "os", runtime.GOOS)
	return defaultCEFRemoteDebugPort
}

func getCefDebugPortLinux() uint16 {
	const defaultPort uint16 = defaultCEFRemoteDebugPort
	const prefix = "--remote-debugging-port="

	entries, err := os.ReadDir("/proc")
	if err != nil {
		slog.Error("Failed to inspect /proc for steamwebhelper", "error", err)
		return defaultPort
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid := entry.Name()
		_, parseErr := strconv.Atoi(pid)
		if parseErr != nil {
			continue
		}

		commPath := filepath.Join("/proc", pid, "comm")
		comm, readErr := os.ReadFile(commPath)
		if readErr != nil {
			continue
		}

		name := strings.ToLower(strings.TrimSpace(string(comm)))
		if name != "steamwebhelper" {
			continue
		}

		cmdlinePath := filepath.Join("/proc", pid, "cmdline")
		cmdline, readErr := os.ReadFile(cmdlinePath)
		if readErr != nil || len(cmdline) == 0 {
			continue
		}

		args := strings.FieldsSeq(strings.ReplaceAll(string(cmdline), "\x00", " "))
		for arg := range args {
			if !strings.HasPrefix(arg, prefix) {
				continue
			}

			portStr := strings.TrimPrefix(arg, prefix)

			if strings.Contains(portStr, ",") {
				split := strings.Split(portStr, ",")
				portStr = split[0]
			}

			port, parseErr := strconv.Atoi(portStr)
			if parseErr != nil || port <= 0 || port > 65535 {
				continue
			}

			return uint16(port)
		}
	}

	return defaultPort
}

func CEFRemoteDebugReachable(ctx context.Context, cfg *config.Steam) bool {

	tabs, err := GetCEFTabs(ctx, cfg)
	if err != nil {
		slog.Error("Error getting CEF tabs", "error", err)
		return false
	}

	for _, tab := range tabs {
		if strings.HasPrefix(tab.URL, "https://steamloopback.host") {
			return true
		}
	}

	return false
}

func CEFRemoteDebugEnableFilePresent(cfg *config.Steam) (bool, error) {
	steamDir := cfg.InstallDir
	var err error
	if steamDir == "" {
		steamDir, err = steamPath()
		if err != nil {
			slog.Error("Could not determine Steam Path", "error", err)
			return false, err
		}
	}

	_, err = os.Stat(filepath.Join(steamDir, ".cef-enable-remote-debugging"))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	slog.Error("Error checking for .cef-enable-remote-debugging file", "error", err)
	return false, err
}

func EnableCefRemoteDebug(cfg *config.Steam) (fileCreated bool, err error) {
	filePresent, err := CEFRemoteDebugEnableFilePresent(cfg)
	if err != nil {
		return false, err
	}
	if filePresent {
		return false, nil
	}
	steamDir := cfg.InstallDir
	if steamDir == "" {
		steamDir, err = ExecuteableDir()
		if err != nil {
			return false, err
		}
	}

	filePath := filepath.Join(steamDir, ".cef-enable-remote-debugging")
	if err = os.WriteFile(filePath, nil, 0644); err != nil {
		if !errors.Is(err, os.ErrPermission) {
			return false, err
		}
		slog.Info("direct file creation failed, attempting elevated creation", "error", err)
		if err = createCefFileElevated(filePath); err != nil {
			return false, err
		}
	}
	return true, nil
}

func createCefFileElevated(filePath string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("elevated file creation not supported on %s", runtime.GOOS)
	}
	exe, err := helper.GetOwnExecutablePath()
	if err != nil {
		return err
	}
	cmd := exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-Command",
		fmt.Sprintf("Start-Process '%s' -ArgumentList '--create-cef-file','\"%s\"' -Verb RunAs",
			strings.ReplaceAll(exe, "'", "''"),
			strings.ReplaceAll(filePath, "'", "''"),
		))
	return cmd.Run()
}

func CreateCefFile(path string) error {
	return os.WriteFile(path, nil, 0644)
}
