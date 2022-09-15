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

type BillBalanceManager struct {
	modulebase.ResourceManager
}

var (
	BillBalances BillBalanceManager
)

func init() {
	BillBalances = BillBalanceManager{NewMeterManager("bill_balance", "bill_balances",
		[]string{},
		[]string{},
	)}
	register(&BillBalances)
}

func (this *BillBalanceManager) Get(session *mcclient.ClientSession, id string, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	dict := params.(*jsonutils.JSONDict)
	//dict.Add(jsonutils.NewString("instances"), "query_group")
	return Bills.Get(session, id, dict)
}
