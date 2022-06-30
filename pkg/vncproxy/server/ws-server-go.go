package server

import (
	"golang.org/x/net/websocket"
	"io"
	"net/http"
	"net/url"
	"yunion.io/x/log"
)

type WsServer struct {
	cfg *ServerConfig
}

type WsHandler func(io.ReadWriter, *ServerConfig, string)

func (wsServer *WsServer) Listen(urlStr string, handlerFunc WsHandler) {

	if urlStr == "" {
		urlStr = "/"
	}
	url, err := url.Parse(urlStr)
	if err != nil {
		log.Errorf("error while parsing url: ", err)
	}

	http.Handle(url.Path, websocket.Handler(
		func(ws *websocket.Conn) {
			path := ws.Request().URL.Path
			var sessionId string
			if path != "" {
				sessionId = path[1:]
			}

			ws.PayloadType = websocket.BinaryFrame
			handlerFunc(ws, wsServer.cfg, sessionId)
		}))

	err = http.ListenAndServe(url.Host, nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}

func (wsServer *WsServer) Handler(handlerFunc WsHandler) websocket.Handler {
	return websocket.Handler(
		func(ws *websocket.Conn) {
			//path := ws.Request().URL.Path
			//if path != "" {
			//	sessionId = path[1:]
			//}

			var sessionId string
			ws.PayloadType = websocket.BinaryFrame
			handlerFunc(ws, wsServer.cfg, sessionId)
		})
}
