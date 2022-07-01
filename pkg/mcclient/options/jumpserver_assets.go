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

package options

import (
	"yunion.io/x/jsonutils"
)

type JumpServerNodeListOptions struct {
	BaseListOptions
}

func (opts *JumpServerNodeListOptions) Params() (jsonutils.JSONObject, error) {
	return ListStructToParams(opts)
}

type JumpServerUserCreateOptions struct {
	//Id       string   `json:"id" help:"Resource VM Id"`
	Username string `json:"username" help:"Resource JS Name"`
	Name     string `json:"name" help:"Resource JS Name"`
	Email    string `json:"email" help:"Resource VM Email"`
	Source   string `json:"source" help:"Resource VM source"`
	//Nodes    []string `json:"nodes" help:"Resource VM Nodes"`
}

type JumpServerAssetCreateOptions struct {
	Id        string   `json:"id" help:"Resource VM Id"`
	Hostname  string   `json:"hostname" help:"Resource VM Name"`
	AdminUser string   `json:"admin_user" help:"Resource Admin user"`
	Ip        string   `json:"ip" help:"Resource VM Ip"`
	Platform  string   `json:"platform" help:"Resource VM platform"`
	Nodes     []string `json:"nodes" help:"Resource VM Nodes"`
	Protocols []string `json:"protocols" help:"Resource VM protocols"`
	Port      int      `json:"port" help:"Resource VM port"`
}

type JumpServerAssetPermsCreateOptions struct {
	//Id       string   `json:"id" help:"Resource VM Id"`
	Users       []string `json:"users" help:"Resource JS users"`
	SystemUsers []string `json:"system_users" help:"Resource JS system users"`
	Assets      []string `json:"assets" help:"Resource JS assets"`
	Name        string   `json:"name" help:"Resource JS Name"`
	Actions     []string `json:"actions" help:"Resource JS action"`
}

type JumpServerAssetPermsUpdateOptions struct {
	Id    string   `json:"id" help:"Resource VM Id"`
	Users []string `json:"users" help:"Resource JS users"`
	//SystemUsers []string `json:"system_users" help:"Resource JS system users"`
	Assets []string `json:"assets" help:"Resource JS assets"`
	//Name        string   `json:"name" help:"Resource JS Name"`
}

func (opts *JumpServerAssetCreateOptions) Params() (jsonutils.JSONObject, error) {
	return jsonutils.Marshal(opts).(*jsonutils.JSONDict), nil
}

func (opts *JumpServerUserCreateOptions) Params() (jsonutils.JSONObject, error) {
	return jsonutils.Marshal(opts).(*jsonutils.JSONDict), nil
}

func (opts *JumpServerAssetPermsCreateOptions) Params() (jsonutils.JSONObject, error) {
	return jsonutils.Marshal(opts).(*jsonutils.JSONDict), nil
}

func (opts *JumpServerAssetPermsUpdateOptions) Params() (jsonutils.JSONObject, error) {
	return jsonutils.Marshal(opts).(*jsonutils.JSONDict), nil
}
