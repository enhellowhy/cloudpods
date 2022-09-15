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
	"yunion.io/x/onecloud/pkg/apis"
)

type BillResourceDetails struct {
	BillId    string  `json:"bill_id"`
	ItemFee   string  `json:"item_fee"`
	ItemRate  float64 `json:"item_rate"`
	ResId     string  `json:"res_id"`
	ResName   string  `json:"res_name"`
	ResType   string  `json:"res_type"`
	StartTime string  `json:"start_time"`
	EndTime   string  `json:"end_time"`
}

type BillResourceListInput struct {
	apis.ResourceBaseListInput
	StartDay int    `json:"start_day"`
	EndDay   int    `json:"end_day"`
	Project  string `json:"project"`
	ResType  string `json:"res_type"`

	// 以资源名称过滤列表
	//ResNames []string `json:"res_name"`
	ResNames string `json:"res_name"`
	//Names []string `json:"name" help:"filter by names"`
}

type BillResourceCreateInput struct {
	apis.ResourceBaseCreateInput

	ResourceId   string `json:"resource_id"`
	ResourceType string `json:"resource_type"`
	ResourceName string `json:"resource_name"`
	Project      string `json:"project"`
	ProjectId    string `json:"project_id"`
	RegionId     string `json:"region_id"`
	Region       string `json:"region"`
	ZoneId       string `json:"zone_id"`
	Zone         string `json:"zone"`
	ProgressId   string `json:"progress_id"`
	AssociateId  string `json:"associate_id"`
	Linked       bool   `json:"linked"`
	Cpu          int    `json:"cpu"`
	Mem          int    `json:"mem"`
	Size         int    `json:"size"`
	Model        string `json:"model"`
	UsageModel   string `json:"usage_model"`
}
