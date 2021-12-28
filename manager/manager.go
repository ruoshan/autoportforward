// Manager uses two dedicated bi-directional streams for communicating between local and remote agent.
// Here are the commands:
//  - PING: expected PONG response
//  - FWD {rport}: create a new listener on the receving side, pick a random port on the receiving side
//  - DEL {rport}: delete the listener on the receiving side
package manager

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// All commands are three-letter string over the wire
const CMD_LEN = 3
const (
	PING = "png"
	FWD  = "fwd"
	DEL  = "del"
	ACK  = "ack"
)

type Manager struct {
	receiver      io.ReadWriteCloser
	sender        io.ReadWriteCloser
	cmdCh         chan string
	shutdownHook  func()
	once          sync.Once
	logger        *log.Logger
	localPortMap  map[uint16]uint16 // forward target port => local listener port
	remotePortMap map[uint16]bool   // up-to-date remote listening ports
	fwdCallback   func(port uint16) (finalPort uint16, err error)
	delCallback   func(port uint16) error
}

func NewManager(receiver io.ReadWriteCloser, sender io.ReadWriteCloser, logger *log.Logger, shutdownHook func()) *Manager {
	return &Manager{
		receiver:      receiver,
		sender:        sender,
		cmdCh:         make(chan string),
		shutdownHook:  shutdownHook,
		once:          sync.Once{},
		logger:        logger,
		localPortMap:  make(map[uint16]uint16),
		remotePortMap: make(map[uint16]bool),
		fwdCallback:   nil,
		delCallback:   nil,
	}
}

func (m *Manager) Run() {
	go m.receivingLoop()
	go m.sendingLoop()
	go m.healthcheck()
}

func (m *Manager) receivingLoop() {
	buf := make([]byte, CMD_LEN)
	for {
		_, err := io.ReadFull(m.receiver, buf)
		if err != nil {
			m.Shutdown()
			return
		}
		switch string(buf) {
		case PING:
			// noop
		case FWD:
			ports := m.decodeSlice(m.receiver)
			m.fwdPorts(ports)
			m.DumpPorts()
		case DEL:
			ports := m.decodeSlice(m.receiver)
			m.delPorts(ports)
			m.DumpPorts()
		default:
			panic(fmt.Sprintf("unknown manager command: %v", buf))
		}
		m.receiver.Write([]byte(ACK))
	}
}

func (m *Manager) fwdPorts(ports []uint16) {
	for _, p := range ports {
		if _, ok := m.localPortMap[p]; ok {
			continue
		}
		m.localPortMap[p] = 0
		if m.fwdCallback != nil {
			lport, _ := m.fwdCallback(p)
			m.localPortMap[p] = lport
		}
	}
}

func (m *Manager) delPorts(ports []uint16) {
	for _, p := range ports {
		delete(m.localPortMap, p)
		if m.delCallback != nil {
			m.delCallback(p)
		}
	}
}

func (m *Manager) sendingLoop() {
	buf := make([]byte, CMD_LEN)
	for cmd := range m.cmdCh {
		if cmd == "" {
			return
		}
		m.sender.Write([]byte(cmd))
		timer := time.AfterFunc(5*time.Second, func() {
			m.logger.Println("Timeout!")
			m.Shutdown()
		})
		_, err := io.ReadFull(m.sender, buf)
		timer.Stop()
		if err != nil {
			m.logger.Println("Error resp")
			m.Shutdown()
		}
		if string(buf) != ACK {
			m.logger.Println("Unexpected resp")
			m.Shutdown()
		}
	}
}

// yamux has its own healthcheck implemented, this is kinda redundant.
func (m *Manager) healthcheck() {
	tick := time.NewTicker(5 * time.Second)
	for range tick.C {
		m.cmdCh <- PING
	}
}

func (m *Manager) encodeSlice(s []uint16) []byte {
	hdr := make([]byte, 2)
	binary.BigEndian.PutUint16(hdr, uint16(len(s)))
	buf := &bytes.Buffer{}
	buf.Write(hdr)
	binary.Write(buf, binary.BigEndian, s)
	return buf.Bytes()
}

func (m *Manager) decodeSlice(r io.Reader) []uint16 {
	hdr := make([]byte, 2)
	_, err := io.ReadFull(r, hdr)
	if err != nil {
		return nil
	}
	size := binary.BigEndian.Uint16(hdr)
	ports := make([]uint16, size)
	binary.Read(r, binary.BigEndian, ports)
	return ports
}

// Update takes a full list of ports that're listening.
// This method is called by portscanner.
func (m *Manager) UpdatePorts(ports []uint16) {
	fwdList := make([]uint16, 0, 10)
	delList := make([]uint16, 0, 10)
	newPortMap := make(map[uint16]bool)
	for _, p := range ports {
		newPortMap[p] = true
		if !m.remotePortMap[p] {
			// new ports to fwd
			fwdList = append(fwdList, p)
		}
	}
	for p := range m.remotePortMap {
		if !newPortMap[p] {
			// old ports to del
			delList = append(delList, p)
		}
	}
	m.remotePortMap = newPortMap
	if len(fwdList) > 0 {
		m.cmdCh <- FWD + string(m.encodeSlice(fwdList))
	}
	if len(delList) > 0 {
		m.cmdCh <- DEL + string(m.encodeSlice(delList))
	}
}

func (m *Manager) DumpPorts() {
	lst := make([]string, 0, 10)
	for targetPort, listenPort := range m.localPortMap {
		lst = append(lst, fmt.Sprintf("%d ==> %d", listenPort, targetPort))
	}
	fmt.Fprintf(os.Stderr, "\r%s", strings.Repeat(" ", 100))
	fmt.Fprintf(os.Stderr, "\rLISTENING PORTS: [%s]", strings.Join(lst, ", "))
}

func (m *Manager) SetCallbacks(fwdCallback func(port uint16) (finalPort uint16, err error), delCallback func(port uint16) error) {
	m.fwdCallback = fwdCallback
	m.delCallback = delCallback
}

func (m *Manager) Shutdown() {
	m.once.Do(func() {
		m.logger.Println("Shutting down")
		m.receiver.Close()
		m.sender.Close()
		close(m.cmdCh)
		m.shutdownHook()
	})
}
