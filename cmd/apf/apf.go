package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/ruoshan/autoportforward/bootstrap"
	"github.com/ruoshan/autoportforward/logger"
	"github.com/ruoshan/autoportforward/manager"
	"github.com/ruoshan/autoportforward/mux"
	"github.com/ruoshan/autoportforward/proxy"
)

var version string = "dev"

var log = logger.GetNullLogger()

func sigHandler(fn func()) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		fn()
	}()
}

var isK8s = flag.Bool("k", false, "proxy for Kubernetes pod")
var isPodman = flag.Bool("p", false, "proxy for Podman container")
var dbg = flag.Bool("d", false, "log debug info to /tmp/autoportforward.log")
var reverse = flag.String("r", "", "comma-separated port list. eg. 8080,9090\nlistening ports in the container and forwarding them back")

func init() {
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), `Usage:
    * apf {docker container ID / name}
    * apf -k {namespace}/{pod ID}
    * apf -p {podman container ID / name}
Flags:`)
		flag.PrintDefaults()
		fmt.Printf("Version: %s\n", version)
	}
}

func printPrelude() {
	fmt.Print(`
*  ==> : Forwarding local listening ports to (==>) remote ports
*  <== : Forwarding to local ports from (<==) remote listening ports (use -r option)

`)
}

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	containerId := flag.Arg(0)

	if *dbg {
		log = logger.GetLogger()
	}

	var rt bootstrap.RTType = bootstrap.DOCKER
	if *isK8s {
		rt = bootstrap.KUBERNETES
	}
	if *isPodman {
		rt = bootstrap.PODMAN
	}

	// Bootstrap: copy the agent(tar archive) into the container
	msg, err := bootstrap.Bootstrap(rt, containerId)
	if err != nil {
		panic(fmt.Sprintf("Failed to bootstrap: %s", msg))
	}

	var cmd []string
	switch rt {
	case bootstrap.DOCKER:
		cmd = []string{"docker", "exec", "-i", containerId, "/apf-agent"}
	case bootstrap.KUBERNETES:
		splits := strings.SplitN(containerId, "/", 2)
		cmd = []string{"kubectl", "exec", "-i", "-n", splits[0], splits[1], "/apf-agent"}
	case bootstrap.PODMAN:
		cmd = []string{"podman", "exec", "-i", containerId, "/apf-agent"}
	}
	if *dbg {
		cmd = append(cmd, "-d")
	}

	var reversePorts []uint16
	if len(*reverse) > 0 {
		splits := strings.Split(*reverse, ",")
		for _, p := range splits {
			i, err := strconv.ParseUint(p, 10, 16)
			if err != nil {
				panic("Invalid port in -r option")
			}
			reversePorts = append(reversePorts, uint16(i))
		}
	}

	ms := mux.NewCmdPipeMuxServer(cmd[0], cmd[1:]...)
	if ms == nil {
		panic("Failed to create mux server")
	}
	sigHandler(func() {
		ms.Shutdown()
	})

	printPrelude()

	// Open two streams for manager. NB: the order of Accept() is different from Connect() in the remote agent
	mgrReceivingStream, _ := ms.Accept()
	mgrSendingStream, _ := ms.Accept()
	mgr := manager.NewManager(mgrReceivingStream, mgrSendingStream, log, func() {
		ms.Close()
	})

	pl := proxy.NewProxyListener(ms, log)
	if pl == nil {
		panic("Failed to create proxy listener")
	}
	mgr.SetCallbacks(pl.NewListener, pl.CloseListener)
	mgr.SetDumpCallback(manager.DumpToStderr)
	mgr.Run()
	if len(reversePorts) > 0 {
		mgr.UpdatePeerPorts(reversePorts)
	}

	pf := proxy.NewProxyForwarder(ms, log)
	if pf == nil {
		panic("Failed to create proxy forwarder")
	}
	pf.Start()
}
