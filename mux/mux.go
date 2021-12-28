package mux

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/hashicorp/yamux"
)

type MuxServer interface {
	Accept() (io.ReadWriteCloser, error)
	Shutdown() error
}

type MuxClient interface {
	Connect() (io.ReadWriteCloser, error)
}

type YAMux struct {
	is_client bool
	session   *yamux.Session
	reader    io.ReadCloser
	writer    io.WriteCloser
}

var _ MuxClient = &YAMux{}
var _ MuxServer = &YAMux{}

func NewYAMux(r io.ReadCloser, w io.WriteCloser, is_client bool) *YAMux {
	ym := &YAMux{
		is_client: is_client,
		reader:    r,
		writer:    w,
	}
	var session *yamux.Session
	var err error
	if is_client {
		session, err = yamux.Client(ym, nil)
	} else {
		session, err = yamux.Server(ym, nil)
	}
	if err != nil {
		return nil
	}
	ym.session = session
	return ym
}

func (ym *YAMux) Write(b []byte) (int, error) {
	return ym.writer.Write(b)
}

func (ym *YAMux) Read(b []byte) (int, error) {
	return ym.reader.Read(b)
}

func (ym *YAMux) Close() error {
	werr := ym.writer.Close()
	rerr := ym.reader.Close()
	if werr == nil {
		return rerr
	}
	if rerr == nil {
		return werr
	}
	return fmt.Errorf("failed to close both ends of the pipe: %s, %s", werr, rerr)
}

func (ym *YAMux) Accept() (io.ReadWriteCloser, error) {
	return ym.session.Accept()
}

func (ym *YAMux) Connect() (io.ReadWriteCloser, error) {
	return ym.session.Open()
}

func (ym *YAMux) Shutdown() error {
	return nil
}

func NewStdioMuxClient() *YAMux {
	return NewYAMux(os.Stdin, os.Stdout, true)
}

type CmdPipeMuxServer struct {
	*YAMux

	cmd *exec.Cmd
}

var _ MuxServer = &CmdPipeMuxServer{}
var _ MuxClient = &CmdPipeMuxServer{}

func NewCmdPipeMuxServer(name string, args ...string) *CmdPipeMuxServer {
	cmd := exec.Command(name, args...)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	err := cmd.Start()
	if err != nil {
		return nil
	}
	c := &CmdPipeMuxServer{
		cmd: cmd,
	}
	ym := NewYAMux(stdout, stdin, false)
	c.YAMux = ym
	return c
}

func (c *CmdPipeMuxServer) Shutdown() error {
	return c.cmd.Wait()
}
