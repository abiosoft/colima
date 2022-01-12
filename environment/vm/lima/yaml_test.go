package lima

import (
	"fmt"
	"testing"
)

func Test_checkOverlappingMounts(t *testing.T) {
	type args struct {
		mounts []string
	}
	tests := []struct {
		args    args
		wantErr bool
	}{
		{args: args{mounts: []string{"/User", "/User/something"}}, wantErr: true},
		{args: args{mounts: []string{"/User/one", "/User/two"}}, wantErr: false},
		{args: args{mounts: []string{"/User/one", "/User/one_other"}}, wantErr: false},
		{args: args{mounts: []string{"/User/one_other", "/User/one"}}, wantErr: false},
		{args: args{mounts: []string{"/User/one", "/User/one/other"}}, wantErr: true},
		{args: args{mounts: []string{"/User/one/", "/User/one"}}, wantErr: true},
		{args: args{mounts: []string{"/User/one/", "/User/two", "User/one"}}, wantErr: true},
		{args: args{mounts: []string{"/home/a/b/c", "/home/b/c/a", "/home/c/a/b"}}, wantErr: false},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if err := checkOverlappingMounts(tt.args.mounts); (err != nil) != tt.wantErr {
				t.Errorf("checkOverlappingMounts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
