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

package thirdparty

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/apis"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modulebase"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
	"yunion.io/x/onecloud/pkg/mcclient/modules/identity"
	"yunion.io/x/pkg/util/timeutils"
)

type BpmProcessManager struct {
	modulebase.ResourceManager
}

type BizParams struct {
	ProcessInstanceId string `json:"process_instance_id"`
}

var (
	BpmProcess BpmProcessManager
)

func init() {
	BpmProcess = BpmProcessManager{modules.NewBpmManager("bpm", "bpms",
		//[]string{"Id", "Name", "Key", "Enabled", "ExtraId"},
		[]string{},
		[]string{},
	)}
	modulebase.Register(&BpmProcess)
}

func (this *BpmProcessManager) SubmitProcess(session *mcclient.ClientSession, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	path := fmt.Sprint("/apps/api/v1/openapi/process/submitProcess")
	extra, err := this.getServiceExtra(session)
	if err != nil {
		return nil, err
	}
	appId, err := extra.GetString(modules.AppId)
	if err != nil {
		return nil, err
	}
	secretKey, err := extra.GetString(modules.SecretKey)
	if err != nil {
		return nil, err
	}

	var timeStr = strconv.FormatInt(time.Now().Unix()*1000, 10)
	var signature = md5.Sum([]byte(secretKey + timeStr + params.String()))

	header := http.Header{}
	header.Set("Content-Type", "application/json")
	header.Set("tenantid", "li")
	header.Set("timestamp", timeStr)
	header.Set("appid", appId)
	header.Set("signature", fmt.Sprintf("%x", signature))

	_, body, err := modulebase.JsonRequest(this.ResourceManager, session, "POST", path, header, params)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (this *BpmProcessManager) GetProcessHistorys(session *mcclient.ClientSession, id string) (jsonutils.JSONObject, error) {
	path := fmt.Sprint("/ego_api/api/v1/common/getTaskProcess")
	extra, err := this.getServiceExtra(session)
	if err != nil {
		return nil, err
	}
	appId, err := extra.GetString(modules.AppId)
	if err != nil {
		return nil, err
	}
	secretKey, err := extra.GetString(modules.SecretKey)
	if err != nil {
		return nil, err
	}

	bizParams := BizParams{
		ProcessInstanceId: id,
	}

	body := jsonutils.NewDict()
	body.Set(modules.AppId, jsonutils.NewString(appId))
	body.Set("sign_type", jsonutils.NewString("md5"))
	body.Set("tenant_id", jsonutils.NewString("li"))
	body.Set("biz_params", jsonutils.NewString(jsonutils.Marshal(bizParams).String()))

	var timeStr = time.Now().Format(timeutils.CompactTimeFormat)
	var signature = md5.Sum([]byte(secretKey + timeStr + jsonutils.Marshal(bizParams).String()))
	body.Set("timestamp", jsonutils.NewString(timeStr))
	body.Set("sign", jsonutils.NewString(fmt.Sprintf("%x", signature)))

	header := http.Header{}
	header.Set("Content-Type", "application/json")
	header.Set("tenantid", "li")
	//header.Set("timestamp", timeStr)
	//header.Set("appid", appId)
	//header.Set("signature", fmt.Sprintf("%x", signature))

	//bpmUrl := "https://bpm-test.lixiangoa.com/ego_api/api/v1/common/getTaskProcess"
	//httpclient := httputils.GetDefaultClient()
	//_, resp, err := httputils.JSONRequest(httpclient, context.Background(), httputils.POST, bpmUrl, header, body, true)
	_, resp, err := modulebase.JsonRequest(this.ResourceManager, session, "POST", path, header, body)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("empty response")
	}
	code, err := resp.GetString(modules.Code)
	if err != nil {
		return nil, err
	}
	//message, err := resp.GetString(Message)
	//if err != nil {
	//	return nil, err
	//}
	if code != "000000" {
		return nil, fmt.Errorf("error: code %d", code)
	}
	data, err := resp.Get(modules.Data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

type ProcessList struct {
	DoneList []*TaskProcess `json:"done_list"`
	TodoList []*TaskProcess `json:"todo_list"`
}

type TaskProcess struct {
	AssigneeList   []*Assignee `json:"assignee_list"`
	CommandMessage string      `json:"command_message"`
	CreateTime     string      `json:"create_time"`
	CreateStamp    int64       `json:"create_stamp"`
	EndTime        string      `json:"end_time"`
	NodeName       locale      `json:"node_name"`
	TaskComment    string      `json:"task_comment"`
}

type Assignee struct {
	Id           string `json:"id"`
	Name         locale `json:"name"`
	NameZh       string `json:"name_zh"`
	EmployeeName string `json:"employee_name"`
	Email        string `json:"email"`
	HeadImage    string `json:"head_image"`
}

type locale struct {
	Zh string `json:"zh"`
	Ja string `json:"ja"`
	En string `json:"en"`
}

type processList []*TaskProcess

func (a processList) Len() int           { return len(a) }
func (a processList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a processList) Less(i, j int) bool { return a[i].CreateStamp < a[j].CreateStamp }

func (this *BpmProcessManager) GetProcessList(session *mcclient.ClientSession, id string) (*ProcessList, error) {
	data, err := this.GetProcessHistorys(session, id)
	if err != nil {
		return nil, err
	}
	l := new(ProcessList)
	err = data.Unmarshal(l)
	if err != nil {
		return nil, err
	}
	sort.Sort(processList(l.DoneList))
	sort.Sort(processList(l.TodoList))

	return l, nil
}

func (this *BpmProcessManager) getServiceExtra(session *mcclient.ClientSession) (jsonutils.JSONObject, error) {
	result, err := identity.ServicesV3.GetByName(session, apis.SERVICE_TYPE_BPM, nil)
	if err != nil {
		return nil, err
	}
	extra, err := result.Get(modules.Extra)
	if err != nil {
		return nil, err
	}
	return extra, nil
}
