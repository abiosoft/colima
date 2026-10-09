package limautil

import (
	"strings"
	"testing"
)

func TestLimactlDropsLimaWorkDir(t *testing.T) {
	t.Setenv("LIMA_WORKDIR", "/host/only/path")

	cmd := Limactl("list")
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, EnvLimaWorkDir+"=") {
			t.Fatalf("Limactl env still contains %q", e)
		}
	}
	foundHome := false
	for _, e := range cmd.Env {
		if strings.HasPrefix(e, EnvLimaHome+"=") {
			foundHome = true
			break
		}
	}
	if !foundHome {
		t.Fatalf("Limactl env missing %s; env=%v", EnvLimaHome, cmd.Env)
	}
}
