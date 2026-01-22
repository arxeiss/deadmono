package analysis

import (
	"context"
	"fmt"
	"os/exec"
)

func getCommandOutput(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s\nErr: %s", out, err.Error())
	}
	return out, nil
}
