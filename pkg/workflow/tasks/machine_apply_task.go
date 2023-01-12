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
	"strings"
	"time"
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/apis/compute"
	apis "yunion.io/x/onecloud/pkg/apis/workflow"
	computemod "yunion.io/x/onecloud/pkg/mcclient/modules/compute"
	"yunion.io/x/onecloud/pkg/mcclient/modules/scheduler"
	"yunion.io/x/pkg/util/sets"
	//"yunion.io/x/onecloud/pkg/cloudcommon/cmdline"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/util/httputils"
	"yunion.io/x/onecloud/pkg/util/logclient"
	"yunion.io/x/onecloud/pkg/workflow/options"
	//computeapi "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/workflow/models"
)

type MachineApplyTask struct {
	taskman.STask
}

func init() {
	taskman.RegisterTask(MachineApplyTask{})
}

func (self *MachineApplyTask) taskFailed(ctx context.Context, workflow *models.SWorkflowProcessInstance, reason string) {
	log.Errorf("fail to apply machine workflow %q", workflow.GetId())
	workflow.SetStatus(self.UserCred, apis.WORKFLOW_INSTANCE_STATUS_FAILED, reason)
	workflow.SetState(models.COMPLETED)
	workflow.SetMetadata(ctx, "sys_error", reason, self.UserCred)
	logclient.AddActionLogWithContext(ctx, workflow, logclient.ACT_BPM_APPLY_MACHINE, reason, self.UserCred, false)
	self.SetStageFailed(ctx, jsonutils.NewString(reason))
}

func (self *MachineApplyTask) OnInit(ctx context.Context, obj db.IStandaloneModel, body jsonutils.JSONObject) {
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

	//session := new(mcclient.ClientSession)
	session := auth.GetSession(ctx, self.UserCred, options.Options.Region)
	valid := self.UserCred.IsValid()
	if !valid {
		//if !valid || session.GetToken().GetTokenString() == auth.GUEST_TOKEN {
		//ws, err := auth.Client().AuthenticateOAuth2("2fb61a6ce17d49e9812f876c3d3db418", "GJqbqhjS3aIlMlI1100000001175400", self.UserCred.GetProjectId(), self.UserCred.GetProjectName(), self.UserCred.GetProjectDomain(), "")
		//if err != nil {
		//	self.taskFailed(ctx, workflow, "fail to parse workflow setting")
		//	return
		//}
		session = auth.GetAdminSession(ctx, options.Options.Region)
		self.UserCred = session.GetToken()
	}

	dict := params.(*jsonutils.JSONDict)
	dict.Set("user_data", jsonutils.NewString(workflow.Initiator))
	failedList, succeedList := self.createInstances(session, params)
	//input, err := cmdline.FetchServerCreateInputByJSON(params)
	if len(failedList) != 0 {
		reason := fmt.Sprintf("fail to create instances, failed length is %d, %s", len(failedList), failedList[0])
		self.taskFailed(ctx, workflow, reason)
		return
	}
	self.SetStage("on_wait_guest_ready", nil)
	self.OnWaitGuestReady(ctx, session, workflow, succeedList, params)
	//taskNotify := options.BoolV(input.TaskNotify)
	//if taskNotify {
	//	s.PrepareTask()
	//}
	//var err error
	//if taskNotify {
	//	s.WaitTaskNotify()
	//}
	//if err != nil {
	//	self.taskFailed(ctx, notification, "fail to fetch ReceiverNotifications")
	//	return
	//}
}

