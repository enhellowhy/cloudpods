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

package tasks

import (
	"context"
	"fmt"
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	apis "yunion.io/x/onecloud/pkg/apis/workflow"
	//"yunion.io/x/onecloud/pkg/cloudcommon/cmdline"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/util/logclient"
	"yunion.io/x/onecloud/pkg/workflow/options"
	//computeapi "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/workflow/models"
)

type MachineApplyRetryTask struct {
	MachineApplyTask
}

func init() {
	taskman.RegisterTask(MachineApplyRetryTask{})
}

func (self *MachineApplyRetryTask) taskFailed(ctx context.Context, workflow *models.SWorkflowProcessInstance, reason string) {
	log.Errorf("fail to retry apply machine workflow %q", workflow.GetId())
	workflow.SetStatus(self.UserCred, apis.WORKFLOW_INSTANCE_STATUS_FAILED, reason)
	//workflow.SetState(models.COMPLETED)
	logclient.AddActionLogWithContext(ctx, workflow, logclient.ACT_BPM_APPLY_MACHINE, reason, self.UserCred, false)
	self.SetStageFailed(ctx, jsonutils.NewString(reason))
}

func (self *MachineApplyRetryTask) OnInit(ctx context.Context, obj db.IStandaloneModel, body jsonutils.JSONObject) {
	workflow := obj.(*models.SWorkflowProcessInstance)
	if workflow.Status == apis.WORKFLOW_INSTANCE_STATUS_OK || workflow.Status == apis.WORKFLOW_INSTANCE_STATUS_SUCCESS {
		//if workflow.Status == apis.WORKFLOW_INSTANCE_STATUS_OK{
		self.SetStageComplete(ctx, nil)
		return
	}

	//todo create server
	params, err := jsonutils.ParseString(workflow.Setting)
	if err != nil {
		self.taskFailed(ctx, workflow, "fail to parse workflow setting")
		return
	}

	session := auth.GetAdminSession(ctx, options.Options.Region)
	self.UserCred = session.GetToken()

	count, _ := body.Get("count")
	dict := params.(*jsonutils.JSONDict)
	dict.Set("user_data", jsonutils.NewString(workflow.Initiator))
	dict.Set("__count__", count)
	failedList, succeedList := self.createInstances(session, params)
	//input, err := cmdline.FetchServerCreateInputByJSON(params)
	if len(failedList) != 0 {
		reason := fmt.Sprintf("fail to create instances, failed length is %d, %s", len(failedList), failedList[0])
		self.taskFailed(ctx, workflow, reason)
		return
	}
	self.SetStage("on_wait_guest_ready", nil)
	self.OnWaitGuestReady(ctx, session, workflow, succeedList, params)
}
