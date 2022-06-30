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

package models

import (
	"context"
	"time"
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/apis"
	api "yunion.io/x/onecloud/pkg/apis/workflow"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
	"yunion.io/x/onecloud/pkg/workflow/options"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/util/stringutils"
	"yunion.io/x/sqlchemy"
)

const (
	//instance state
	EXTERNALLY_TERMINATED = "EXTERNALLY_TERMINATED"
	PENDING               = "PENDING"
	IN_PROCESS            = "IN_PROCESS"
	CREATING              = "CREATING"
	CHANGING              = "CHANGING"
	COMPLETED             = "COMPLETED"
	INIT                  = "INIT"
	SUBMIT_SUCCESS        = "SUBMIT_SUCCESS"
	SUBMIT_FAIL           = "SUBMIT_FAIL"
	SUBMITTING            = "SUBMITTING"

	// AuditStatus
	AUDIT_APPROVED = "approved"
	AUDIT_REFUSED  = "refused"
)

// +onecloud:swagger-gen-ignore
type SWorkflowProcessInstanceManager struct {
	//db.SModelBaseManager
	db.SEnabledStatusStandaloneResourceBaseManager
	db.SProjectizedResourceBaseManager
}

var WorkflowProcessInstanceManager *SWorkflowProcessInstanceManager

func init() {
	WorkflowProcessInstanceManager = &SWorkflowProcessInstanceManager{
		SEnabledStatusStandaloneResourceBaseManager: db.NewEnabledStatusStandaloneResourceBaseManager(
			SWorkflowProcessInstance{},
			"workflow_process_instance",
			"workflow_process_instance",
			"workflow_process_instances",
		),
	}
	WorkflowProcessInstanceManager.SetVirtualObject(WorkflowProcessInstanceManager)
}

/*
+--------------------+--------------+------+-----+---------+-------+
| Field              | Type         | Null | Key | Default | Extra |
+--------------------+--------------+------+-----+---------+-------+
| created_at         | datetime     | NO   | MUL | NULL    |       |
| updated_at         | datetime     | NO   |     | NULL    |       |
| update_version     | int(11)      | NO   |     | 0       |       |
| deleted_at         | datetime     | YES  |     | NULL    |       |
| deleted            | tinyint(1)   | NO   |     | 0       |       |
| id                 | varchar(128) | NO   | PRI | NULL    |       |
| external_id        | varchar(128) | NO   |     | NULL    |       |
| name               | varchar(128) | NO   | MUL | NULL    |       |
| initiator          | varchar(128) | NO   | MUL | NULL    |       |
| type               | varchar(128) | NO   |     | NULL    |       |
| key                | varchar(64)  | NO   |     | NULL    |       |
| setting            | text         | YES  |     | NULL    |       |
| description        | varchar(256) | YES  |     | NULL    |       |
| status             | varchar(36)  | NO   |     | init    |       |
| domain_id          | varchar(64)  | NO   | MUL | default |       |
| tenant_id          | varchar(128) | NO   | MUL | NULL    |       |
| pending_deleted_at | datetime     | YES  |     | NULL    |       |
| pending_deleted    | tinyint(1)   | NO   | MUL | 0       |       |
| enabled            | tinyint(1)   | NO   |     | 0       |       |
+--------------------+--------------+------+-----+---------+-------+
*/

type SWorkflowProcessInstance struct {
	db.SEnabledStatusStandaloneResourceBase
	db.SProjectizedResourceBase

	Ids          string `length:"text" nullable:"true" list:"user" update:"admin" json:"ids"`
	ExternalId   string `width:"128" charset:"utf8" nullable:"false" list:"user" create:"domain_optional" update:"admin" json:"external_id"`
	Initiator    string `width:"128" charset:"utf8" nullable:"false" index:"true" list:"user" create:"domain_optional" update:"admin" json:"initiator"`
	Type         string `width:"128" charset:"utf8" nullable:"false" list:"user" create:"domain_optional" update:"admin" json:"type"`
	Key          string `width:"64" charset:"utf8" nullable:"false" list:"user" create:"domain_optional" update:"admin" json:"key"`
	State        string `width:"36" charset:"ascii" nullable:"false" default:"INIT" list:"user" update:"user"`
	AuditStatus  string `width:"36" charset:"ascii" list:"user" update:"user" json:"audit_status"`
	Setting      string `length:"text" nullable:"true"`
	ServerConf   string `length:"text" nullable:"true"`
	IsRevoked    bool   `nullable:"false" default:"false" list:"user" create:"domain_optional" json:"is_revoked"`
	RevokeReason string `length:"text" nullable:"true"`
}

