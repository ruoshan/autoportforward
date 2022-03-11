package portscan

import (
	"reflect"
	"strings"
	"testing"
)

const content = `sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
0: 00000000:232C 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 86255494 1 0000000000000000 100 0 0 10 0
1: 00000000:006F 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 18129 1 0000000000000000 100 0 0 10 0
2: 00000000:0050 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 86197589 1 0000000000000000 100 0 0 10 0
`

func Test_parseProcNetTcp(t *testing.T) {
	f := strings.NewReader(content)
	ports := parseProcNetTcp(f)
	if ports[0] != 9004 || ports[2] != 80 {
		t.FailNow()
	}
}

func Test_mergePorts(t *testing.T) {
	type args struct {
		portsV4 []uint16
		portsV6 []uint16
	}
	tests := []struct {
		name string
		args args
		want []uint16
	}{
		{
			name: "Remove duplicated port",
			args: args{
				portsV4: []uint16{123, 124, 125},
				portsV6: []uint16{111, 123, 125, 126},
			},
			want: []uint16{111, 123, 124, 125, 126},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergePorts(tt.args.portsV4, tt.args.portsV6); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergePorts() = %v, want %v", got, tt.want)
			}
		})
	}
}
