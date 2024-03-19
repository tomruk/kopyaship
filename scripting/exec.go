package scripting

import (
	"context"
	"os"
	"os/exec"
)

type Exec struct {
	ctx context.Context
	sw  []string
}

func newExec(ctx context.Context, sw ...string) *Exec {
	return &Exec{
		ctx: ctx,
		sw:  sw,
	}
}

func (e *Exec) Location() string { return e.sw[0] }

func (e *Exec) Run() error {
	cmd := exec.CommandContext(e.ctx, e.sw[0], e.sw[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
