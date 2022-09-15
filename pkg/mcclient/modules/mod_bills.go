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

type BillManager struct {
	modulebase.ResourceManager
}

var (
	Bills BillManager
)

func init() {
	Bills = BillManager{NewMeterManager("bill_detail", "bill_details",
		[]string{},
		[]string{},
	)}
	register(&Bills)
}

//func (this *BillManager) GetResources(session *mcclient.ClientSession, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
//	rates, err := this.ListInContexts(session, params, nil)
//	if err != nil {
//		return nil, err
//	}
//	if rates.Total == 0 {
//		//return nil, nil
//		return nil, fmt.Errorf("no find effective res")
//	}
//
//	var untilDate string
//	flag := false
//	for i := range rates.Data {
//		val, until, find, err := rateReadFilter(rates.Data[i], untilDate, flag, params)
//		if err == nil {
//			rates.Data[i] = val
//		} else {
//			log.Warningf("rateReadFilter fail for %s: %s", rates.Data[i], err)
//		}
//		untilDate = until
//		if find {
//			return val, nil
//		}
//	}
//
//	return nil, fmt.Errorf("no find effective res")
//}

func (this *BillManager) GetTotal(session *mcclient.ClientSession, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	total, err := this.PerformClassAction(session, "total", params)
	return total, err
}
