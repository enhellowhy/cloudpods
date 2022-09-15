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

package billing

import (
	"time"
	"yunion.io/x/onecloud/pkg/apis"
)

type RateDetails struct {
	Id            string `json:"id"`
	EffectiveDate string `json:"effective_date"`
	EffectiveFlag string `json:"effective_flag"`
	Duration      string `json:"duration"`
	Model         string `json:"model"`
	RateText      string `json:"rate_text"`
	ResType       string `json:"res_type"`
	Unit          string `json:"unit"`
	UntilDate     string `json:"until_date"`
}

type RateListInput struct {
	apis.ResourceBaseListInput
	Action  string `json:"action"`
	ResType string `json:"res_type"`
	Model   string `json:"model"`
	//NeedGroup bool   `json:"need_group"`
}

type RateCreateInput struct {
	apis.ResourceBaseCreateInput

	EffectiveDate string `json:"effective_date"`
	Duration      string `json:"duration"`
	Model         string `json:"model"`
	StorageType   string `json:"storage_type"`
	MediumType    string `json:"medium_type"`
	RateText      string `json:"rate_text"`
	ResType       string `json:"res_type"`
	RequestType   string `json:"request_type"`
	Unit          string `json:"unit"`
	CpuCoreCount  int    `json:"cpu_core_count"`
	Manufacture   string `json:"manufacture"`
	MemorySizeMb  int    `json:"memory_size_mb"`
}

type RateUpdateInput struct {
	apis.ResourceBaseUpdateInput
	EnableTime    time.Time `json:"enable_time"`
	EffectiveDate string    `json:"effective_date"`
	EffectiveFlag string    `json:"effective_flag"`
	//Duration      string  `json:"duration"`
	//Model         string  `json:"model"`
	Brand       string  `json:"brand"`
	StorageType string  `json:"storage_type"`
	MediumType  string  `json:"medium_type"`
	RateText    string  `json:"rate_text"`
	ResType     string  `json:"res_type"`
	Price       float64 `json:"price"`
	Unit        string  `json:"unit"`
}
