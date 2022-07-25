package lima

import (
	"fmt"
	"net"
	"testing"
)

func Test_includesHost(t *testing.T) {
	type args struct {
		hostsFileContent string
		host             string
		ip               net.IP
	}
	tests := []struct {
		args args
		want bool
	}{
		{
			want: false, args: args{
				hostsFileContent: "",
				host:             "localhost",
				ip:               net.ParseIP("127.0.0.1"),
			},
		},
		{
			want: false, args: args{
				hostsFileContent: "127.0.0.1",
				host:             "localhost",
				ip:               net.ParseIP("127.0.0.1"),
			},
		},
		{
			want: true, args: args{
				hostsFileContent: "127.0.0.1 myhost",
				host:             "myhost",
				ip:               net.ParseIP("127.0.0.1"),
			},
		},
		{
			want: false, args: args{
				hostsFileContent: "127.0.0.1 myhost\n",
				host:             "host",
				ip:               net.ParseIP("127.0.0.1"),
			},
		},
		{
			want: false, args: args{
				hostsFileContent: "127.0.0.1 host\n",
				host:             "host",
				ip:               net.ParseIP("127.0.1.1"),
			},
		},
		{
			want: true, args: args{
				hostsFileContent: "127.0.0.1 host\n127.0.1.1 host",
				host:             "host",
				ip:               net.ParseIP("127.0.1.1"),
			},
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if got := includesHost(tt.args.hostsFileContent, tt.args.host, tt.args.ip); got != tt.want {
				t.Errorf("includesHost() = %v, want %v", got, tt.want)
			}
		})
	}
}
