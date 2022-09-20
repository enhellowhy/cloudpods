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
	"github.com/pkg/errors"
	"math"
	"strconv"
	"strings"
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modulebase"
)

const (
	SEPARATOR_COMMA      = ","
	SEPARATOR_SEMICOLON  = ";"
	SEPARATOR_EQUAL_SIGN = "="
)

type PriceInfoManager struct {
	modulebase.ResourceManager
}

var (
	PriceInfos PriceInfoManager
)

func init() {
	PriceInfos = PriceInfoManager{NewMeterManager("price_info", "price_infos",
		[]string{},
		[]string{},
	)}
	Register(&PriceInfos)
}

func (this *PriceInfoManager) List(session *mcclient.ClientSession, params jsonutils.JSONObject) (*modulebase.ListResult, error) {
	priceResult := new(modulebase.ListResult)
	priceResult.Total = 1
	data := jsonutils.NewDict()

	//cpu=2,model=g1;mem=2;disk=10,model=hybrid::rbd-hybrid;disk=0,model=::
	spec, err := params.GetString("spec")
	if err != nil {
		return nil, errors.Wrap(err, "params spec")
	}
	resType, err := params.GetString("res_type")
	if err != nil {
		return nil, errors.Wrap(err, "params res_type")
	}
	//RES_TYPE_BAREMETAL
	if resType == RES_TYPE_BAREMETAL {
		//cpu:64/mem:128473M/manufacture:Powerleader/model:PR1710P
		dict := jsonutils.NewDict()
		dict.Set("scope", jsonutils.NewString("system"))
		dict.Set("action", jsonutils.NewString(ACTION_QUERY_HSITORY))
		dict.Set("res_type", jsonutils.NewString(RES_TYPE_BAREMETAL))
		dict.Set("model", jsonutils.NewString(spec))
		baremetalRate, err := Rates.GetEffectiveRes(session, dict)
		if err != nil {
			return nil, errors.Wrap(err, "get baremetal rates err")
		}
		baremetalPrice, _ := baremetalRate.Float("price")
		baremetalHourPrice := round(baremetalPrice, 6)

		data.Set("currency", jsonutils.NewString("CNY"))
		data.Set("discount", jsonutils.NewInt(0))
		data.Set("hour_gross_price", jsonutils.NewFloat64(baremetalHourPrice))
		data.Set("hour_price", jsonutils.NewFloat64(baremetalHourPrice))
		data.Set("sum_price", jsonutils.NewFloat64(baremetalHourPrice))
		//data.Set("month_gross_price", jsonutils.NewFloat64(hourPrice*24*30))
		//data.Set("month_price", jsonutils.NewFloat64(hourPrice*24*30))

		priceResult.Data = append(priceResult.Data, data)
		return priceResult, nil
	}
	resList := strings.Split(spec, SEPARATOR_SEMICOLON)
	if len(resList) != 4 {
		return nil, fmt.Errorf("spec params info dismatched")
	}

	dict := jsonutils.NewDict()
	dict.Set("scope", jsonutils.NewString("system"))
	dict.Set("action", jsonutils.NewString(ACTION_QUERY_HSITORY))

	//RES_TYPE_CPU
	cpuSpec := resList[0]
	cpuSpecList := strings.Split(cpuSpec, SEPARATOR_COMMA)
	core, _ := strconv.Atoi(strings.Split(cpuSpecList[0], SEPARATOR_EQUAL_SIGN)[1])
	cpuModel := strings.Split(cpuSpecList[1], SEPARATOR_EQUAL_SIGN)[1]

	dict.Set("res_type", jsonutils.NewString(RES_TYPE_CPU))
	dict.Set("model", jsonutils.NewString(cpuModel))
	cpuRate, err := Rates.GetEffectiveRes(session, dict)
	if err != nil {
		return nil, errors.Wrap(err, "get cpu rates err")
	}
	cpuPrice, _ := cpuRate.Float("price")
	cpuHourPrice := round(cpuPrice*float64(core), 6)

	//RES_TYPE_MEM:
	memSpec := resList[1]
	memSize, _ := strconv.Atoi(strings.Split(memSpec, SEPARATOR_EQUAL_SIGN)[1])

	dict.Set("res_type", jsonutils.NewString(RES_TYPE_MEM))
	dict.Set("model", jsonutils.NewString(""))
	memRate, err := Rates.GetEffectiveRes(session, dict)
	if err != nil {
		return nil, errors.Wrap(err, "get mem rates err")
	}
	memPrice, _ := memRate.Float("price")
	memHourPrice := round(memPrice*float64(memSize), 6)

	// system disk
	sysDiskSpec := resList[2]
	sysDiskSpecList := strings.Split(sysDiskSpec, SEPARATOR_COMMA)
	sysDiskSize, _ := strconv.Atoi(strings.Split(sysDiskSpecList[0], SEPARATOR_EQUAL_SIGN)[1])
	sysDiskModel := strings.Split(sysDiskSpecList[1], SEPARATOR_EQUAL_SIGN)[1]
	var sysDiskPrice, sysDiskHourPrice float64
	if sysDiskSize == 0 {
		sysDiskPrice, sysDiskHourPrice = 0.00, 0.00
	} else {
		dict.Set("res_type", jsonutils.NewString(RES_TYPE_DISK))
		dict.Set("model", jsonutils.NewString(sysDiskModel))
		sysDiskRate, err := Rates.GetEffectiveRes(session, dict)
		if err != nil {
			return nil, errors.Wrap(err, "get sys disk rates err")
		}
		sysDiskPrice, _ = sysDiskRate.Float("price")
		sysDiskHourPrice = round(sysDiskPrice*float64(sysDiskSize), 6)
	}

	// data disk
	dataDiskSpec := resList[3]
	dataDiskSpecList := strings.Split(dataDiskSpec, SEPARATOR_COMMA)
	dataDiskSize, _ := strconv.Atoi(strings.Split(dataDiskSpecList[0], SEPARATOR_EQUAL_SIGN)[1])
	dataDiskModel := strings.Split(dataDiskSpecList[1], SEPARATOR_EQUAL_SIGN)[1]

	var dataDiskPrice, dataDiskHourPrice float64
	if dataDiskSize == 0 {
		dataDiskPrice, dataDiskHourPrice = 0.00, 0.00
	} else {
		if dataDiskModel == sysDiskModel {
			dataDiskPrice = sysDiskPrice
			dataDiskHourPrice = round(dataDiskPrice*float64(dataDiskSize), 6)
		} else {
			dict.Set("res_type", jsonutils.NewString(RES_TYPE_DISK))
			dict.Set("model", jsonutils.NewString(dataDiskModel))
			dataDiskRate, err := Rates.GetEffectiveRes(session, dict)
			if err != nil {
				return nil, errors.Wrap(err, "get data disk rates err")
			}
			dataDiskPrice, _ = dataDiskRate.Float("price")
			dataDiskHourPrice = round(dataDiskPrice*float64(dataDiskSize), 6)
		}
	}

	// sum
	instanceHourPrice := round(cpuHourPrice+memHourPrice, 6)
	hourPrice := round(instanceHourPrice+sysDiskHourPrice+dataDiskHourPrice, 6)

	data.Set("currency", jsonutils.NewString("CNY"))
	data.Set("discount", jsonutils.NewInt(0))
	data.Set("hour_gross_price", jsonutils.NewFloat64(hourPrice))
	data.Set("hour_price", jsonutils.NewFloat64(hourPrice))
	data.Set("sum_price", jsonutils.NewFloat64(hourPrice))
	data.Set("instance_hour_gross_price", jsonutils.NewFloat64(instanceHourPrice))
	data.Set("instance_hour_price", jsonutils.NewFloat64(instanceHourPrice))
	data.Set("sys_disk_hour_gross_price", jsonutils.NewFloat64(sysDiskHourPrice))
	data.Set("sys_disk_hour_price", jsonutils.NewFloat64(sysDiskHourPrice))
	data.Set("data_disk_hour_gross_price", jsonutils.NewFloat64(dataDiskHourPrice))
	data.Set("data_disk_hour_price", jsonutils.NewFloat64(dataDiskHourPrice))
	//data.Set("month_gross_price", jsonutils.NewFloat64(hourPrice*24*30))
	//data.Set("month_price", jsonutils.NewFloat64(hourPrice*24*30))

	priceResult.Data = append(priceResult.Data, data)
	return priceResult, nil
}

func round(f float64, n int) float64 {
	pow10_n := math.Pow10(n)
	return math.Trunc((f+0.5/pow10_n)*pow10_n) / pow10_n
}
