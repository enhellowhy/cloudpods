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
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	computemod "yunion.io/x/onecloud/pkg/mcclient/modules/compute"
	"yunion.io/x/onecloud/pkg/util/httputils"
	"yunion.io/x/onecloud/pkg/util/logclient"
	"yunion.io/x/onecloud/pkg/workflow/models"
	"yunion.io/x/onecloud/pkg/workflow/options"
	"yunion.io/x/pkg/util/sets"
)

type LoadbalancerApplyTask struct {
	taskman.STask
}

func init() {
	taskman.RegisterTask(LoadbalancerApplyTask{})
}

func (self *LoadbalancerApplyTask) taskFailed(ctx context.Context, workflow *models.SWorkflowProcessInstance, reason string) {
	log.Errorf("fail to apply lb workflow %q", workflow.GetId())
	workflow.SetStatus(self.UserCred, apis.WORKFLOW_INSTANCE_STATUS_FAILED, reason)
	workflow.SetState(models.COMPLETED)
	logclient.AddActionLogWithContext(ctx, workflow, logclient.ACT_BPM_APPLY_LOADBALANCER, reason, self.UserCred, false)
	self.SetStageFailed(ctx, jsonutils.NewString(reason))
}

func (self *LoadbalancerApplyTask) OnInit(ctx context.Context, obj db.IStandaloneModel, body jsonutils.JSONObject) {
	workflow := obj.(*models.SWorkflowProcessInstance)
	if workflow.Status == apis.WORKFLOW_INSTANCE_STATUS_OK || workflow.Status == apis.WORKFLOW_INSTANCE_STATUS_SUCCESS {
		//if workflow.Status == apis.WORKFLOW_INSTANCE_STATUS_OK{
		self.SetStageComplete(ctx, nil)
		return
	}

	params, err := jsonutils.ParseString(workflow.Setting)
	if err != nil {
		self.taskFailed(ctx, workflow, "fail to parse workflow setting")
		return
	}

	session := auth.GetSession(ctx, self.UserCred, options.Options.Region)
	valid := self.UserCred.IsValid()
	if !valid {
		session = auth.GetAdminSession(ctx, options.Options.Region)
		self.UserCred = session.GetToken()
	}

	//dict := params.(*jsonutils.JSONDict)
	//dict.Set("user_id", jsonutils.NewString(workflow.Initiator))
	failedList, succeedList := self.createInstances(session, params)
	//input, err := cmdline.FetchServerCreateInputByJSON(params)
	if len(failedList) != 0 {
		reason := fmt.Sprintf("fail to create lb instances, failed length is %d, %s", len(failedList), failedList[0])
		self.taskFailed(ctx, workflow, reason)
		return
	}
	self.SetStage("on_wait_lb_ready", nil)
	self.OnWaitLoadbalancerReady(ctx, session, workflow, succeedList, params)
}

func (self *LoadbalancerApplyTask) OnWaitLoadbalancerReady(ctx context.Context, session *mcclient.ClientSession, workflow *models.SWorkflowProcessInstance, succeedList []SInstance, params jsonutils.JSONObject) {
	// wait for create complete
	//input := new(compute.LoadbalancerCreateInput)
	//_ = params.Unmarshal(input)

	retChan := make(chan SCreateRet, len(succeedList))

	lbIds := make([]string, len(succeedList))
	for i, instane := range succeedList {
		lbIds[i] = instane.ID
	}
	// check all server's status
	var waitLimit, waitinterval time.Duration
	waitLimit = 5 * time.Minute
	waitinterval = 5 * time.Second
	go self.checkAllLbs(session, lbIds, retChan, waitLimit, waitinterval)

	log.Infof("LoadbalancerApplyTask waiting for all lb ready")
	for {
		ret, ok := <-retChan
		if !ok {
			break
		}
		if ret.Status != compute.LB_STATUS_ENABLED {
			reason := fmt.Sprintf("not ready for lb instances %s, status is %s", ret.Id, ret.Status)
			self.taskFailed(ctx, workflow, reason)
			return
		}
	}
	// success
	workflow.SetStatus(self.UserCred, apis.WORKFLOW_INSTANCE_STATUS_SUCCESS, "")
	workflow.SetState(models.COMPLETED)
	logclient.AddActionLogWithContext(ctx, workflow, logclient.ACT_BPM_APPLY_LOADBALANCER, "", self.UserCred, true)
	self.SetStageComplete(ctx, nil)
}

func (self *LoadbalancerApplyTask) createInstances(session *mcclient.ClientSession, params jsonutils.JSONObject) ([]string, []SInstance) {
	var failedList []string
	var succeedList []SInstance
	input := new(compute.LoadbalancerCreateInput)
	err := params.Unmarshal(input)
	if err != nil {
		failedList = append(failedList, err.Error())
		return failedList, succeedList
	}
	//count := 1

	ret, err := computemod.Loadbalancers.Create(session, params)
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

func (self *LoadbalancerApplyTask) checkAllLbs(session *mcclient.ClientSession, lbIds []string, retChan chan SCreateRet,
	waitLimit, waitInterval time.Duration) {
	lbIDSet := sets.NewString(lbIds...)
	timer := time.NewTimer(waitLimit)
	ticker := time.NewTicker(waitInterval)
	defer func() {
		close(retChan)
		ticker.Stop()
		timer.Stop()
		log.Debugf("finish all check jobs when creating lbs")
	}()
	log.Debugf("Lb id: %s", lbIds)
	for {
		select {
		default:
			for _, id := range lbIDSet.UnsortedList() {
				ret, e := computemod.Loadbalancers.GetSpecific(session, id, "status", nil)
				if e != nil {
					log.Errorf("Loadbalancers.GetSpecific failed: %s", e)
					<-ticker.C
					continue
				}
				log.Debugf("ret from GetSpecific: %s", ret.String())
				status, _ := ret.GetString("status")
				if status == compute.LB_STATUS_ENABLED || strings.HasSuffix(status, "fail") || strings.HasSuffix(status, "failed") {
					lbIDSet.Delete(id)
					retChan <- SCreateRet{
						Id:     id,
						Status: status,
					}
				}
			}
			if lbIDSet.Len() == 0 {
				return
			}
			<-ticker.C
		case <-timer.C:
			log.Errorf("some check jobs for lb timeout")
			for _, id := range lbIDSet.UnsortedList() {
				retChan <- SCreateRet{
					Id:     id,
					Status: "timeout",
				}
			}
			return
		}
	}
}
