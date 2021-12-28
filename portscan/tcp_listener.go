package portscan

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// Please refer to the format of this file here: https://www.kernel.org/doc/Documentation/networking/proc_net_tcp.txt
// This is a demo of the content of the file:
//   sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
//   0: 00000000:232C 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 86255494 1 0000000000000000 100 0 0 10 0
//   1: 00000000:006F 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 18129 1 0000000000000000 100 0 0 10 0
//   2: 00000000:0050 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 86197589 1 0000000000000000 100 0 0 10 0
// What we want to get is the `local_address` of those line that has `st` == `0A`.
// the `0A` is the TCP_LISTEN state of a TCP socket, see `/include/net/tcp_states.h` in kernel source.
const PROC_TCP = "/proc/net/tcp"

type TCPListenerScanner struct{}

func parseProcNetTcp(f io.Reader) []uint16 {
	ports := make([]uint16, 0, 10)
	scanner := bufio.NewScanner(f)
	scanner.Scan() // skip first line
	for scanner.Scan() {
		segments := strings.SplitN(strings.TrimSpace(scanner.Text()), " ", 5)
		st := segments[3]
		if st != "0A" {
			break
		}
		portStr := strings.SplitN(segments[1], ":", 2)[1]
		buf, _ := hex.DecodeString(portStr)
		port := binary.BigEndian.Uint16(buf)
		ports = append(ports, port)
	}
	return ports
}

func portsChanged(a, b []uint16) bool {
	if len(a) != len(b) {
		return true
	}
	sort.SliceStable(a, func(i, j int) bool {
		return a[i] < a[j]
	})
	sort.SliceStable(b, func(i, j int) bool {
		return b[i] < b[j]
	})
	for i := range a {
		if a[i] != b[i] {
			return true
		}
	}
	return false
}

func (t *TCPListenerScanner) Parse() []uint16 {
	f, _ := os.Open(PROC_TCP)
	defer f.Close()
	return parseProcNetTcp(f)
}

func (t *TCPListenerScanner) Run(emit chan<- []uint16) {
	tick := time.NewTicker(1 * time.Second)
	prev := t.Parse()
	emit <- prev
	for range tick.C {
		current := t.Parse()
		if portsChanged(prev, current) {
			prev = make([]uint16, len(current))
			copy(prev, current)
			emit <- current
		}
	}
}
