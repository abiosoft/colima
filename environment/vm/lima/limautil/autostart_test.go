package limautil

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		output    string
		want      string
		supported bool
		wantErr   bool
	}{
		{output: "limactl version 2.3.0\n", want: "2.3.0", supported: true},
		{output: "limactl version 2.4.1\n", want: "2.4.1", supported: true},
		{output: "limactl version v2.3.0\n", want: "2.3.0", supported: true},
		{output: "limactl version 2.2.0\n", want: "2.2.0", supported: false},
		// development builds sort below the release they precede, and are rejected.
		{output: "limactl version 2.2.0-17-gcd1a4b23\n", want: "2.2.0-17-gcd1a4b23", supported: false},
		{output: "limactl version 2.3.0-beta.0\n", want: "2.3.0-beta.0", supported: false},
		{output: "", wantErr: true},
		{output: "limactl version not-a-version\n", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			version, err := parseVersion(tt.output)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseVersion(%q) expected an error, got %v", tt.output, version)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseVersion(%q) returned an error: %v", tt.output, err)
			}
			if got := version.String(); got != tt.want {
				t.Errorf("parseVersion(%q) = %q, want %q", tt.output, got, tt.want)
			}
			if got := !version.LessThan(minAutostartVersion); got != tt.supported {
				t.Errorf("version %q supported = %v, want %v", tt.want, got, tt.supported)
			}
		})
	}
}
