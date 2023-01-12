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

package workflow

import (
	"time"
	"yunion.io/x/onecloud/pkg/apis"
)

type WorkflowProcessDefineCreateInput struct {
	apis.ModelBaseListInput
	apis.EnabledBaseResourceCreateInput

	Id string `json:"id"`

	// description: config type
	// required: true
	// example: apply-machine
	Type string `json:"type"`
	Key  string `json:"key"`

	// description: config type
	// required: true
	// example: apply-machine
	Name string `json:"name"`
}

type WorkflowProcessInstanceCreateInput struct {
	Variables WorkflowProcessInstanceInput `json:"variables"`
}

type WorkflowProcessInstanceInput struct {
	apis.EnabledStatusStandaloneResourceCreateInput
	apis.ProjectizedResourceCreateInput

	// description: config type
	// required: true
	// example: apply-machine

	Ids                   string `json:"ids"`
	Initiator             string `json:"initiator"`
	ProcessDefinitionKey  string `json:"process_definition_key"`
	Price                 string `json:"price"`
	ServerCreateParameter string `json:"server-create-paramter"`
	Parameter             string `json:"paramter"`
	ServerConf            string `json:"serverConf"`
}

type WorkflowProcessDefineListInput struct {
	apis.ModelBaseListInput

	apis.EnabledResourceBaseListInput
}

type WorkflowProcessInstanceListInput struct {
	apis.EnabledStatusStandaloneResourceListInput
	apis.ProjectizedResourceListInput

	UserId string `json:"user_id"`
}
type Task struct {
	LocalVariables struct {
		Comment string `json:"comment"`
	} `json:"local_variables"`
}

type STask struct {
	ActivityName       string   `json:"activity_name"`
	TaskAssigneeName   []string `json:"task_assignee_name"`
	EndTime            string   `json:"end_time"`
	CommandMessage     string   `json:"command_message"`
	TaskAssigneeAvatar []string `json:"task_assignee_avatar"`
	Task               Task     `json:"task"`
}

type WorkflowProcessInstanceDetails struct {
	//apis.EnabledStatusStandaloneResourceDetails
	apis.ResourceBaseDetails

	StartTime             time.Time `json:"start_time"`
	EndTime               time.Time `json:"end_time"`
	ProcessDefinitionName string    `json:"process_definition_name"`
	ProcessDefinitionKey  string    `json:"process_definition_key"`
	Log                   []*STask  `json:"log"`

	Metadata  map[string]string                       `json:"metadata"`
	Variables WorkflowProcessInstanceVariablesDetails `json:"variables"`
}

type WorkflowProcessInstanceVariablesDetails struct {
	Ids                   string `json:"ids"`
	Initiator             string `json:"initiator"`
	InitiatorName         string `json:"initiator_name"`
	AuditStatus           string `json:"audit_status"`
	ResourceProjectId     string `json:"resource_project_id"`
	ResourceProjectName   string `json:"resource_project_name"`
	Parameter             string `json:"paramter"`
	ServerConf            string `json:"serverConf"`
	ServerCreateParameter string `json:"server-create-paramter"`
}
