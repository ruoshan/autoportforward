package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"testing"
)

type pipe struct {
	r io.ReadCloser
	w io.WriteCloser
}

func (p *pipe) Read(b []byte) (n int, err error) {
	return p.r.Read(b)
}

func (p *pipe) Write(b []byte) (n int, err error) {
	return p.w.Write(b)
}

func (p *pipe) Close() error {
	p.r.Close()
	p.w.Close()
	return nil
}

type MockMux struct {
	p1 io.ReadWriteCloser
	p2 io.ReadWriteCloser
}

func newMockMux() *MockMux {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	return &MockMux{
		p1: &pipe{r: r1, w: w2},
		p2: &pipe{r: r2, w: w1},
	}
}

func (m *MockMux) Connect() (io.ReadWriteCloser, error) {
	return m.p1, nil
}

func (m *MockMux) Accept() (io.ReadWriteCloser, error) {
	return m.p2, nil
}

func (m *MockMux) Shutdown() error {
	return nil
}

func helperSender(t *testing.T, addr string, msg string) {
	t.Logf("Start sending %s", msg)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.FailNow()
	}
	conn.Write([]byte(msg))
	conn.Close()
	t.Logf("Done sending %s", msg)
}

func helperReceiver(t *testing.T, lport uint16, expectedMsg string, sig chan<- struct{}) {
	t.Logf("Start receiving %s", expectedMsg)
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", lport))
	if err != nil {
		t.FailNow()
	}
	svr, err := net.ListenTCP("tcp", addr)
	if err != nil {
		t.FailNow()
	}
	sig <- struct{}{}
	conn, err := svr.Accept()
	if err != nil {
		t.FailNow()
	}
	buf := make([]byte, len(expectedMsg))
	conn.Read(buf)
	conn.Close()
	t.Logf("Done receiving %s", buf)
	sig <- struct{}{}
	if !bytes.Equal(buf, []byte(expectedMsg)) {
		t.FailNow()
	}
}

func Test_proxy(t *testing.T) {
	mux := newMockMux()
	svr := NewProxyListener(mux, log.Default())
	cli := NewProxyForwarder(mux, log.Default())

	lport, err := svr.newListener(38888, 38889)
	if err != nil {
		t.Fatal(err)
	}

	sig := make(chan struct{})

	go func() {
		<-sig
		stream, port := cli.acceptStream()
		<-sig
		cli.forwardLoop(stream, port)
		<-sig
	}()

	sig <- struct{}{}
	helperSender(t, fmt.Sprintf("127.0.0.1:%d", lport), "testmsg")
	helperReceiver(t, 38889, "testmsg", sig)
}
