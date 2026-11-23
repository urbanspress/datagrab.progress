package deskexec

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestBackgroundConsoleProcess(t *testing.T) {
	const childFlag = "GROXY_DESK_TEST_CONSOLE_CHILD"
	if os.Getenv(childFlag) == "1" {
		getConsoleWindow := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow")
		console, _, _ := getConsoleWindow.Call()
		if console != 0 {
			t.Fatal("child process has a console window")
		}
		fmt.Fprintln(os.Stdout, "console-free stdout")
		fmt.Fprintln(os.Stderr, "console-free stderr")
		return
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := CommandContext(ctx, exe, "-test.run=^TestBackgroundConsoleProcess$")
	cmd.Env = append(os.Environ(), childFlag+"=1")
	if executableFlags := cmd.SysProcAttr; !executableFlags.HideWindow || executableFlags.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatalf("missing console suppression flags: %+v", executableFlags)
	}
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "console-free stdout\nconsole-free stderr\nPASS\n" {
		t.Fatalf("unexpected child result: %v, %q", err, out)
	}
}
