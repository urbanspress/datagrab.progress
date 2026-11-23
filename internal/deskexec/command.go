package deskexec

import (
	"context"
	"os/exec"
)

// Command prepares a background process with the same lifetime and I/O as exec.Command.
func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	applyDetachedWindow(cmd)
	return cmd
}

// CommandContext prepares a background process bound to ctx.
func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	applyDetachedWindow(cmd)
	return cmd
}
