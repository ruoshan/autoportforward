package main

import (
	"flag"
	"syscall"

	"github.com/ruoshan/bportforward/logger"
	"github.com/ruoshan/bportforward/manager"
	"github.com/ruoshan/bportforward/mux"
	"github.com/ruoshan/bportforward/portscan"
	"github.com/ruoshan/bportforward/proxy"
)

// NB: agent CAN NOT use stdout as log output! stdout has been taken by the StdioMuxClient.
var log = logger.GetNullLogger()
var dbg = flag.Bool("d", false, "log debug info to /tmp/autoportforward.log")

func main() {
	flag.Parse()
	if *dbg {
		log = logger.GetLogger()
	}

	log.Println("Agent starts")
	mc := mux.NewStdioMuxClient()
	if mc == nil {
		panic("Failed to create mux client")
	}

	// Open two streams for manager
	mgrSendingStream, _ := mc.Connect()
	mgrReceivingStream, _ := mc.Connect()
	mgr := manager.NewManager(mgrReceivingStream, mgrSendingStream, log, func() {
		mc.Close()
	})

	// Keep scanning listening ports
	go func() {
		portscanner := &portscan.TCPListenerScanner{}
		portsCh := make(chan []uint16)
		go portscanner.Run(portsCh)
		for ports := range portsCh {
			mgr.UpdatePorts(ports)
		}
	}()

	pl := proxy.NewProxyListener(mc, log)
	if pl == nil {
		panic("Failed to create proxy server")
	}
	mgr.SetCallbacks(pl.NewListener, pl.CloseListener)
	mgr.Run()

	pf := proxy.NewProxyForwarder(mc, log)
	if pf == nil {
		panic("Failed to create proxy forwarder")
	}
	pf.Start()
	log.Println("Agent stops")
	syscall.Unlink("/apf-agent")
}
