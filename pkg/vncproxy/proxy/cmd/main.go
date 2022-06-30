package main

import (
	"flag"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/vncproxy/proxy"
)

func main() {
	//create default session if required
	var tcpPort = flag.String("tcpPort", "", "tcp port")
	var wsPort = flag.String("wsPort", "", "websocket port")
	var vncPass = flag.String("vncPass", "", "password on incoming vnc connections to the proxy, defaults to no password")
	var recordDir = flag.String("recDir", "", "path to save FBS recordings WILL NOT RECORD if not defined.")
	var targetVnc = flag.String("target", "", "target vnc server (host:port or /path/to/unix.socket)")
	var targetVncPort = flag.String("targPort", "", "target vnc server port (deprecated, use -target)")
	var targetVncHost = flag.String("targHost", "", "target vnc server host (deprecated, use -target)")
	var targetVncPass = flag.String("targPass", "", "target vnc password")
	//var logLevel = flag.String("logLevel", "info", "change logging level")

	flag.Parse()
	log.SetLogLevel(log.Logger(), logrus.DebugLevel)

	if *tcpPort == "" && *wsPort == "" {
		log.Errorf("no listening port defined")
		flag.Usage()
		os.Exit(1)
	}

	if *targetVnc == "" && *targetVncPort == "" {
		log.Errorf("no target vnc server host/port or socket defined")
		flag.Usage()
		os.Exit(1)
	}

	if *vncPass == "" {
		log.Warningln("proxy will have no password")
	}

	tcpURL := ""
	if *tcpPort != "" {
		tcpURL = ":" + string(*tcpPort)
	}
	wsURL := ""
	if *wsPort != "" {
		wsURL = "http://0.0.0.0:" + string(*wsPort) + "/"
	}
	proxy := &proxy.VncProxy{
		WsListeningURL:   wsURL, // empty = not listening on ws
		TCPListeningURL:  tcpURL,
		ProxyVncPassword: *vncPass, //empty = no auth
		SingleSession: &proxy.VncSession{
			Target:         *targetVnc,
			TargetHostname: *targetVncHost,
			TargetPort:     *targetVncPort,
			TargetPassword: *targetVncPass, //"vncPass",
			ID:             "dummySession",
			Status:         proxy.SessionStatusInit,
			Type:           proxy.SessionTypeProxyPass,
		}, // to be used when not using sessions
		UsingSessions: false, //false = single session - defined in the var above
	}

	if *recordDir != "" {
		fullPath, err := filepath.Abs(*recordDir)
		if err != nil {
			log.Errorf("bad recording path: ", err)
		}
		log.Infof("FBS recording is turned on, writing to dir: ", fullPath)
		proxy.RecordingDir = fullPath
		proxy.SingleSession.Type = proxy.SessionTypeRecordingProxy
	} else {
		log.Infoln("FBS recording is turned off")
	}

	proxy.StartListening()
}