func (self *MachineApplyTask) OnWaitGuestReady(ctx context.Context, session *mcclient.ClientSession, workflow *models.SWorkflowProcessInstance, succeedList []SInstance, params jsonutils.JSONObject) {
	// wait for create complete
	input := new(compute.ServerCreateInput)
	_ = params.Unmarshal(input)

	retChan := make(chan SCreateRet, len(succeedList))

	guestIds := make([]string, len(succeedList))
	for i, instane := range succeedList {
		guestIds[i] = instane.ID
	}
	// check all server's status
	var waitLimit, waitinterval time.Duration
	if input.Hypervisor == compute.HYPERVISOR_KVM {
		waitLimit = 10 * time.Minute
		waitinterval = 5 * time.Second
	} else {
		waitLimit = 10 * time.Minute
		waitinterval = 10 * time.Second
	}
	go self.checkAllServer(session, guestIds, retChan, waitLimit, waitinterval)

	//succeedInstances := make([]string, 0, len(succeedList))
	//workerLimit := make(chan struct{}, input.Count)
	log.Infof("MachineApplyTask waiting for all guest running")
	for {
		ret, ok := <-retChan
		if !ok {
			break
		}
		if ret.Status != compute.VM_RUNNING {
			reason := fmt.Sprintf("not ready instances %s, status is %s", ret.Id, ret.Status)
			self.taskFailed(ctx, workflow, reason)
			return
		}
		//workerLimit <- struct{}{}
		// bind ld and db
		//go func() {
		//	succeed := asc.actionAfterCreate(ctx, userCred, session, sg, ret, failRecord)
		//	log.Debugf("action after create instance '%s', succeed: %t", ret.Id, succeed)
		//	if succeed {
		//		succeedInstances = append(succeedInstances, ret.Id)
		//	}
		//	<-workerLimit
		//}()
	}
	// success
	workflow.SetStatus(self.UserCred, apis.WORKFLOW_INSTANCE_STATUS_SUCCESS, "")
	workflow.SetState(models.COMPLETED)
	logclient.AddActionLogWithContext(ctx, workflow, logclient.ACT_BPM_APPLY_MACHINE, "", self.UserCred, true)
	self.SetStageComplete(ctx, nil)
}

type SCreateRet struct {
	Id     string
	Status string
}

type SInstance struct {
	ID   string
	Name string
}

func (self *MachineApplyTask) createInstances(session *mcclient.ClientSession, params jsonutils.JSONObject) ([]string, []SInstance) {
	// forcast first
	var failedList []string
	var succeedList []SInstance
	input := new(compute.ServerCreateInput)
	err := params.Unmarshal(input)
	if err != nil {
		failedList = append(failedList, err.Error())
		return failedList, succeedList
	}
	count := input.Count

	dict := params.(*jsonutils.JSONDict)
	dict.Set("count", jsonutils.NewInt(int64(count)))
	_, err = scheduler.SchedManager.DoForecast(session, dict)
	if err != nil {
		clientErr := err.(*httputils.JSONClientError)
		failedList = append(failedList, clientErr.Details)
		return failedList, succeedList
	}
	dict.Remove("domain_id")
	dict.Remove("count")

	if count == 1 {
		ret, err := computemod.Servers.Create(session, params)
		if err != nil {
			clientErr := err.(*httputils.JSONClientError)
			failedList = append(failedList, clientErr.Details)
			return failedList, succeedList
		}
		id, _ := ret.GetString("id")
		name, _ := ret.GetString("name")
		succeedList = append(succeedList, SInstance{id, name})
		return failedList, succeedList
	}
	rets := computemod.Servers.BatchCreate(session, params, count)
	for _, ret := range rets {
		if ret.Status >= 400 {
			failedList = append(failedList, ret.Data.String())
		} else {
			id, _ := ret.Data.GetString("id")
			name, _ := ret.Data.GetString("name")
			succeedList = append(succeedList, SInstance{id, name})
		}
	}
	return failedList, succeedList
}

func (self *MachineApplyTask) checkAllServer(session *mcclient.ClientSession, guestIds []string, retChan chan SCreateRet,
	waitLimit, waitInterval time.Duration) {
	guestIDSet := sets.NewString(guestIds...)
	timer := time.NewTimer(waitLimit)
	ticker := time.NewTicker(waitInterval)
	defer func() {
		close(retChan)
		ticker.Stop()
		timer.Stop()
		log.Debugf("finish all check jobs when creating servers")
	}()
	log.Debugf("guestIds: %s", guestIds)
	for {
		select {
		default:
			for _, id := range guestIDSet.UnsortedList() {
				ret, e := computemod.Servers.GetSpecific(session, id, "status", nil)
				if e != nil {
					log.Errorf("Servers.GetSpecific failed: %s", e)
					<-ticker.C
					continue
				}
				log.Debugf("ret from GetSpecific: %s", ret.String())
				status, _ := ret.GetString("status")
				if status == compute.VM_RUNNING || strings.HasSuffix(status, "fail") || strings.HasSuffix(status, "failed") {
					guestIDSet.Delete(id)
					retChan <- SCreateRet{
						Id:     id,
						Status: status,
					}
				}
			}
			if guestIDSet.Len() == 0 {
				return
			}
			<-ticker.C
		case <-timer.C:
			log.Errorf("some check jobs for server timeout")
			for _, id := range guestIDSet.UnsortedList() {
				retChan <- SCreateRet{
					Id:     id,
					Status: "timeout",
				}
			}
			return
		}
	}
}
