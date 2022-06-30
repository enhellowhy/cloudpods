// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"fmt"
	"golang.org/x/net/websocket"
	"net/http"
	"strconv"
	"yunion.io/x/onecloud/pkg/vncproxy/proxy"
	"yunion.io/x/onecloud/pkg/webconsole/options"
	"yunion.io/x/onecloud/pkg/webconsole/session"
)

type VncproxyServer struct {
	Proxy *proxy.VncProxy
}

func NewVncproxyServer(s *session.SSession) (websocket.Handler, error) {
	info := s.ISessionData.(*session.RemoteConsoleInfo)
	if info.Host == "" {
		return nil, fmt.Errorf("Empty remote host")
	}
	if info.Port <= 0 {
		return nil, fmt.Errorf("Invalid remote port: %d", info.Port)
	}

	proxy := &proxy.VncProxy{
		//WsListeningURL:   wsURL, // empty = not listening on ws
		//TCPListeningURL:  tcpURL,
		SingleSession: &proxy.VncSession{
			TargetHostname: info.Host,
			InstanceName:   info.InstanceName,
			TargetPort:     strconv.FormatInt(info.Port, 10),
			TargetPassword: info.VncPassword, //"vncPass",
			ID:             info.Id,
			Status:         proxy.SessionStatusInit,
			Type:           proxy.SessionTypeRecordingProxy,
			Cred:           info.Cred,
		}, // to be used when not using sessions
		UsingSessions: false, //false = single session - defined in the var above
		RecordingDir:  options.Options.S3MountPoint,
	}
	return proxy.WebsocketHandler(), nil

	//server := &VncproxyServer{
	//	Proxy: proxy,
	//}

	//return server, nil
}

func (s *VncproxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {}
