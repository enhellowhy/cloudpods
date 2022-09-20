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

//const (
//	APPLY_MACHINE = "apply-machine"
//	APPLY_SERVER_DELETE = "apply-server-delete"
//)

var (
	WorkflowProcessInstances modulebase.ResourceManager
)

func init() {
	WorkflowProcessInstances = NewWorkflowManager("workflow_process_instance", "workflow_process_instances",
		[]string{"Id", "Name", "Key", "Setting", "Initiator"},
		//[]string{"id", "name", "key", "enabled", "extra_id"},
		[]string{},
	)
	Register(&WorkflowProcessInstances)
}
