//go:build !windows

package deskexec

import "os/exec"

func applyDetachedWindow(*exec.Cmd) {}
