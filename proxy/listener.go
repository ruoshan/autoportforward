package proxy

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"syscall"

	"github.com/ruoshan/autoportforward/mux"
)

type ProxyListener struct {
	muxClient mux.MuxClient
	listeners map[uint16]*net.TCPListener
	portMap   map[uint16]uint16 // remote port => local port
	logger    *log.Logger
}

func NewProxyListener(m mux.MuxClient, logger *log.Logger) *ProxyListener {
	return &ProxyListener{
		muxClient: m,
		listeners: make(map[uint16]*net.TCPListener),
		portMap:   make(map[uint16]uint16),
		logger:    logger,
	}
}

// Create new listener that would forward to the remote port (rport).
// The local port will be the same as rport if possible, otherwise:
//   - if rport < 1024, lport == rport + 10000
//   - fallback: a random port is chosen for lport
func (p *ProxyListener) NewListener(rport uint16) (lport uint16, err error) {
	lport = rport
	if rport < 1024 {
		lport = rport + 5000
	}
	lport, err = p.newListener(lport, rport)
	if errors.Is(err, syscall.EADDRINUSE) {
		lport, err = p.newListener(0, rport)
	}
	return lport, err
}

func (p *ProxyListener) newListener(lport, rport uint16) (finalPort uint16, err error) {
	p.logger.Printf("New listener: %d", lport)
	laddr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", lport))
	l, err := net.ListenTCP("tcp4", laddr)
	if err != nil {
		p.logger.Printf("Failed to listen: %s", err)
		return 0, err
	}
	if lport != 0 {
		p.listeners[lport] = l
		p.portMap[rport] = lport
	} else {
		tcpAddr := l.Addr().(*net.TCPAddr)
		lport = uint16(tcpAddr.Port)
		p.listeners[lport] = l
		p.portMap[rport] = lport
	}

	go p.listenLoop(l, rport)
	return lport, nil
}

func (p *ProxyListener) PortInUsed(lport uint16) bool {
	_, ok := p.listeners[lport]
	return ok
}

func (p *ProxyListener) CloseListener(rport uint16) error {
	lport := p.portMap[rport]
	p.logger.Printf("Close listener: %d", lport)
	err := p.listeners[lport].Close()
	delete(p.listeners, lport)
	return err
}

func (p *ProxyListener) listenLoop(l net.Listener, rport uint16) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go func() {
			stream, err := p.muxClient.Connect()
			if err != nil {
				p.logger.Println("Failed to connect to proxy client")
				return
			}

			// Prelude: before start the bi-streaming, need to tell the mux server which
			// target port to proxy to
			buf := make([]byte, 2)
			binary.BigEndian.PutUint16(buf, rport)
			stream.Write(buf)

			wg := sync.WaitGroup{}
			wg.Add(2)
			go func() {
				io.Copy(conn, stream)
				conn.Close()
				wg.Done()
			}()
			go func() {
				io.Copy(stream, conn)
				stream.Close()
				wg.Done()
			}()
			wg.Wait()
		}()
	}
}
