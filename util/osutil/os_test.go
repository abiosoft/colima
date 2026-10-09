package osutil

import (
	"os"
	"slices"
	"testing"
)

func TestEnvironWithout(t *testing.T) {
	t.Setenv("LIMA_WORKDIR", "/tmp/does-not-exist-in-vm")
	t.Setenv("COLIMA_TEST_KEEP", "1")

	got := EnvironWithout("LIMA_WORKDIR")
	if slices.Contains(got, "LIMA_WORKDIR=/tmp/does-not-exist-in-vm") {
		t.Fatalf("EnvironWithout kept LIMA_WORKDIR: %v", got)
	}
	if !slices.Contains(got, "COLIMA_TEST_KEEP=1") {
		t.Fatalf("EnvironWithout dropped unrelated var: %v", got)
	}

	// empty skip list returns a full environ copy that still includes LIMA_WORKDIR
	all := EnvironWithout()
	if !slices.Contains(all, "LIMA_WORKDIR=/tmp/does-not-exist-in-vm") {
		t.Fatalf("EnvironWithout() without keys should keep LIMA_WORKDIR")
	}

	// ensure LookupEnv still sees the var on the process (we only filtered the slice)
	if _, ok := os.LookupEnv("LIMA_WORKDIR"); !ok {
		t.Fatal("t.Setenv LIMA_WORKDIR missing from process env")
	}
}
