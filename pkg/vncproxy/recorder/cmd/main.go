package main

import (
	"flag"
	"github.com/sirupsen/logrus"
	"net"
	"os"
	"time"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/vncproxy/client"
	"yunion.io/x/onecloud/pkg/vncproxy/common"
	"yunion.io/x/onecloud/pkg/vncproxy/encodings"
	"yunion.io/x/onecloud/pkg/vncproxy/recorder"
)

func main() {
	// var tcpPort = flag.String("tcpPort", "", "tcp port")
	// var wsPort = flag.String("wsPort", "", "websocket port")
	// var vncPass = flag.String("vncPass", "", "password on incoming vnc connections to the proxy, defaults to no password")
	var recordDir = flag.String("recFile", "/Users/wanghuanyu/recording.fbs", "FBS file to create, recordings WILL NOT RECORD IF EMPTY.")
	var targetVncPort = flag.String("targPort", "5905", "target vnc server port")
	var targetVncPass = flag.String("targPass", "FYQ54zkF", "target vnc password")
	var targetVncHost = flag.String("targHost", "192.168.13.87", "target vnc hostname")
	//var logLevel = flag.String("logLevel", "info", "change logging level")

	flag.Parse()
	log.SetLogLevel(log.Logger(), logrus.DebugLevel)

	if *targetVncHost == "" {
		log.Errorln("no target vnc server host defined")
		flag.Usage()
		os.Exit(1)
	}

	if *targetVncPort == "" {
		log.Errorln("no target vnc server port defined")
		flag.Usage()
		os.Exit(1)
	}

	if *targetVncPass == "" {
		log.Warningln("no password defined, trying to connect with null authentication")
	}
	if *recordDir == "" {
		log.Warningln("FBS recording is turned off")
	} else {
		log.Infof("Recording rfb stream into file: '%s'", *recordDir)
	}

	//nc, err := net.Dial("tcp", "192.168.1.101:5903")
	nc, err := net.Dial("tcp", *targetVncHost+":"+*targetVncPort)

	if err != nil {
		log.Errorf("error connecting to vnc server: %s", err)
	}
	var noauth client.ClientAuthNone
	authArr := []client.ClientAuth{&client.PasswordAuth{Password: *targetVncPass}, &noauth}

	//vncSrvMessagesChan := make(chan common.ServerMessage)

	//rec, err := recorder.NewRecorder("c:/Users/betzalel/recording.rbs")
	rec, err := recorder.NewRecorder(*recordDir) //"/Users/amitbet/vncRec/recording.rbs")
	if err != nil {
		log.Errorf("error creating recorder: %s", err)
		return
	}

	clientConn, err := client.NewClientConn(nc,
		&client.ClientConfig{
			Auth:      authArr,
			Exclusive: true,
		})

	clientConn.Listeners.AddListener(rec)
	clientConn.Listeners.AddListener(&recorder.RfbRequester{Conn: clientConn, Name: "Rfb Requester"})
	clientConn.Connect()

	if err != nil {
		log.Errorf("error creating client: %s", err)
		return
	}
	// err = clientConn.FramebufferUpdateRequest(false, 0, 0, 1024, 768)
	// if err != nil {
	// 	logger.Errorf("error requesting fb update: %s", err)
	// }
	encs := []common.IEncoding{
		&encodings.TightEncoding{},
		//&encodings.TightPngEncoding{},
		//&encodings.RREEncoding{},
		//&encodings.ZLibEncoding{},
		//&encodings.ZRLEEncoding{},
		//&encodings.CopyRectEncoding{},
		//coRRE := encodings.CoRREEncoding{},
		//hextile := encodings.HextileEncoding{},
		&encodings.PseudoEncoding{int32(common.EncJPEGQualityLevelPseudo8)},
	}

	clientConn.SetEncodings(encs)
	//width := uint16(1280)
	//height := uint16(800)

	//clientConn.FramebufferUpdateRequest(false, 0, 0, width, height)

	// // clientConn.SetPixelFormat(&common.PixelFormat{
	// // 	BPP:        32,
	// // 	Depth:      24,
	// // 	BigEndian:  0,
	// // 	TrueColor:  1,
	// // 	RedMax:     255,
	// // 	GreenMax:   255,
	// // 	BlueMax:    255,
	// // 	RedShift:   16,
	// // 	GreenShift: 8,
	// // 	BlueShift:  0,
	// // })

	// start := getNowMillisec()
	// go func() {
	// 	for {
	// 		if getNowMillisec()-start >= 10000 {
	// 			break
	// 		}

	// 		err = clientConn.FramebufferUpdateRequest(true, 0, 0, 1280, 800)
	// 		if err != nil {
	// 			logger.Errorf("error requesting fb update: %s", err)
	// 		}
	// 		time.Sleep(250 * time.Millisecond)
	// 	}
	// 	clientConn.Close()
	// }()

	for {
		time.Sleep(time.Minute)
	}
}
func getNowMillisec() int {
	return int(time.Now().UnixNano() / int64(time.Millisecond))
}