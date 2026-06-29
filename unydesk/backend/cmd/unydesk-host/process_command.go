package main

import (
	"context"
	"os/exec"
)

func newBackgroundCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	configureBackgroundCommand(cmd)
	return cmd
}
