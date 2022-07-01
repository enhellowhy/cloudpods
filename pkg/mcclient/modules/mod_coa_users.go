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
	"net/http"
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/apis"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modulebase"
)

type CoaUsersManager struct {
	modulebase.ResourceManager
}

var (
	CoaUsers CoaUsersManager
)

func init() {
	CoaUsers = CoaUsersManager{NewCoaManager("coauser", "coausers",
		[]string{"ID", "Name", "Job_number", "Department_id", "User_name"},
		[]string{"ID", "Name", "Job_number", "Department_id", "User_name"},
		//[]string{},
	)}

	register(&CoaUsers)
}

func (this *CoaUsersManager) Get(session *mcclient.ClientSession, id string, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	path := fmt.Sprintf("/api/info/user_by_id/%s", id)
	accessToken, err := this.getServiceAuthorizationToken(session)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set(Authorization, "Bearer "+accessToken)
	_, body, err := modulebase.JsonRequest(this.ResourceManager, session, "GET", path, header, params)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, fmt.Errorf("empty response")
	}
	code, err := body.Int(Code)
	if err != nil {
		return nil, err
	}
	message, err := body.GetString(Message)
	if err != nil {
		return nil, err
	}
	if code != 0 || message != "" {
		return nil, fmt.Errorf("error: code %d, message '%s'", code, message)
	}
	data, err := body.Get(Data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (this *CoaUsersManager) GetDepartment(session *mcclient.ClientSession, id string, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	path := fmt.Sprintf("/api/info/departments/%s", id)
	accessToken, err := this.getServiceAuthorizationToken(session)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set(Authorization, "Bearer "+accessToken)
	_, body, err := modulebase.JsonRequest(this.ResourceManager, session, "GET", path, header, params)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, fmt.Errorf("empty response")
	}
	code, err := body.Int(Code)
	if err != nil {
		return nil, err
	}
	message, err := body.GetString(Message)
	if err != nil {
		return nil, err
	}
	if code != 0 || message != "" {
		return nil, fmt.Errorf("error: code %d, message '%s'", code, message)
	}
	data, err := body.Get(Data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (this *CoaUsersManager) SendMarkdownMessage(session *mcclient.ClientSession, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	path := fmt.Sprint("/api/message/markdown")
	accessToken, err := this.getServiceAuthorizationToken(session)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set(Authorization, "Bearer "+accessToken)
	_, body, err := modulebase.JsonRequest(this.ResourceManager, session, "POST", path, header, params)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, fmt.Errorf("empty response")
	}
	code, err := body.Int(Code)
	if err != nil {
		return nil, err
	}
	message, err := body.GetString(Message)
	if err != nil {
		return nil, err
	}
	if code != 0 || message != "success" {
		return nil, fmt.Errorf("error: code %d, message '%s'", code, message)
	}
	data, err := body.Get(Data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (this *CoaUsersManager) getServiceAuthorizationToken(session *mcclient.ClientSession) (string, error) {
	result, err := ServicesV3.GetByName(session, apis.SERVICE_TYPE_COA, nil)
	if err != nil {
		return "", err
	}
	extra, err := result.Get(Extra)
	if err != nil {
		return "", err
	}
	token, err := extra.GetString(AccessToken)
	if err != nil {
		return "", err
	}
	return token, nil
}