func (manager *SWorkflowProcessInstanceManager) ListItemFilter(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	input api.WorkflowProcessInstanceListInput,
) (*sqlchemy.SQuery, error) {
	var err error
	q, err = manager.SEnabledStatusStandaloneResourceBaseManager.ListItemFilter(ctx, q, userCred, input.EnabledStatusStandaloneResourceListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SEnabledStatusStandaloneResourceBaseManager.ListItemFilter")
	}
	q, err = manager.SProjectizedResourceBaseManager.ListItemFilter(ctx, q, userCred, input.ProjectizedResourceListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SProjectizedResourceBaseManager.ListItemFilter")
	}
	if len(input.UserId) > 0 {
		q.Equals("initiator", input.UserId)
	}

	return q, nil
}

func (manager *SWorkflowProcessInstanceManager) FilterById(q *sqlchemy.SQuery, idStr string) *sqlchemy.SQuery {
	return q.Equals("id", idStr)
}

func (manager *SWorkflowProcessInstanceManager) ValidateCreateData(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	ownerId mcclient.IIdentityProvider,
	query jsonutils.JSONObject,
	input api.WorkflowProcessInstanceCreateInput,
) (api.WorkflowProcessInstanceInput, error) {
	var err error
	var output api.WorkflowProcessInstanceInput
	if &input.Variables != nil {
		output = input.Variables
		//params, err := jsonutils.ParseString(output.ServerCreateParameter)
		//if err != nil {
		//	return output, errors.Wrap(err, "validateServerCreateParameter")
		//}
		//generateName, err := params.GetString("generate_name")
		//if err != nil {
		//	return output, errors.Wrap(err, "validateProcessInstanceGenerateName")
		//}
		//output.Name = output.ProcessDefinitionKey + ":" + generateName
		output.Name = output.ProcessDefinitionKey + ":" + stringutils.UUID4()
	}
	output.EnabledStatusStandaloneResourceCreateInput, err = manager.SEnabledStatusStandaloneResourceBaseManager.ValidateCreateData(ctx, userCred, ownerId, query, output.EnabledStatusStandaloneResourceCreateInput)
	if err != nil {
		return output, errors.Wrap(err, "EnabledStatusStandaloneResourceCreateInput.ValidateCreateData")
	}

	//output.ProjectizedResourceCreateInput.ProjectDomainId, err = manager.SProjectizedResourceBaseManager.(ctx, userCred, ownerId, query, output.EnabledStatusStandaloneResourceCreateInput)
	//if err != nil {
	//	return output, errors.Wrap(err, "EnabledStatusStandaloneResourceCreateInput.ValidateCreateData")
	//}
	return output, nil
}

func (manager *SWorkflowProcessInstanceManager) QueryDistinctExtraField(q *sqlchemy.SQuery, field string) (*sqlchemy.SQuery, error) {
	var err error

	q, err = manager.SEnabledStatusStandaloneResourceBaseManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}
	q, err = manager.SProjectizedResourceBaseManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}

	return q, httperrors.ErrNotFound
}

func (manager *SWorkflowProcessInstanceManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []api.WorkflowProcessInstanceDetails {
	rows := make([]api.WorkflowProcessInstanceDetails, len(objs))

	baseRows := manager.SResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)
	s := auth.GetAdminSession(context.Background(), options.Options.Region, "")
	//recordIds := make([]string, len(objs))
	for i := range rows {
		instance := objs[i].(*SWorkflowProcessInstance)
		var endTime time.Time
		if instance.State == COMPLETED || instance.State == EXTERNALLY_TERMINATED {
			endTime = instance.UpdatedAt
		}
		ins, _ := jsonutils.ParseString(instance.Setting)
		resourceProjectId, _ := ins.GetString("project_id")

		//serviceObj, err := modules.ServicesV3.Get(s, service, nil)
		project, _ := modules.Projects.GetById(s, resourceProjectId, nil)
		resourceProjectName, _ := project.GetString("name")
		//
		user, _ := modules.UsersV3.GetById(s, instance.Initiator, nil)
		var userName string
		if user == nil {
			userName = "-"
		} else {
			userName, _ = user.GetString("displayname")
		}
		logs := make([]*api.STask, 0)
		processList, err := modules.BpmProcess.GetProcessList(s, instance.ExternalId)
		if err != nil {
			log.Errorf("Get BPM process err %v", err)
		}
		if processList != nil {
			for _, process := range processList.DoneList {
				t := new(api.STask)
				t.EndTime = process.EndTime
				t.ActivityName = process.NodeName.Zh
				for _, assignee := range process.AssigneeList {
					t.TaskAssigneeName = append(t.TaskAssigneeName, assignee.EmployeeName)
					t.TaskAssigneeAvatar = append(t.TaskAssigneeAvatar, assignee.HeadImage)
				}
				t.Task = api.Task{
					LocalVariables: struct {
						Comment string `json:"comment"`
					}(struct{ Comment string }{
						Comment: process.TaskComment}),
				}
				t.CommandMessage = process.CommandMessage
				logs = append(logs, t)
			}
			for _, process := range processList.TodoList {
				t := new(api.STask)
				t.EndTime = process.EndTime
				t.ActivityName = process.NodeName.Zh
				for _, assignee := range process.AssigneeList {
					t.TaskAssigneeName = append(t.TaskAssigneeName, assignee.EmployeeName)
					t.TaskAssigneeAvatar = append(t.TaskAssigneeAvatar, assignee.HeadImage)
				}
				t.Task = api.Task{
					LocalVariables: struct {
						Comment string `json:"comment"`
					}(struct{ Comment string }{
						Comment: process.TaskComment}),
				}
				t.CommandMessage = process.CommandMessage
				logs = append(logs, t)
			}
		}

		variables := api.WorkflowProcessInstanceVariablesDetails{
			Ids:                 instance.Ids,
			Initiator:           instance.Initiator,
			InitiatorName:       userName,
			AuditStatus:         instance.AuditStatus,
			ResourceProjectId:   resourceProjectId,
			ResourceProjectName: resourceProjectName,
			ServerConf:          instance.ServerConf,
		}
		switch instance.Key {
		case modules.APPLY_MACHINE:
			variables.ServerCreateParameter = instance.Setting
		case modules.APPLY_SERVER_CHANGECONFIG, modules.APPLY_BUCKET, modules.APPLY_LOADBALANCER:
			variables.Parameter = instance.Setting
		}

		rows[i] = api.WorkflowProcessInstanceDetails{
			ResourceBaseDetails:   baseRows[i],
			StartTime:             instance.CreatedAt,
			EndTime:               endTime,
			Log:                   logs,
			ProcessDefinitionKey:  instance.Key,
			ProcessDefinitionName: modules.WorkflowProcessDefinitionsMap[instance.Key],
			//Variables: api.WorkflowProcessInstanceVariablesDetails{
			//	Ids:                   instance.Ids,
			//	Initiator:             instance.Initiator,
			//	InitiatorName:         userName,
			//	AuditStatus:           instance.AuditStatus,
			//	ResourceProjectId:     resourceProjectId,
			//	ResourceProjectName:   resourceProjectName,
			//	ServerCreateParameter: instance.Setting,
			//	ServerConf:            instance.ServerConf,
			//},
			Variables: variables,
		}
	}
	return rows
}

func (manager *SWorkflowProcessInstanceManager) OrderByExtraFields(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.WorkflowProcessInstanceListInput,
) (*sqlchemy.SQuery, error) {
	var err error

	q, err = manager.SEnabledStatusStandaloneResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.EnabledStatusStandaloneResourceListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SEnabledStatusStandaloneResourceBaseManager.OrderByExtraFields")
	}
	q, err = manager.SProjectizedResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.ProjectizedResourceListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SProjectizedResourceBaseManager.OrderByExtraFields")
	}

	return q, nil
}

func (instance *SWorkflowProcessInstance) CustomizeCreate(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data jsonutils.JSONObject) error {
	var input api.WorkflowProcessInstanceInput
	err := data.Unmarshal(&input)
	if err != nil {
		return err
	}
	//p.Id = db.DefaultUUIDGenerator()
	instance.Key = input.ProcessDefinitionKey
	instance.Type = input.ProcessDefinitionKey
	switch input.ProcessDefinitionKey {
	case modules.APPLY_MACHINE:
		instance.Setting = input.ServerCreateParameter
	case modules.APPLY_SERVER_CHANGECONFIG, modules.APPLY_BUCKET, modules.APPLY_LOADBALANCER:
		instance.Setting = input.Parameter
	}
	instance.Ids = input.Ids
	instance.ServerConf = input.ServerConf
	instance.ProjectId = input.ProjectId
	return nil
}

func (instance *SWorkflowProcessInstance) PostCreate(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data jsonutils.JSONObject) {
	err := instance.StartBpmProcessSubmitTask(ctx, userCred)
	if err != nil {
		log.Errorf("unable to StartWorkflowSendTask: %v", err)
	}
}

