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
	"strings"
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modulebase"
)

type BillAnalysisManager struct {
	modulebase.ResourceManager
}

var (
	BillAnalysises BillAnalysisManager
)

func init() {
	BillAnalysises = BillAnalysisManager{NewMeterManager("bill_analysis", "bill_analysises",
		[]string{},
		[]string{},
	)}
	register(&BillAnalysises)
}

func (this *BillAnalysisManager) List(session *mcclient.ClientSession, params jsonutils.JSONObject) (*modulebase.ListResult, error) {
	result := new(modulebase.ListResult)
	dict := params.(*jsonutils.JSONDict)
	t, _ := dict.GetString("query_type")
	if t == "" {
		return nil, httperrors.ErrActionNotFound
	}
	t = strings.ReplaceAll(t, "_", "-")
	//dict.Add(jsonutils.NewBool(true), "need_group")
	r, err := Bills.Get(session, t, dict)
	if err != nil {
		return nil, err
	}
	var d []jsonutils.JSONObject
	err = r.Unmarshal(&d)
	if err != nil {
		return nil, err
	}
	result.Data = d
	result.Total = len(d)
	return result, nil
}

func (this *BillAnalysisManager) Get(session *mcclient.ClientSession, id string, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	dict := params.(*jsonutils.JSONDict)
	//dict.Add(jsonutils.NewString("instances"), "query_group")
	return Bills.Get(session, id, dict)
}
