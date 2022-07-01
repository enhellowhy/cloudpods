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
	"errors"
	"fmt"
	"net/http"
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/apis"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modulebase"
)

type JumpServerAssetManager struct {
	modulebase.ResourceManager
}

var (
	JsAsset JumpServerAssetManager
)

func init() {
	JsAsset = JumpServerAssetManager{NewJumpServerManager("jsasset", "jsassets",
		[]string{"ID", "Key", "Name", "Value", "Org_id", "Org_name", "Full_value"},
		//[]string{},
		[]string{},
	)}

	register(&JsAsset)
}

func (this *JumpServerAssetManager) List(session *mcclient.ClientSession, params jsonutils.JSONObject) (*modulebase.ListResult, error) {
	path := fmt.Sprintf("/api/v1/assets/nodes/")
	token, err := this.getServiceAuthorizationToken(session)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set(Authorization, token)
	_, body, err := modulebase.JsonRequest(this.ResourceManager, session, "GET", path, header, nil)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, fmt.Errorf("empty response")
	}
	rets, err := body.GetArray()
	if err != nil {
		return nil, err
	}
	nextMarker, _ := body.GetString("next_marker")
	markerField, _ := body.GetString("marker_field")
	markerOrder, _ := body.GetString("marker_order")
	total, _ := body.Int("total")
	limit, _ := body.Int("limit")
	offset, _ := body.Int("offset")
	if len(nextMarker) == 0 && total == 0 {
		total = int64(len(rets))
	}
	return &modulebase.ListResult{
		Data:  rets,
		Total: int(total), Limit: int(limit), Offset: int(offset),
		NextMarker:  nextMarker,
		MarkerField: markerField,
		MarkerOrder: markerOrder,
	}, nil
}

func (this *JumpServerAssetManager) ListUser(session *mcclient.ClientSession, params jsonutils.JSONObject) (*modulebase.ListResult, error) {
	path := fmt.Sprintf("/api/v1/users/users/")
	token, err := this.getServiceAuthorizationToken(session)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set(Authorization, token)
	_, body, err := modulebase.JsonRequest(this.ResourceManager, session, "GET", path, header, nil)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, fmt.Errorf("empty response")
	}
	rets, err := body.GetArray()
	if err != nil {
		return nil, err
	}
	nextMarker, _ := body.GetString("next_marker")
	markerField, _ := body.GetString("marker_field")
	markerOrder, _ := body.GetString("marker_order")
	total, _ := body.Int("total")
	limit, _ := body.Int("limit")
	offset, _ := body.Int("offset")
	if len(nextMarker) == 0 && total == 0 {
		total = int64(len(rets))
	}
	return &modulebase.ListResult{
		Data:  rets,
		Total: int(total), Limit: int(limit), Offset: int(offset),
		NextMarker:  nextMarker,
		MarkerField: markerField,
		MarkerOrder: markerOrder,
	}, nil
}

func (this *JumpServerAssetManager) ListAssetPerms(session *mcclient.ClientSession, params jsonutils.JSONObject) (*modulebase.ListResult, error) {
	path := fmt.Sprintf("/api/v1/perms/asset-permissions/")
	token, err := this.getServiceAuthorizationToken(session)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set(Authorization, token)
	_, body, err := modulebase.JsonRequest(this.ResourceManager, session, "GET", path, header, nil)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, fmt.Errorf("empty response")
	}
	rets, err := body.GetArray()
	if err != nil {
		return nil, err
	}
	nextMarker, _ := body.GetString("next_marker")
	markerField, _ := body.GetString("marker_field")
	markerOrder, _ := body.GetString("marker_order")
	total, _ := body.Int("total")
	limit, _ := body.Int("limit")
	offset, _ := body.Int("offset")
	if len(nextMarker) == 0 && total == 0 {
		total = int64(len(rets))
	}
	return &modulebase.ListResult{
		Data:  rets,
		Total: int(total), Limit: int(limit), Offset: int(offset),
		NextMarker:  nextMarker,
		MarkerField: markerField,
		MarkerOrder: markerOrder,
	}, nil
}

func (this *JumpServerAssetManager) Create(session *mcclient.ClientSession, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	path := fmt.Sprintf("/api/v1/assets/assets/")
	token, err := this.getServiceAuthorizationToken(session)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set(Authorization, token)
	//data := params.(*jsonutils.JSONDict)
	//data.Set("domain_id", jsonutils.NewString(domainId))
	//data.Set("project_id", jsonutils.NewString(projectId))
	//url := newSchedURL("forecast")
	//_, obj, err := modulebase.JsonRequest(this.ResourceManager, s, "POST", url, nil, data)
	_, body, err := modulebase.JsonRequest(this.ResourceManager, session, "POST", path, header, params)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, fmt.Errorf("empty response")
	}
	return body, nil
}

func (this *JumpServerAssetManager) CreateUser(session *mcclient.ClientSession, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	path := fmt.Sprintf("/api/v1/users/users/")
	token, err := this.getServiceAuthorizationToken(session)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set(Authorization, token)
	//data := params.(*jsonutils.JSONDict)
	//data.Set("domain_id", jsonutils.NewString(domainId))
	//data.Set("project_id", jsonutils.NewString(projectId))
	//url := newSchedURL("forecast")
	//_, obj, err := modulebase.JsonRequest(this.ResourceManager, s, "POST", url, nil, data)
	_, body, err := modulebase.JsonRequest(this.ResourceManager, session, "POST", path, header, params)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, fmt.Errorf("empty response")
	}
	return body, nil
}

