package lima

import (
	"strings"
	"testing"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
)

func TestDiskMountScriptFilesystem(t *testing.T) {
	script := diskMountScript(true, "xfs")

	for _, expected := range []string{
		`DISK_FS="xfs"`,
		`[ "$ACTUAL_FS" != "$DISK_FS" ]`,
		`mkfs -t "$DISK_FS" -L "$DISK_LABEL" "$DISK_PART"`,
		`mount -t "$DISK_FS" "$DISK_PART" "$MOUNT_POINT"`,
	} {
		if !strings.Contains(script, expected) {
			t.Errorf("diskMountScript() does not contain %q", expected)
		}
	}
}

func TestDiskResizeScript(t *testing.T) {
	instanceID := config.CurrentProfile().ID
	tests := []struct {
		fsType string
		want   string
	}{
		{fsType: "ext4", want: "resize2fs " + diskByLabelPath(instanceID) + " || true"},
		{fsType: "xfs", want: "xfs_growfs " + limautil.MountPoint() + " || true"},
	}

	for _, tt := range tests {
		t.Run(tt.fsType, func(t *testing.T) {
			if got := diskResizeScript(tt.fsType, instanceID); got != tt.want {
				t.Errorf("diskResizeScript() = %q, want %q", got, tt.want)
			}
		})
	}
}
