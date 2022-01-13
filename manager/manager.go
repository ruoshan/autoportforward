// Manager uses two dedicated bidirectional streams for communication between local and remote agent.
// Here are the commands:
//  - PING: expected PONG response
//  - FWD {rport}: create a new listener on the receiving side
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

// Req
const (
	PING = "png"
	FWD  = "fwd"
	DEL  = "del"
)

// Resp
const (
	LSN = "lsn"
	ACK = "ack"
)

type Manager struct {
	receiver     io.ReadWriteCloser
	sender       io.ReadWriteCloser
	cmdCh        chan string
	lsnCh        chan []uint16
	shutdownHook func()
	once         sync.Once
	logger       *log.Logger
	localPortMap map[uint16]uint16 // target port => local listener port
	peerPortMap  map[uint16]uint16 // peer's listening ports: target port => peer listener port
	fwdCallback  func(port uint16) (finalPort uint16, err error)
	delCallback  func(port uint16) error
	dumpCallback func(localPortMap, peerPortMap map[uint16]uint16)
}

func NewManager(receiver io.ReadWriteCloser, sender io.ReadWriteCloser, logger *log.Logger, shutdownHook func()) *Manager {
	return &Manager{
		receiver:     receiver,
		sender:       sender,
		cmdCh:        make(chan string),
		lsnCh:        make(chan []uint16),
		shutdownHook: shutdownHook,
		once:         sync.Once{},
		logger:       logger,
		localPortMap: make(map[uint16]uint16),
		peerPortMap:  make(map[uint16]uint16),
		fwdCallback:  nil,
		delCallback:  nil,
		dumpCallback: nil,
	}
}

func (m *Manager) Run() {
	go m.receivingLoop()
	go m.sendingLoop()
	go m.healthcheck()
}

func (m *Manager) receivingLoop() {
	defer func() {
		m.logger.Println("Stop receiving")
	}()
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
			lports := m.fwdPorts(ports)
			m.receiver.Write([]byte(LSN))
			m.receiver.Write(m.encodeSlice(lports))
			m.DumpPorts()
			continue
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

func (m *Manager) fwdPorts(ports []uint16) (lports []uint16) {
	lports = make([]uint16, 0, 10)
	for _, p := range ports {
		if _, ok := m.localPortMap[p]; ok {
			continue
		}
		m.localPortMap[p] = 0
		if m.fwdCallback != nil {
			lport, _ := m.fwdCallback(p)
			m.localPortMap[p] = lport
		}
		lports = append(lports, m.localPortMap[p])
	}
	return lports
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
	defer func() {
		m.logger.Println("Stop sending")
	}()
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
		switch string(buf) {
		case LSN:
			m.lsnCh <- m.decodeSlice(m.sender)
		case ACK:
			// OK
		default:
			m.logger.Println("Unexpected resp")
			m.Shutdown()
		}
	}
}

// yamux has its own healthcheck implemented, this is kinda redundant.
func (m *Manager) healthcheck() {
	defer func() {
		m.logger.Println("Stop healthcheck")
	}()
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

// UpdatePeerPorts takes a full list of ports that're going to be listened on the peer side.
// This will also command the peer to remove oudated ports from listening.
func (m *Manager) UpdatePeerPorts(ports []uint16) {
	fwdList := make([]uint16, 0, 10)
	delList := make([]uint16, 0, 10)
	newPortMap := make(map[uint16]uint16)
	for _, p := range ports {
		if _, ok := m.peerPortMap[p]; !ok {
			// new ports to fwd
			fwdList = append(fwdList, p)
			newPortMap[p] = 0 // the peer's listening port is not determined until the FWD cmd is confirmed
		} else {
			newPortMap[p] = m.peerPortMap[p]
		}
	}
	for p := range m.peerPortMap {
		if _, ok := newPortMap[p]; !ok {
			// old ports to del
			delList = append(delList, p)
		}
	}
	m.peerPortMap = newPortMap

	if len(fwdList) > 0 {
		m.cmdCh <- FWD + string(m.encodeSlice(fwdList))
		peerListenPorts := <-m.lsnCh
		if len(fwdList) != len(peerListenPorts) {
			panic("Expected FWD length equal to LSN")
		}
		for i, p := range fwdList {
			m.peerPortMap[p] = peerListenPorts[i]
		}
	}

	if len(delList) > 0 {
		m.cmdCh <- DEL + string(m.encodeSlice(delList))
	}

	if len(delList)+len(fwdList) > 0 {
		m.DumpPorts()
	}
}

func (m *Manager) SetDumpCallback(dumpCallback func(local, peer map[uint16]uint16)) {
	m.dumpCallback = dumpCallback
}

func (m *Manager) DumpPorts() {
	if m.dumpCallback != nil {
		m.dumpCallback(m.localPortMap, m.peerPortMap)
	}
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

func DumpToStderr(localPortMap, peerPortMap map[uint16]uint16) {
	lst := make([]string, 0, 10)
	for targetPort, listenPort := range localPortMap {
		lst = append(lst, fmt.Sprintf("%d ==> %d", listenPort, targetPort))
	}
	for targetPort, listenPort := range peerPortMap {
		lst = append(lst, fmt.Sprintf("%d <== %d", targetPort, listenPort))
	}
	fmt.Fprintf(os.Stderr, "\r%s", strings.Repeat(" ", 100))
	fmt.Fprintf(os.Stderr, "\rForwarding: [%s]", strings.Join(lst, ", "))
}
