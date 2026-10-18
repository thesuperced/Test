package sisr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows"
)

var errAlreadyRunning = errors.New("SISR is already running; exit the existing tray instance before starting another copy")

func acquireSingleInstance() (func(), error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	processes, listErr := process.ProcessesWithContext(ctx)
	if listErr == nil {
		currentPID := int32(os.Getpid())
		for _, candidate := range processes {
			if candidate.Pid == currentPID {
				continue
			}
			name, nameErr := candidate.NameWithContext(ctx)
			if nameErr == nil && strings.EqualFold(name, "SISR.exe") {
				showAlreadyRunningMessage()
				return nil, errAlreadyRunning
			}
		}
	}

	name, err := windows.UTF16PtrFromString(`Local\SISR.SingleInstance`)
	if err != nil {
		return nil, fmt.Errorf("create single-instance name: %w", err)
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		_ = windows.CloseHandle(handle)
		showAlreadyRunningMessage()
		return nil, errAlreadyRunning
	}
	if err != nil {
		return nil, fmt.Errorf("create single-instance mutex: %w", err)
	}
	return func() { _ = windows.CloseHandle(handle) }, nil
}

func showAlreadyRunningMessage() {
	text, _ := windows.UTF16PtrFromString("SISR is already running. Exit the existing SISR tray instance before starting this copy.")
	caption, _ := windows.UTF16PtrFromString("SISR already running")
	_, _ = windows.MessageBox(0, text, caption, windows.MB_OK|windows.MB_ICONWARNING)
}
