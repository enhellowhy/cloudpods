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

package compute

import (
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
)

func init() {
	type ZabbixAuthOptions struct {
		Auth string `json:"-"`
	}
	R(&ZabbixAuthOptions{}, "zabbix-user-login", "Zabbix user login", func(s *mcclient.ClientSession, arg *ZabbixAuthOptions) error {
		result, err := modules.Zabbix.UserLogin(s)
		if err != nil {
			return err
		}
		o := new(jsonutils.JSONDict)
		o.Set("auth", jsonutils.NewString(result))
		printObject(o)
		return nil
	})
	R(&ZabbixAuthOptions{}, "zabbix-user-logout", "Zabbix user logout", func(s *mcclient.ClientSession, arg *ZabbixAuthOptions) error {
		result, err := modules.Zabbix.UserLogout(s, arg.Auth)
		if err != nil {
			return err
		}
		printObject(result)
		return nil
	})
	type ZabbixHostGetOptions struct {
		Auth     string `json:"-"`
		HostName string `json:"-"`
		Ip       string `json:"-"`
	}
	R(&ZabbixHostGetOptions{}, "zabbix-host-get", "Zabbix host get", func(s *mcclient.ClientSession, opts *ZabbixHostGetOptions) error {
		result, err := modules.Zabbix.HostGet(s, opts.Auth, opts.HostName, opts.Ip)
		if err != nil {
			return err
		}
		o := new(jsonutils.JSONDict)
		o.Set("hostId", jsonutils.NewString(result))
		printObject(o)
		return nil
	})
	type ZabbixHostIdOptions struct {
		Auth string `json:"-"`
		Id   string `json:"-"`
	}
	R(&ZabbixHostIdOptions{}, "zabbix-host-disable", "Zabbix host disable", func(s *mcclient.ClientSession, opts *ZabbixHostIdOptions) error {
		result, err := modules.Zabbix.HostDisable(s, opts.Auth, opts.Id)
		if err != nil {
			return err
		}
		printObject(result)
		return nil
	})
	R(&ZabbixHostIdOptions{}, "zabbix-host-enable", "Zabbix host enable", func(s *mcclient.ClientSession, opts *ZabbixHostIdOptions) error {
		result, err := modules.Zabbix.HostEnable(s, opts.Auth, opts.Id)
		if err != nil {
			return err
		}
		printObject(result)
		return nil
	})
	R(&ZabbixHostIdOptions{}, "zabbix-host-delete", "Zabbix host delete", func(s *mcclient.ClientSession, opts *ZabbixHostIdOptions) error {
		result, err := modules.Zabbix.HostDelete(s, opts.Auth, opts.Id)
		if err != nil {
			return err
		}
		printObject(result)
		return nil
	})
}