func (instance *SWorkflowProcessInstance) PostUpdate(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) {
	reason, err := data.GetString("revoke_reason")
	if err != nil {
		log.Errorf("%s process instance get revoke reason params err: %v", instance.Id, err)
		return
		//return nil, errors.Wrap(err, "EnabledPerformEnable")
	}
	err = instance.performRevoke(userCred, reason)
	if err != nil {
		log.Errorf("%s process instance perform revoke err: %v", instance.Id, err)
	}
	log.Infof("%s process instance perform revoke success", instance.Id)
}

func (instance *SWorkflowProcessInstance) performRevoke(userCred mcclient.TokenCredential, reason string) error {
	diff, err := db.Update(instance, func() error {
		instance.IsRevoked = true
		instance.RevokeReason = reason
		instance.State = EXTERNALLY_TERMINATED
		return nil
	})
	if err != nil {
		return err
	}
	db.OpsLog.LogEvent(instance, db.ACT_UPDATE, diff, userCred)
	return err
}

func (instance *SWorkflowProcessInstance) PerformApprove(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, input apis.PerformEnableInput) (jsonutils.JSONObject, error) {
	switch instance.Key {
	case modules.APPLY_MACHINE:
		diff, err := db.Update(instance, func() error {
			instance.AuditStatus = AUDIT_APPROVED
			instance.State = CREATING
			return nil
		})
		if err != nil {
			return nil, errors.Wrap(err, "Workflow.PerformApprove MachineApplyTask")
		}
		instance.SetStatus(userCred, api.WORKFLOW_INSTANCE_STATUS_OPERATING, "")
		db.OpsLog.LogEvent(instance, db.ACT_UPDATE, diff, userCred)
		err = instance.StartMachineApplyTask(ctx, userCred)
		if err != nil {
			return nil, errors.Wrap(err, "Workflow.PerformApprove MachineApplyTask")
		}
	case modules.APPLY_SERVER_CHANGECONFIG:
		diff, err := db.Update(instance, func() error {
			instance.AuditStatus = AUDIT_APPROVED
			instance.State = CHANGING
			return nil
		})
		if err != nil {
			return nil, errors.Wrap(err, "Workflow.PerformApprove MachineChangeConfigTask")
		}
		instance.SetStatus(userCred, api.WORKFLOW_INSTANCE_STATUS_OPERATING, "")
		db.OpsLog.LogEvent(instance, db.ACT_UPDATE, diff, userCred)
		err = instance.StartMachineChangeConfigTask(ctx, userCred)
		if err != nil {
			return nil, errors.Wrap(err, "Workflow.PerformApprove MachineChangeConfigTask")
		}
	case modules.APPLY_BUCKET:
		diff, err := db.Update(instance, func() error {
			instance.AuditStatus = AUDIT_APPROVED
			instance.State = CREATING
			return nil
		})
		if err != nil {
			return nil, errors.Wrap(err, "Workflow.PerformApprove BucketApplyTask")
		}
		instance.SetStatus(userCred, api.WORKFLOW_INSTANCE_STATUS_OPERATING, "")
		db.OpsLog.LogEvent(instance, db.ACT_UPDATE, diff, userCred)
		err = instance.StartBucketApplyTask(ctx, userCred)
		if err != nil {
			return nil, errors.Wrap(err, "Workflow.PerformApprove BucketApplyTask")
		}
	case modules.APPLY_LOADBALANCER:
		diff, err := db.Update(instance, func() error {
			instance.AuditStatus = AUDIT_APPROVED
			instance.State = CREATING
			return nil
		})
		if err != nil {
			return nil, errors.Wrap(err, "Workflow.PerformApprove LoadbalancerApplyTask")
		}
		instance.SetStatus(userCred, api.WORKFLOW_INSTANCE_STATUS_OPERATING, "")
		db.OpsLog.LogEvent(instance, db.ACT_UPDATE, diff, userCred)
		err = instance.StartLoadbalancerApplyTask(ctx, userCred)
		if err != nil {
			return nil, errors.Wrap(err, "Workflow.PerformApprove LoadbalancerApplyTask")
		}
	}
	//t := time.Time{}
	//isOpen := true
	//tasks, err := taskman.TaskManager.FetchTasksOfObject(instance, t, &isOpen)
	//if err != nil {
	//	return nil, errors.Wrap(err, "Workflow.PerformApprove")
	//log.Errorf("unable to StartWorkflowSendTask: %v", err)
	//}
	//var a *taskman.STask
	//for _, task := range tasks {
	//	if task.TaskName == "MachineApplyTask" {
	//		a = &task
	//		break
	//	}
	//}

	//a.ScheduleRun(nil)
	return nil, nil
}

