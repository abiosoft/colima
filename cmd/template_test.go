package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/embedded"
	"gopkg.in/yaml.v3"
)

func TestTemplateFileOrDefault(t *testing.T) {
	originalVMType := defaultVMType
	originalMountType := startCmd.Flag("mount-type").DefValue
	t.Cleanup(func() {
		defaultVMType = originalVMType
		startCmd.Flag("mount-type").DefValue = originalMountType
	})

	for _, tt := range []struct {
		name      string
		vmType    string
		mountType string
		existing  string
	}{
		{name: "QEMU defaults", vmType: "qemu", mountType: "sshfs"},
		{name: "VZ defaults", vmType: "vz", mountType: "virtiofs"},
		{name: "existing template", vmType: "vz", mountType: "virtiofs", existing: "vmType: qemu\nmountType: 9p\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("COLIMA_HOME", t.TempDir())
			defaultVMType = tt.vmType
			startCmd.Flag("mount-type").DefValue = tt.mountType

			if tt.existing != "" {
				if err := os.MkdirAll(filepath.Dir(templateFile()), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(templateFile(), []byte(tt.existing), 0644); err != nil {
					t.Fatal(err)
				}
			}

			result, err := templateFileOrDefault()
			if err != nil {
				t.Fatal(err)
			}
			if tt.existing != "" {
				if result != tt.existing {
					t.Fatalf("existing template changed: %q", result)
				}
				return
			}

			data, err := embedded.Read("defaults/colima.yaml")
			if err != nil {
				t.Fatal(err)
			}
			var want, got config.Config
			if err := yaml.Unmarshal(data, &want); err != nil {
				t.Fatal(err)
			}
			want.VMType = tt.vmType
			want.MountType = tt.mountType
			if err := yaml.Unmarshal([]byte(result), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("template config = %#v, want %#v", got, want)
			}
		})
	}
}
