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
	"strconv"
	"strings"
	"time"
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modulebase"
)

type RateManager struct {
	modulebase.ResourceManager
}

var (
	Rates RateManager
)

func init() {
	Rates = RateManager{NewMeterManager("rate", "rates",
		[]string{"Id", "Model", "Brand"},
		[]string{},
	)}
	Register(&Rates)
}

const (
	Date_FORMAT = "2006-01-02TZ"
	TIME_FORMAT = "2006-01-02T15:04:05.000000Z"

	// Action
	ACTION_QUERY_HSITORY = "queryhistory"
	ACTION_QUERY_GROUP   = "querygroup"

	RES_TYPE_VM        = "vm"
	RES_TYPE_BAREMETAL = "baremetal"
	RES_TYPE_GPU       = "gpu"
	RES_TYPE_CPU       = "cpu"
	RES_TYPE_MEM       = "mem"
	RES_TYPE_DISK      = "disk"
	RES_TYPE_EIP       = "eip"
	FLAG_YES           = "YES"
	FLAG_NO            = "NO"

	CPU_MODEL  = "g1,c1"
	DISK_MODEL = "rotate::local,ssd::local,ssd::rbd,rotate::rbd,hybrid::rbd"
)

// no order
//var resMap = map[string]string{
//	"cpu":  "g1,c1",
//	"mem":  "",
//	"disk": "rotate::local,ssd::local,ssd::rbd,rotate::rbd,hybrid::rbd",
//}

func (this *RateManager) List(session *mcclient.ClientSession, params jsonutils.JSONObject) (*modulebase.ListResult, error) {
	//return this.ListInContexts(session, params, nil)
	action, _ := params.GetString("action")
	if action == ACTION_QUERY_GROUP {
		return this.findRateGroup(session, params)
	} else if action == ACTION_QUERY_HSITORY {
		return this.findRateHistory(session, params)
	} else {
		return nil, fmt.Errorf("no action")
	}
}

func (this *RateManager) findRateGroup(session *mcclient.ClientSession, params jsonutils.JSONObject) (*modulebase.ListResult, error) {
	groups := new(modulebase.ListResult)

	res, _ := params.GetString("res_type")
	switch res {
	case RES_TYPE_BAREMETAL, RES_TYPE_GPU:
		rates, err := this.ListInContexts(session, params, nil)
		if err != nil {
			return nil, err
		}
		if rates.Total == 0 {
			return rates, nil
		}
		params.(*jsonutils.JSONDict).Set("action", jsonutils.NewString(ACTION_QUERY_HSITORY))
		for _, rate := range rates.Data {
			model, _ := rate.GetString("model")
			//params.(*jsonutils.JSONDict).Set("res_type", jsonutils.NewString(RES_TYPE_BAREMETAL))
			params.(*jsonutils.JSONDict).Set("model", jsonutils.NewString(model))
			r, err := this.GetEffectiveRes(session, params)
			if err != nil {
				log.Errorf("get baremetal GetEffectiveRes %v", err)
				continue
			}
			groups.Data = append(groups.Data, r)
		}
		//return rates, nil
	case RES_TYPE_VM:
		//RES_TYPE_CPU
		for _, model := range strings.Split(CPU_MODEL, ",") {
			params.(*jsonutils.JSONDict).Set("res_type", jsonutils.NewString(RES_TYPE_CPU))
			params.(*jsonutils.JSONDict).Set("model", jsonutils.NewString(model))
			cpu, err := this.GetEffectiveRes(session, params)
			if err != nil {
				log.Errorf("cpu %v", err)
				continue
			}
			groups.Data = append(groups.Data, cpu)
		}
		//RES_TYPE_MEM:
		params.(*jsonutils.JSONDict).Set("res_type", jsonutils.NewString(RES_TYPE_MEM))
		params.(*jsonutils.JSONDict).Set("model", jsonutils.NewString(""))
		mem, err := this.GetEffectiveRes(session, params)
		if err != nil {
			log.Errorf("mem %v", err)
		} else {
			groups.Data = append(groups.Data, mem)
		}
		//RES_TYPE_DISK:
		for _, model := range strings.Split(DISK_MODEL, ",") {
			params.(*jsonutils.JSONDict).Set("res_type", jsonutils.NewString(RES_TYPE_DISK))
			params.(*jsonutils.JSONDict).Set("model", jsonutils.NewString(model))
			disk, err := this.GetEffectiveRes(session, params)
			if err != nil {
				log.Errorf("disk %v", err)
				continue
			}
			groups.Data = append(groups.Data, disk)
		}
	}
	groups.Total = len(groups.Data)
	return groups, nil
}