func (instance *SWorkflowProcessInstance) PerformRetry(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	diff, err := db.Update(instance, func() error {
		instance.State = CREATING
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "Workflow.PerformRetry")
	}
	instance.SetStatus(userCred, api.WORKFLOW_INSTANCE_STATUS_OPERATING, "")
	db.OpsLog.LogEvent(instance, db.ACT_UPDATE, diff, userCred)
	err = instance.StartMachineApplyRetryTask(ctx, userCred, data)
	if err != nil {
		return nil, errors.Wrap(err, "Workflow.PerformRetry")
	}

	//a.ScheduleRun(nil)
	return nil, nil
}

func (instance *SWorkflowProcessInstance) PerformReject(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, input apis.PerformEnableInput) (jsonutils.JSONObject, error) {
	diff, err := db.Update(instance, func() error {
		instance.AuditStatus = AUDIT_REFUSED
		instance.State = COMPLETED
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "Workflow.PerformReject")
	}
	instance.SetStatus(userCred, api.WORKFLOW_INSTANCE_STATUS_OK, "")
	db.OpsLog.LogEvent(instance, db.ACT_UPDATE, diff, userCred)
	return nil, nil
}

func (instance *SWorkflowProcessInstance) SetBpmExternalId(id string) error {
	_, err := db.Update(instance, func() error {
		instance.ExternalId = id
		return nil
	})
	return err
}

func (instance *SWorkflowProcessInstance) SetState(changeState string) error {
	_, err := db.Update(instance, func() error {
		instance.State = changeState
		return nil
	})
	return err
}

func (instance *SWorkflowProcessInstance) StartBpmProcessSubmitTask(ctx context.Context, userCred mcclient.TokenCredential) error {
	task, err := taskman.TaskManager.NewTask(ctx, "BpmProcessSubmitTask", instance, userCred, nil, "", "")
	if err != nil {
		return err
	}
	instance.SetState(SUBMITTING)
	task.ScheduleRun(nil)
	return nil
}

func (instance *SWorkflowProcessInstance) StartMachineApplyTask(ctx context.Context, userCred mcclient.TokenCredential) error {
	task, err := taskman.TaskManager.NewTask(ctx, "MachineApplyTask", instance, userCred, nil, "", "")
	if err != nil {
		return err
	}
	task.ScheduleRun(nil)
	return nil
}

func (instance *SWorkflowProcessInstance) StartMachineChangeConfigTask(ctx context.Context, userCred mcclient.TokenCredential) error {
	task, err := taskman.TaskManager.NewTask(ctx, "MachineChangeConfigTask", instance, userCred, nil, "", "")
	if err != nil {
		return err
	}
	task.ScheduleRun(nil)
	return nil
}

func (instance *SWorkflowProcessInstance) StartMachineApplyRetryTask(ctx context.Context, userCred mcclient.TokenCredential, data jsonutils.JSONObject) error {
	task, err := taskman.TaskManager.NewTask(ctx, "MachineApplyRetryTask", instance, userCred, nil, "", "")
	if err != nil {
		return err
	}

	task.ScheduleRun(data)
	return nil
}

func (instance *SWorkflowProcessInstance) StartBucketApplyTask(ctx context.Context, userCred mcclient.TokenCredential) error {
	task, err := taskman.TaskManager.NewTask(ctx, "BucketApplyTask", instance, userCred, nil, "", "")
	if err != nil {
		return err
	}
	task.ScheduleRun(nil)
	return nil
}

func (instance *SWorkflowProcessInstance) StartLoadbalancerApplyTask(ctx context.Context, userCred mcclient.TokenCredential) error {
	task, err := taskman.TaskManager.NewTask(ctx, "LoadbalancerApplyTask", instance, userCred, nil, "", "")
	if err != nil {
		return err
	}
	task.ScheduleRun(nil)
	return nil
}

//func (instance *SWorkflowProcessInstance) OnMachineApplyTask(ctx context.Context, userCred mcclient.TokenCredential) error {
//	task, err := taskman.TaskManager.FetchTasksOfObject(ctx, "MachineApplyTask", instance, userCred, nil, "", "")
//	if err != nil {
//		return err
//	}
//	task.ScheduleRun(nil)
//	return nil
//}
