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
	"yunion.io/x/pkg/util/sets"
	//"yunion.io/x/onecloud/pkg/cloudcommon/cmdline"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
	"yunion.io/x/onecloud/pkg/util/httputils"
	"yunion.io/x/onecloud/pkg/util/logclient"
	"yunion.io/x/onecloud/pkg/workflow/options"
	//computeapi "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/workflow/models"
)

type MachineChangeConfigTask struct {
	taskman.STask
}

func init() {
	taskman.RegisterTask(MachineChangeConfigTask{})
}

func (self *MachineChangeConfigTask) taskFailed(ctx context.Context, workflow *models.SWorkflowProcessInstance, reason string) {
	log.Errorf("fail to changeconfig machine workflow %q", workflow.GetId())
	workflow.SetStatus(self.UserCred, apis.WORKFLOW_INSTANCE_STATUS_FAILED, reason)
	workflow.SetState(models.COMPLETED)
	logclient.AddActionLogWithContext(ctx, workflow, logclient.ACT_BPM_CHANGECONFIG_MACHINE, reason, self.UserCred, false)
	self.SetStageFailed(ctx, jsonutils.NewString(reason))
}

func (self *MachineChangeConfigTask) OnInit(ctx context.Context, obj db.IStandaloneModel, body jsonutils.JSONObject) {
	workflow := obj.(*models.SWorkflowProcessInstance)
	if workflow.Status == apis.WORKFLOW_INSTANCE_STATUS_OK || workflow.Status == apis.WORKFLOW_INSTANCE_STATUS_SUCCESS {
		self.SetStageComplete(ctx, nil)
		return
	}

	//change config server
	params, err := jsonutils.ParseString(workflow.Setting)
	if err != nil {
		self.taskFailed(ctx, workflow, "fail to parse change config workflow setting")
		return
	}

	session := auth.GetSession(ctx, self.UserCred, options.Options.Region, "")
	valid := self.UserCred.IsValid()
	if !valid {
		session = auth.GetAdminSession(ctx, options.Options.Region, "")
		self.UserCred = session.GetToken()
	}

	ids := strings.Split(workflow.Ids, ",")
	failedList, succeedList := self.changeInstances(session, params, ids)
	if len(failedList) != 0 {
		reason := fmt.Sprintf("fail to change config instances, failed length is %d, %s", len(failedList), failedList[0])
		self.taskFailed(ctx, workflow, reason)
		return
	}
	self.SetStage("on_wait_guest_ready", nil)
	self.OnWaitGuestReady(ctx, session, workflow, succeedList, params)
}

func (self *MachineChangeConfigTask) OnWaitGuestReady(ctx context.Context, session *mcclient.ClientSession, workflow *models.SWorkflowProcessInstance, succeedList []SInstance, params jsonutils.JSONObject) {
	// wait for change config complete
	retChan := make(chan SCreateRet, len(succeedList))

	guestIds := make([]string, len(succeedList))
	for i, instane := range succeedList {
		guestIds[i] = instane.ID
	}

	// check all server's status
	var waitLimit, waitInterval time.Duration
	waitLimit = 5 * time.Minute
	waitInterval = 5 * time.Second
	go self.checkAllServer(session, guestIds, retChan, waitLimit, waitInterval)

	log.Infof("MachineChangeConfigTask waiting for all guest running")
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
	}

	// success
	workflow.SetStatus(self.UserCred, apis.WORKFLOW_INSTANCE_STATUS_SUCCESS, "")
	workflow.SetState(models.COMPLETED)
	logclient.AddActionLogWithContext(ctx, workflow, logclient.ACT_BPM_CHANGECONFIG_MACHINE, "", self.UserCred, true)
	self.SetStageComplete(ctx, nil)
}

func (self *MachineChangeConfigTask) changeInstances(session *mcclient.ClientSession, params jsonutils.JSONObject, ids []string) ([]string, []SInstance) {
	var failedList []string
	var succeedList []SInstance

	count := len(ids)
	if count == 1 {
		ret, err := modules.Servers.PerformAction(session, ids[0], "change-config", params)
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
	rets := modules.Servers.BatchPerformAction(session, ids, "change-config", params)
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

func (self *MachineChangeConfigTask) checkAllServer(session *mcclient.ClientSession, guestIds []string, retChan chan SCreateRet,
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
				ret, e := modules.Servers.GetSpecific(session, id, "status", nil)
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
