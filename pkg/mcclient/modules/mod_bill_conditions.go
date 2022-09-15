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
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modulebase"
)

type BillConditionManager struct {
	modulebase.ResourceManager
}

var (
	BillConditions BillConditionManager
)

func init() {
	BillConditions = BillConditionManager{NewMeterManager("bill_condition", "bill_conditions",
		[]string{},
		[]string{},
	)}
	register(&BillConditions)
}

func (this *BillConditionManager) List(session *mcclient.ClientSession, params jsonutils.JSONObject) (*modulebase.ListResult, error) {
	result := new(modulebase.ListResult)
	result.Total = 1

	data := jsonutils.NewDict()
	data.Set("item_id", jsonutils.NewString("CNY"))
	data.Set("item_name", jsonutils.NewString("CNY"))
	data.Set("cost_conversion_origin", jsonutils.NewBool(false))
	data.Set("exchange_rate_available", jsonutils.NewBool(false))

	result.Data = append(result.Data, data)
	return result, nil
}
