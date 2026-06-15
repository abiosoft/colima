package limautil

import "testing"

func Test_mirrorURL(t *testing.T) {
	const ghURL = "https://github.com/abiosoft/colima-core/releases/download/v0.10.4/img.raw.gz"

	tests := []struct {
		name   string
		url    string
		mirror string
		want   string
	}{
		{
			name:   "empty mirror returns url unchanged",
			url:    ghURL,
			mirror: "",
			want:   ghURL,
		},
		{
			name:   "github prefix replaced",
			url:    ghURL,
			mirror: "https://artifactory.mycompany.com/artifactory/github",
			want:   "https://artifactory.mycompany.com/artifactory/github/abiosoft/colima-core/releases/download/v0.10.4/img.raw.gz",
		},
		{
			name:   "mirror trailing slash does not double up",
			url:    ghURL,
			mirror: "https://artifactory.mycompany.com/artifactory/github/",
			want:   "https://artifactory.mycompany.com/artifactory/github/abiosoft/colima-core/releases/download/v0.10.4/img.raw.gz",
		},
		{
			name:   "non-github url returned unchanged",
			url:    "https://example.com/abiosoft/img.raw.gz",
			mirror: "https://artifactory.mycompany.com/artifactory/github",
			want:   "https://example.com/abiosoft/img.raw.gz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mirrorURL(tt.url, tt.mirror); got != tt.want {
				t.Errorf("mirrorURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
