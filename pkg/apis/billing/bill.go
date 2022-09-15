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
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/apis"
)

type BillDetails struct {
	BillId      string                 `json:"bill_id"`
	ItemFee     string                 `json:"item_fee"`
	ItemRate    float64                `json:"item_rate"`
	ResId       string                 `json:"res_id"`
	ResName     string                 `json:"res_name"`
	ResType     string                 `json:"res_type"`
	StartTime   string                 `json:"start_time"`
	EndTime     string                 `json:"end_time"`
	TotalUsage  float64                `json:"total_usage"`
	TotalAmount float64                `json:"total_amount"`
	Content     []jsonutils.JSONObject `json:"content"`
}

type BillListInput struct {
	apis.ResourceBaseListInput
	apis.ProjectizedResourceListInput

	//Project string `json:"project"`
	ResType string `json:"res_type"`
	Brand   string `json:"brand"`

	// 以资源名称过滤列表
	//ResNames []string `json:"res_name"`
	ResNames string `json:"res_name"`
	//Names []string `json:"name" help:"filter by names"`

	// bill_resources
	StartDay   int    `json:"start_day"`
	EndDay     int    `json:"end_day"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
	QueryGroup string `json:"query_group"`
	QueryType  string `json:"query_type"`
	DataType   string `json:"data_type"`
	NeedGroup  bool   `json:"need_group"`
}

type BillCreateInput struct {
	apis.ResourceBaseCreateInput

	ResourceType string  `json:"resource_type"`
	Rate         float64 `json:"rate"`
	GrossAmount  float64 `json:"gross_amount"`
	Amount       float64 `json:"amount"`
	PriceUnit    string  `json:"price_unit"`
	Usage        float64 `json:"usage"`
	UsageType    string  `json:"usage_type"`
	ResourceId   string  `json:"resource_id"`
	ResourceName string  `json:"resource_name"`
	Project      string  `json:"project"`
	ProjectId    string  `json:"project_id"`
	RegionId     string  `json:"region_id"`
	Region       string  `json:"region"`
	ZoneId       string  `json:"zone_id"`
	Zone         string  `json:"zone"`
	Spec         string  `json:"spec"`
	ProgressId   string  `json:"progress_id"`
	AssociateId  string  `json:"associate_id"`
	Linked       bool    `json:"linked"`
}
