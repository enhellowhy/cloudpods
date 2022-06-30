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

import "yunion.io/x/onecloud/pkg/mcclient/modulebase"

const (
	APPLY_MACHINE             = "apply-machine"
	APPLY_BUCKET              = "apply-bucket"
	APPLY_LOADBALANCER        = "apply-loadbalancer"
	APPLY_SERVER_CHANGECONFIG = "apply-server-changeconfig"
	APPLY_SERVER_DELETE       = "apply-server-delete"
)

var (
	WorkflowProcessDefinitions    modulebase.ResourceManager
	WorkflowProcessDefinitionsMap = map[string]string{
		APPLY_MACHINE:      "主机申请",
		APPLY_BUCKET:       "存储桶申请",
		APPLY_LOADBALANCER: "负载均衡申请",
		//APPLY_SERVER_CHANGECONFIG: "主机变配",
		APPLY_SERVER_CHANGECONFIG: "主机调整配置",
		APPLY_SERVER_DELETE:       "主机删除",
	}
)

func init() {
	WorkflowProcessDefinitions = NewWorkflowManager("workflow_process_definition", "workflow_process_definitions",
		[]string{"Id", "Name", "Key", "Enabled", "ExtraId"},
		//[]string{"id", "name", "key", "enabled", "extra_id"},
		[]string{},
	)
	register(&WorkflowProcessDefinitions)
}
