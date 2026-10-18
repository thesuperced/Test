package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var testBuildFiles = []string{
	"SISR.exe",
	"viiper.exe",
	"SDL3.dll",
	"TEST-BUILD-README.txt",
	"SHA256SUMS.txt",
}

func main() {
	if runtime.GOOS != "windows" {
		fmt.Println("This uninstaller is for the Windows test build only.")
		return
	}
	executable, err := os.Executable()
	if err != nil {
		fail("Cannot locate the test-build folder", err)
	}
	dir := filepath.Dir(executable)

	fmt.Println("SISR Keyboard Recovery TEST Build Uninstaller")
	fmt.Println()
	fmt.Println("This removes only test-build files from:")
	fmt.Println(dir)
	fmt.Println()
	fmt.Println("Your normal SISR installation and AppData configuration are preserved.")
	fmt.Println("Fully quit the test SISR and VIIPER first.")
	fmt.Print("Type UNINSTALL to continue: ")
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if strings.TrimSpace(answer) != "UNINSTALL" {
		fmt.Println("Cancelled. No files were removed.")
		pause()
		return
	}

	for _, name := range testBuildFiles {
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Could not remove %s: %v\n", name, err)
			fmt.Println("Quit the test SISR and VIIPER, then run this uninstaller again.")
			pause()
			return
		}
	}

	quotedExecutable := executable
	command := fmt.Sprintf(`ping 127.0.0.1 -n 3 >nul & del /f /q "%s"`, quotedExecutable)
	if err := exec.Command("cmd.exe", "/C", command).Start(); err != nil {
		fail("Test files were removed, but the uninstaller could not remove itself", err)
	}
	fmt.Println("The SISR test build was removed. You may delete the empty folder.")
}

func fail(message string, err error) {
	fmt.Printf("%s: %v\n", message, err)
	pause()
	os.Exit(1)
}

func pause() {
	fmt.Print("Press Enter to close...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
