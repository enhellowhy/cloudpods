package main

import (
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/vncproxy/common"
	"yunion.io/x/onecloud/pkg/vncproxy/encodings"
	"yunion.io/x/onecloud/pkg/vncproxy/player"
	"yunion.io/x/onecloud/pkg/vncproxy/server"
)

func main() {
	wsPort := flag.String("wsPort", "8888", "websocket port for player to listen to client connections")
	tcpPort := flag.String("tcpPort", "5900", "tcp port for player to listen to client connections")
	fbsFile := flag.String("fbsFile", "", "fbs file to serve to all connecting clients")
	//logLevel := flag.String("logLevel", "info", "change logging level")

	flag.Parse()
	log.SetLogLevel(log.Logger(), logrus.DebugLevel)

	fmt.Println("**************************************************************************")
	fmt.Println("*** This is a toy server that replays a single FBS file to all clients ***")
	fmt.Println("**************************************************************************")

	if *fbsFile == "" {
		log.Errorln("there is no FBS file to replay to incoming clients")
		flag.Usage()
		os.Exit(1)
	}

	if *tcpPort == "" && *wsPort == "" {
		log.Errorln("no listening port defined")
		flag.Usage()
		os.Exit(1)
	}

	//chServer := make(chan common.ClientMessage)
	//chClient := make(chan common.ServerMessage)

	encs := []common.IEncoding{
		&encodings.RawEncoding{},
		&encodings.TightEncoding{},
		&encodings.EncCursorPseudo{},
		&encodings.TightPngEncoding{},
		&encodings.RREEncoding{},
		&encodings.ZLibEncoding{},
		&encodings.ZRLEEncoding{},
		&encodings.CopyRectEncoding{},
		&encodings.CoRREEncoding{},
		&encodings.HextileEncoding{},
	}

	cfg := &server.ServerConfig{
		//SecurityHandlers: []SecurityHandler{&ServerAuthNone{}, &ServerAuthVNC{}},
		SecurityHandlers: []server.SecurityHandler{&server.ServerAuthNone{}},
		Encodings:        encs,
		PixelFormat:      common.NewPixelFormat(32),
		ClientMessages:   server.DefaultClientMessages,
		DesktopName:      []byte("workDesk"),
		Height:           uint16(768),
		Width:            uint16(1024),
	}

	cfg.NewConnHandler = func(cfg *server.ServerConfig, conn *server.ServerConn) error {
		//fbs, err := loadFbsFile("/Users/amitbet/Dropbox/recording.rbs", conn)
		//fbs, err := loadFbsFile("/Users/amitbet/vncRec/recording.rbs", conn)
		fbs, err := player.ConnectFbsFile(*fbsFile, conn)

		if err != nil {
			log.Errorf("TestServer.NewConnHandler: Error in loading FBS: ", err)
			return err
		}
		conn.Listeners.AddListener(player.NewFBSPlayListener(conn, fbs))
		return nil
	}

	if *tcpPort == "" && *wsPort == "" {
		log.Errorln("no listening port defined")
		flag.Usage()
		os.Exit(1)
	}

	url := "http://0.0.0.0:" + *wsPort + "/"

	if *tcpPort != "" && *wsPort != "" {
		log.Infof("running two listeners: tcp port: %s, ws url: %s", *tcpPort, url)

		go server.WsServe(url, cfg)
		server.TcpServe(":"+*tcpPort, cfg)
	}
	if *tcpPort == "" && *wsPort != "" {
		log.Infof("running ws listener url: %s", url)
		server.WsServe(url, cfg)
	}
	log.Infof("running tcp listener on port: %s", *tcpPort)
	server.TcpServe(":"+*tcpPort, cfg)

}
