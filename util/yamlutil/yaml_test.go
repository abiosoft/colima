package yamlutil

import (
	"net"
	"reflect"
	"testing"

	"github.com/abiosoft/colima/config"
	"gopkg.in/yaml.v3"
)

func Test_encode_Docker(t *testing.T) {
	conf := config.Config{
		Docker:     map[string]any{"insecure-registries": []any{"127.0.0.1"}},
		Network:    config.Network{DNSResolvers: []net.IP{net.ParseIP("1.1.1.1")}},
		Kubernetes: config.Kubernetes{K3sArgs: []string{"--disable=traefik"}},
	}

	tests := []struct {
		name    string
		args    config.Config
		want    config.Config
		wantErr bool
	}{
		{name: "nested", args: conf, want: conf},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := encodeYAML(tt.args)
			var got config.Config
			if err := yaml.Unmarshal(b, &got); err != nil {
				t.Errorf("resulting byte is not a valid yaml: %v", err)
				return
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("save() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.Docker, tt.want.Docker) {
				t.Errorf("save() = %+v\nwant %+v", got.Docker, tt.want.Docker)
			}
		})
	}
}