func (this *JumpServerAssetManager) CreatePerms(session *mcclient.ClientSession, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	path := fmt.Sprintf("/api/v1/perms/asset-permissions/")
	token, err := this.getServiceAuthorizationToken(session)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set(Authorization, token)
	//data := params.(*jsonutils.JSONDict)
	//data.Set("domain_id", jsonutils.NewString(domainId))
	//data.Set("project_id", jsonutils.NewString(projectId))
	//url := newSchedURL("forecast")
	//_, obj, err := modulebase.JsonRequest(this.ResourceManager, s, "POST", url, nil, data)
	_, body, err := modulebase.JsonRequest(this.ResourceManager, session, "POST", path, header, params)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, fmt.Errorf("empty response")
	}
	return body, nil
}

func (this *JumpServerAssetManager) Delete(session *mcclient.ClientSession, id string, body jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	path := fmt.Sprintf("/api/v1/assets/assets/%s/", id)
	token, err := this.getServiceAuthorizationToken(session)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set(Authorization, token)
	_, _, err = modulebase.JsonRequest(this.ResourceManager, session, "DELETE", path, header, nil)
	if err != nil {
		return nil, err
	}
	data := new(jsonutils.JSONDict)
	data.Set("id", jsonutils.NewString(id))
	return data, nil
}

func (this *JumpServerAssetManager) Get(session *mcclient.ClientSession, id string, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	path := fmt.Sprintf("/api/v1/assets/assets/%s/", id)
	token, err := this.getServiceAuthorizationToken(session)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set(Authorization, token)
	_, body, err := modulebase.JsonRequest(this.ResourceManager, session, "GET", path, header, params)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, fmt.Errorf("empty response")
	}
	return body, nil
}

func (this *JumpServerAssetManager) GetByIP(session *mcclient.ClientSession, ip string, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	path := fmt.Sprintf("/api/v1/assets/assets/?ip=%s", ip)
	token, err := this.getServiceAuthorizationToken(session)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set(Authorization, token)
	_, body, err := modulebase.JsonRequest(this.ResourceManager, session, "GET", path, header, params)
	if err != nil {
		return nil, err
	}
	//if body == nil {
	//	return nil, fmt.Errorf("empty response")
	//}
	rets, err := body.GetArray()
	if err != nil {
		return nil, err
	}
	if len(rets) > 1 {
		errStr := fmt.Sprintf("asset for %s is not only.", ip)
		return nil, errors.New(errStr)
	}
	if len(rets) == 0 {
		return nil, nil
	}
	return rets[0], nil
}

func (this *JumpServerAssetManager) GetPermsByName(session *mcclient.ClientSession, name string, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	path := fmt.Sprintf("/api/v1/perms/asset-permissions/?name=%s", name)
	token, err := this.getServiceAuthorizationToken(session)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set(Authorization, token)
	_, body, err := modulebase.JsonRequest(this.ResourceManager, session, "GET", path, header, params)
	if err != nil {
		return nil, err
	}
	rets, err := body.GetArray()
	if err != nil {
		return nil, err
	}
	if len(rets) > 1 {
		errStr := fmt.Sprintf("asset perms for %s is not only.", name)
		return nil, errors.New(errStr)
	}
	if len(rets) == 0 {
		return nil, nil
	}
	return rets[0], nil
}

func (this *JumpServerAssetManager) PatchPerms(session *mcclient.ClientSession, id string, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	path := fmt.Sprintf("/api/v1/perms/asset-permissions/%s/", id)
	token, err := this.getServiceAuthorizationToken(session)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set(Authorization, token)
	//data := params.(*jsonutils.JSONDict)
	//data.Set("domain_id", jsonutils.NewString(domainId))
	//data.Set("project_id", jsonutils.NewString(projectId))
	//url := newSchedURL("forecast")
	//_, obj, err := modulebase.JsonRequest(this.ResourceManager, s, "POST", url, nil, data)
	_, body, err := modulebase.JsonRequest(this.ResourceManager, session, "PATCH", path, header, params)
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, fmt.Errorf("empty response")
	}
	return body, nil
}

func (this *JumpServerAssetManager) GetUser(session *mcclient.ClientSession, email string, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	path := fmt.Sprintf("/api/v1/users/users/?email=%s", email)
	token, err := this.getServiceAuthorizationToken(session)
	if err != nil {
		return nil, err
	}

	header := http.Header{}
	header.Set(Authorization, token)
	_, body, err := modulebase.JsonRequest(this.ResourceManager, session, "GET", path, header, params)
	if err != nil {
		return nil, err
	}
	//if body == nil {
	//	return nil, fmt.Errorf("empty response")
	//}
	rets, err := body.GetArray()
	if err != nil {
		return nil, err
	}
	if len(rets) > 1 {
		errStr := fmt.Sprintf("user for %s is not only.", email)
		return nil, errors.New(errStr)
	}
	//name, _ := rets[0].Get("name")
	//id, _ := rets[0].Get("id")
	//prettyRes := new(jsonutils.JSONDict)
	//prettyRes.Add(jsonutils.NewString(email), "email")
	//prettyRes.Add(name, "name")
	//prettyRes.Add(id, "id")
	//
	//return prettyRes, nil
	if len(rets) == 0 {
		return nil, nil
	}
	return rets[0], nil
}

func (this *JumpServerAssetManager) getServiceAuthorizationToken(session *mcclient.ClientSession) (string, error) {
	result, err := ServicesV3.GetByName(session, apis.SERVICE_TYPE_JUMPSERVER, nil)
	if err != nil {
		return "", err
	}
	extra, err := result.Get(Extra)
	if err != nil {
		return "", err
	}
	token, err := extra.GetString(Authorization)
	if err != nil {
		return "", err
	}
	return token, nil
}
