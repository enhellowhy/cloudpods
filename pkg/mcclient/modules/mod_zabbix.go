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

package modules

import (
	"fmt"
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/apis"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modulebase"
)

type ZabbixManager struct {
	modulebase.ResourceManager
}

var (
	Zabbix ZabbixManager
)

func init() {
	Zabbix = ZabbixManager{NewZabbixManager("zabbix", "zabbixs",
		[]string{},
		[]string{},
	)}

	register(&Zabbix)
}

func (this *ZabbixManager) jsonRpc(session *mcclient.ClientSession, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	path := "/api_jsonrpc.php"

	//return modulebase.Post(this.ResourceManager, session, path, params, "")
	resp, err := modulebase.Post(this.ResourceManager, session, path, jsonutils.Marshal(params), "")
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("zabbix json rpc response empty")
	}
	_, err = resp.Get(Result)
	if err != nil {
		respError, e := resp.Get(Error)
		if e != nil {
			return nil, fmt.Errorf("zabbix json rpc response get 'error' failed %v", e)
		}
		data, e := respError.GetString(Data)
		if e != nil {
			return nil, e
		}
		return nil, fmt.Errorf(data)
	}
	return resp, nil
}

type UserLoginParams struct {
	Id      int    `json:"id"`
	JsonRpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		User     string `json:"user"`
		Password string `json:"password"`
	} `json:"params"`
}

type ZabbixBaseOptions struct {
	JsonRpc string `json:"jsonrpc" help:"Json RPC version" default:"2.0"`
	Method  string `json:"method" help:"Rpc request method"`
	Auth    string `json:"auth" help:"Rpc request auth"`
	Id      int    `json:"id" help:"Rpc request id" default:"100"`
}

//host update
type ZabbixHostUpdateOptions struct {
	ZabbixBaseOptions
	Params ZabbixHostUpdateParams `json:"params"`
}

type ZabbixHostUpdateParams struct {
	HostId string `json:"hostid"`
	Status int    `json:"status"`
}

// host get
type ZabbixHostGetOptions struct {
	ZabbixBaseOptions
	Params ZabbixHostGetParams `json:"params"`
}

type ZabbixHostGetParams struct {
	Filter ZabbixHostGetFilterParams `json:"filter"`
}

type ZabbixHostGetFilterParams struct {
	Host string `json:"host"`
	Ip   string `json:"ip"`
}

//host delete
type ZabbixHostDeleteOptions struct {
	ZabbixBaseOptions
	Params []string `json:"params"`
}

func (this *ZabbixManager) UserLogin(session *mcclient.ClientSession) (string, error) {
	path := "/api_jsonrpc.php"
	user, password, err := this.getServiceUserPassword(session)
	if err != nil {
		return "", err
	}

	params := new(UserLoginParams)
	params.Id = 1
	params.JsonRpc = "2.0"
	params.Method = "user.login"
	params.Params = struct {
		User     string `json:"user"`
		Password string `json:"password"`
	}{User: user, Password: password}

	body, err := modulebase.Post(this.ResourceManager, session, path, jsonutils.Marshal(params), "")
	if err != nil {
		return "", err
	}
	if body == nil {
		return "", fmt.Errorf("empty response")
	}
	auth, err := body.GetString(Result)
	if err != nil {
		return "", err
	}
	//return result
	if auth != "" {
		return auth, nil
	}

	errBody, err := body.Get(Error)
	if err != nil {
		return "", err
	}
	if errBody == nil {
		return "", fmt.Errorf("error body is null?")
	}
	data, err := errBody.GetString(Data)
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf(data)
}

func (this *ZabbixManager) UserLogout(session *mcclient.ClientSession, auth string) (jsonutils.JSONObject, error) {
	params := new(jsonutils.JSONDict)
	params.Set("id", jsonutils.NewInt(1))
	params.Set("method", jsonutils.NewString("user.logout"))
	params.Set("jsonrpc", jsonutils.NewString("2.0"))
	params.Set("params", jsonutils.NewArray())
	params.Set("auth", jsonutils.NewString(auth))

	return this.jsonRpc(session, params)
}

func (this *ZabbixManager) HostGet(session *mcclient.ClientSession, auth, host, ip string) (string, error) {
	params := new(ZabbixHostGetOptions)
	params.Method = "host.get"
	params.Id = 100
	params.JsonRpc = "2.0"
	params.Params = ZabbixHostGetParams{
		Filter: ZabbixHostGetFilterParams{
			Host: host,
			Ip:   ip,
		},
	}
	params.Auth = auth

	resp, err := this.jsonRpc(session, jsonutils.Marshal(params))
	if err != nil {
		return "", err
	}
	hosts, _ := resp.GetArray(Result)
	if len(hosts) == 0 {
		return "", nil
	}
	hostId, _ := hosts[0].GetString("hostid")
	return hostId, nil
}

func (this *ZabbixManager) HostDelete(session *mcclient.ClientSession, auth, hostId string) (jsonutils.JSONObject, error) {
	params := new(ZabbixHostDeleteOptions)
	params.Method = "host.delete"
	params.Id = 100
	params.JsonRpc = "2.0"
	params.Params = []string{hostId}
	params.Auth = auth

	return this.jsonRpc(session, jsonutils.Marshal(params))
}

func (this *ZabbixManager) HostDisable(session *mcclient.ClientSession, auth, hostId string) (jsonutils.JSONObject, error) {
	params := new(ZabbixHostUpdateOptions)
	params.Method = "host.update"
	params.Id = 100
	params.JsonRpc = "2.0"
	params.Params = ZabbixHostUpdateParams{
		hostId,
		1,
	}
	params.Auth = auth

	return this.jsonRpc(session, jsonutils.Marshal(params))
}

func (this *ZabbixManager) HostEnable(session *mcclient.ClientSession, auth, hostId string) (jsonutils.JSONObject, error) {
	params := new(ZabbixHostUpdateOptions)
	params.Method = "host.update"
	params.Id = 100
	params.JsonRpc = "2.0"
	params.Params = ZabbixHostUpdateParams{
		hostId,
		0,
	}
	params.Auth = auth

	return this.jsonRpc(session, jsonutils.Marshal(params))
}

func (this *ZabbixManager) getServiceUserPassword(session *mcclient.ClientSession) (string, string, error) {
	result, err := ServicesV3.GetByName(session, apis.SERVICE_TYPE_ZABBIX, nil)
	if err != nil {
		return "", "", err
	}
	extra, err := result.Get(Extra)
	if err != nil {
		return "", "", err
	}
	user, err := extra.GetString(User)
	if err != nil {
		return "", "", err
	}
	password, err := extra.GetString(Password)
	if err != nil {
		return "", "", err
	}
	return user, password, nil
}
