package transport

import (
	"reflect"
	"testing"
)

func TestPathToCommand(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    any
		wantErr bool
	}{
		{
			"root listing",
			"/show/interface/",
			map[string]any{"show": map[string]any{"interface": nil}},
			false,
		},
		{
			"single iface lookup",
			"/show/interface/Wireguard0",
			map[string]any{"show": map[string]any{"interface": map[string]any{"name": "Wireguard0"}}},
			false,
		},
		{
			"query param",
			"/show/interface/system-name?name=Wireguard0",
			map[string]any{"show": map[string]any{"interface": map[string]any{"system-name": map[string]any{"name": "Wireguard0"}}}},
			false,
		},
		{
			"deep nested",
			"/show/sc/dns-proxy/route",
			map[string]any{"show": map[string]any{"sc": map[string]any{"dns-proxy": map[string]any{"route": nil}}}},
			false,
		},
		{
			"rc object-group",
			"/show/rc/object-group/fqdn",
			map[string]any{"show": map[string]any{"rc": map[string]any{"object-group": map[string]any{"fqdn": nil}}}},
			false,
		},
		{
			"running-config no params",
			"/show/running-config",
			map[string]any{"show": map[string]any{"running-config": nil}},
			false,
		},
		{
			"leading slash optional",
			"show/interface/",
			map[string]any{"show": map[string]any{"interface": nil}},
			false,
		},
		{
			"empty path",
			"",
			nil,
			true,
		},
		{
			"malformed query — empty",
			"/show/x?",
			nil,
			true,
		},
		{
			"malformed query — no value",
			"/show/x?key",
			nil,
			true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := pathToCommand(tc.path)
			if tc.wantErr {
				if err == nil {
					t.Errorf("pathToCommand(%q) = no error, want error", tc.path)
				}
				return
			}
			if err != nil {
				t.Fatalf("pathToCommand(%q) err = %v", tc.path, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("pathToCommand(%q):\n  got  %#v\n  want %#v", tc.path, got, tc.want)
			}
		})
	}
}
