//go:build windows

package sisr

import (
	"context"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"github.com/Alia5/SISR/logging"
)

func watchFocusDiagnostics(ctx context.Context) {
	dll := syscall.NewLazyDLL("user32.dll")
	foreground := dll.NewProc("GetForegroundWindow")
	process := dll.NewProc("GetWindowThreadProcessId")
	class := dll.NewProc("GetClassNameW")
	image := syscall.NewLazyDLL("kernel32.dll").NewProc("QueryFullProcessImageNameW")
	var last uintptr
	for ctx.Err() == nil {
		hwnd, _, _ := foreground.Call()
		if hwnd != last {
			var pid uint32
			process.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
			var buffer [256]uint16
			class.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
			executable := ""
			if handle, err := syscall.OpenProcess(0x1000, false, pid); err == nil {
				var path [1024]uint16
				length := uint32(len(path))
				ok, _, _ := image.Call(uintptr(handle), 0, uintptr(unsafe.Pointer(&path[0])), uintptr(unsafe.Pointer(&length)))
				if ok != 0 {
					executable = filepath.Base(syscall.UTF16ToString(path[:]))
				}
				_ = syscall.CloseHandle(handle)
			}
			logging.Diagnostic("foreground_window", map[string]any{
				"hwnd": hwnd, "pid": pid, "class": syscall.UTF16ToString(buffer[:]), "process": executable,
			})
			last = hwnd
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(250 * time.Millisecond):
		}
	}
}
