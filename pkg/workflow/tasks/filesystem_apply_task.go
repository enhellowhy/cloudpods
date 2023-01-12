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

type FileSystemApplyTask struct {
	taskman.STask
}

func init() {
	taskman.RegisterTask(FileSystemApplyTask{})
}

func (self *FileSystemApplyTask) taskFailed(ctx context.Context, workflow *models.SWorkflowProcessInstance, reason string) {
	log.Errorf("fail to apply file system workflow %q", workflow.GetId())
	workflow.SetStatus(self.UserCred, apis.WORKFLOW_INSTANCE_STATUS_FAILED, reason)
	workflow.SetState(models.COMPLETED)
	workflow.SetMetadata(ctx, "sys_error", reason, self.UserCred)
	logclient.AddActionLogWithContext(ctx, workflow, logclient.ACT_BPM_APPLY_FILESYSTEM, reason, self.UserCred, false)
	self.SetStageFailed(ctx, jsonutils.NewString(reason))
}

func (self *FileSystemApplyTask) OnInit(ctx context.Context, obj db.IStandaloneModel, body jsonutils.JSONObject) {
	workflow := obj.(*models.SWorkflowProcessInstance)
	if workflow.Status == apis.WORKFLOW_INSTANCE_STATUS_OK || workflow.Status == apis.WORKFLOW_INSTANCE_STATUS_SUCCESS {
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

	failedList, succeedList := self.createFileSystems(session, params)
	if len(failedList) != 0 {
		reason := fmt.Sprintf("fail to create file systems, failed length is %d, %s", len(failedList), failedList[0])
		self.taskFailed(ctx, workflow, reason)
		return
	}
	self.SetStage("on_wait_filesystem_ready", nil)
	self.OnWaitFileSystemReady(ctx, session, workflow, succeedList, params)
}

func (self *FileSystemApplyTask) OnWaitFileSystemReady(ctx context.Context, session *mcclient.ClientSession, workflow *models.SWorkflowProcessInstance, succeedList []SInstance, params jsonutils.JSONObject) {
	// wait for create complete
	input := new(compute.FileSystemCreateInput)
	_ = params.Unmarshal(input)

	retChan := make(chan SCreateRet, len(succeedList))

	ids := make([]string, len(succeedList))
	for i, instane := range succeedList {
		ids[i] = instane.ID
	}
	// check all file systems status
	var waitLimit, waitInterval time.Duration
	waitLimit = 5 * time.Minute
	waitInterval = 5 * time.Second
	go self.checkAllFileSystems(session, ids, retChan, waitLimit, waitInterval)

	log.Infof("FileSystemApplyTask waiting for all file systems ready")
	for {
		ret, ok := <-retChan
		if !ok {
			break
		}
		if ret.Status != compute.NAS_STATUS_AVAILABLE {
			reason := fmt.Sprintf("not ready file systems %s, status is %s", ret.Id, ret.Status)
			self.taskFailed(ctx, workflow, reason)
			return
		}
	}
	// success
	workflow.SetStatus(self.UserCred, apis.WORKFLOW_INSTANCE_STATUS_SUCCESS, "")
	workflow.SetState(models.COMPLETED)
	logclient.AddActionLogWithContext(ctx, workflow, logclient.ACT_BPM_APPLY_FILESYSTEM, "", self.UserCred, true)
	self.SetStageComplete(ctx, nil)
}

func (self *FileSystemApplyTask) createFileSystems(session *mcclient.ClientSession, params jsonutils.JSONObject) ([]string, []SInstance) {
	var failedList []string
	var succeedList []SInstance
	input := new(compute.FileSystemCreateInput)
	err := params.Unmarshal(input)
	if err != nil {
		failedList = append(failedList, err.Error())
		return failedList, succeedList
	}
	//count := 1

	ret, err := computemod.FileSystems.Create(session, params)
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

func (self *FileSystemApplyTask) checkAllFileSystems(session *mcclient.ClientSession, ids []string, retChan chan SCreateRet,
	waitLimit, waitInterval time.Duration) {
	iDSet := sets.NewString(ids...)
	timer := time.NewTimer(waitLimit)
	ticker := time.NewTicker(waitInterval)
	defer func() {
		close(retChan)
		ticker.Stop()
		timer.Stop()
		log.Debugf("finish all check jobs when creating filesystems")
	}()
	log.Debugf("file system Ids: %s", ids)
	for {
		select {
		default:
			for _, id := range iDSet.UnsortedList() {
				ret, e := computemod.FileSystems.GetSpecific(session, id, "status", nil)
				if e != nil {
					log.Errorf("FileSystems.GetSpecific failed: %s", e)
					<-ticker.C
					continue
				}
				log.Debugf("ret from GetSpecific: %s", ret.String())
				status, _ := ret.GetString("status")
				if status == compute.NAS_STATUS_AVAILABLE || strings.HasSuffix(status, "fail") || strings.HasSuffix(status, "failed") {
					iDSet.Delete(id)
					retChan <- SCreateRet{
						Id:     id,
						Status: status,
					}
				}
			}
			if iDSet.Len() == 0 {
				return
			}
			<-ticker.C
		case <-timer.C:
			log.Errorf("some check jobs for file systems timeout")
			for _, id := range iDSet.UnsortedList() {
				retChan <- SCreateRet{
					Id:     id,
					Status: "timeout",
				}
			}
			return
		}
	}
}