func (this *RateManager) GetEffectiveRes(session *mcclient.ClientSession, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	rates, err := this.ListInContexts(session, params, nil)
	if err != nil {
		return nil, err
	}
	if rates.Total == 0 {
		//return nil, nil
		return nil, fmt.Errorf("no find effective res")
	}

	var untilDate string
	flag := false
	for i := range rates.Data {
		val, until, find, err := rateReadFilter(rates.Data[i], untilDate, flag, params)
		if err == nil {
			rates.Data[i] = val
		} else {
			log.Warningf("rateReadFilter fail for %s: %s", rates.Data[i], err)
		}
		untilDate = until
		if find {
			return val, nil
		}
	}

	return nil, fmt.Errorf("no find effective res")
}

func (this *RateManager) findRateHistory(session *mcclient.ClientSession, params jsonutils.JSONObject) (*modulebase.ListResult, error) {
	rates, err := this.ListInContexts(session, params, nil)
	if err != nil {
		return nil, err
	}
	if rates.Total == 0 {
		return rates, nil
	}

	var untilDate string
	flag := false

	// order by enable_time desc
	for i := range rates.Data {
		val, until, find, err := rateReadFilter(rates.Data[i], untilDate, flag, params)
		if err == nil {
			rates.Data[i] = val
		} else {
			log.Warningf("rateReadFilter fail for %s: %s", rates.Data[i], err)
		}
		untilDate = until
		if find {
			flag = true
		}
	}
	return rates, nil
}

func rateReadFilter(s jsonutils.JSONObject, untilDate string, flag bool, query jsonutils.JSONObject) (jsonutils.JSONObject, string, bool, error) {
	ss := s.(*jsonutils.JSONDict)
	ret := ss.CopyIncludes("id", "model", "price")
	ret.Add(jsonutils.NewString("day"), "duration")
	//enableTime, _ := s.GetString("enable_time")
	//t, _ := time.Parse(TIME_FORMAT, enableTime)
	t, _ := s.GetTime("enable_time")
	ret.Add(jsonutils.NewString(t.Format(Date_FORMAT)), "effective_date")
	if untilDate != "" {
		ret.Add(jsonutils.NewString(untilDate), "until_date")
	}
	untilDate = t.Add(-24 * time.Hour).Format(Date_FORMAT)

	// res_type
	resType, _ := s.GetString("resource_type")
	ret.Add(jsonutils.NewString(resType), "res_type")

	//unit
	var unit string
	switch resType {
	case RES_TYPE_CPU:
		unit = "1core"
	case RES_TYPE_MEM:
		unit = "1GB"
	case RES_TYPE_EIP:
		unit = "Mbps"
	case RES_TYPE_DISK:
		unit = "1GB"
		models, _ := s.GetString("model")
		ret.Add(jsonutils.NewString(strings.Split(models, "::")[0]), "medium_type")
		ret.Add(jsonutils.NewString(strings.Split(models, "::")[1]), "storage_type")
	default:
		unit = ""
	}
	ret.Add(jsonutils.NewString(unit), "unit")

	//flag
	find := false
	if flag {
		ret.Add(jsonutils.NewString(FLAG_NO), "effective_flag")
	} else {
		if t.After(time.Now()) {
			ret.Add(jsonutils.NewString(FLAG_NO), "effective_flag")
		} else {
			// 因为时间倒序，所以不需要做其他时间段检查
			ret.Add(jsonutils.NewString(FLAG_YES), "effective_flag")
			find = true
		}
	}

	// price
	price, _ := s.Float("price")
	ret.Add(jsonutils.NewString(strconv.FormatFloat(price*24, 'f', 6, 64)), "rate_text")

	return ret, untilDate, find, nil
}
