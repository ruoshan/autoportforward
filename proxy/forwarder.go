package proxy

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/ruoshan/bportforward/mux"
)

type ProxyForwarder struct {
	muxServer mux.MuxServer
	logger    *log.Logger
}

func NewProxyForwarder(m mux.MuxServer, logger *log.Logger) *ProxyForwarder {
	return &ProxyForwarder{
		muxServer: m,
		logger:    logger,
	}
}

func (p *ProxyForwarder) Start() {
	for {
		stream, rport := p.acceptStream()
		if stream == nil {
			return
		}
		go p.forwardLoop(stream, rport)
	}
}

func (p *ProxyForwarder) acceptStream() (stream io.ReadWriteCloser, rport uint16) {
	stream, err := p.muxServer.Accept()
	if err != nil {
		return nil, 0
	}

	// Prelude: parse the target port (2 bytes)
	buf := make([]byte, 2)
	_, err = stream.Read(buf)
	if err != nil {
		p.logger.Println("Failed to read prelude")
		return nil, 0
	}
	rport = binary.BigEndian.Uint16(buf)
	return stream, rport
}

func (p *ProxyForwarder) forwardLoop(stream io.ReadWriteCloser, rport uint16) {
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", rport))
	if err != nil {
		p.logger.Printf("Failed to dial: %d", rport)
		return
	}

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
}
