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
	"time"
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/cloudcommon/notifyclient"
	"yunion.io/x/onecloud/pkg/compute/models"
	"yunion.io/x/onecloud/pkg/util/logclient"
)

type BucketCreateTask struct {
	taskman.STask
}

func init() {
	taskman.RegisterTask(BucketCreateTask{})
}

func (task *BucketCreateTask) taskFailed(ctx context.Context, bucket *models.SBucket, err error) {
	bucket.SetStatus(task.UserCred, api.BUCKET_STATUS_CREATE_FAIL, err.Error())
	db.OpsLog.LogEvent(bucket, db.ACT_ALLOCATE_FAIL, err, task.UserCred)
	logclient.AddActionLogWithStartable(task, bucket, logclient.ACT_ALLOCATE, err, task.UserCred, false)
	task.SetStageFailed(ctx, jsonutils.NewString(err.Error()))
}

func (task *BucketCreateTask) OnInit(ctx context.Context, obj db.IStandaloneModel, body jsonutils.JSONObject) {
	bucket := obj.(*models.SBucket)

	t, _ := body.GetString("cloudprovider_type")
	if t == api.CLOUD_PROVIDER_TYPE_NEW {
		task.SetStage("on_create_user", nil)
		task.OnCreateUser(ctx, bucket, body)
	} else {
		task.OnCreateBucket(ctx, bucket, body)
	}
}

func (task *BucketCreateTask) OnCreateBucket(ctx context.Context, bucket *models.SBucket, params jsonutils.JSONObject) {
	bucket.SetStatus(task.UserCred, api.BUCKET_STATUS_CREATING, "StartBucketCreateTask")

	err := bucket.RemoteCreate(ctx, task.UserCred)
	if err != nil {
		task.taskFailed(ctx, bucket, err)
		return
	}

	bucket.SetStatus(task.UserCred, api.BUCKET_STATUS_READY, "BucketCreateTask")
	notifyclient.EventNotify(ctx, task.GetUserCred(), notifyclient.SEventNotifyParam{
		Obj:    bucket,
		Action: notifyclient.ActionCreate,
	})
	bucket.NotifyInitiatorFeishuBucketEvent(ctx, task.UserCred)
	logclient.AddActionLogWithStartable(task, bucket, logclient.ACT_ALLOCATE, nil, task.UserCred, true)
	task.SetStageComplete(ctx, nil)
}

func (task *BucketCreateTask) OnCreateUser(ctx context.Context, bucket *models.SBucket, params jsonutils.JSONObject) {
	bucket.SetStatus(task.UserCred, api.USER_STATUS_BUILDING, "StartUserCreateTask")

	account, _ := params.GetString("cloudaccount")
	name, _ := params.GetString("cloudprovider_name")
	id, err := bucket.RemoteCreateUser(ctx, task.UserCred, account, name)
	if err != nil {
		task.taskFailed(ctx, bucket, err)
		return
	}
	accountParams := jsonutils.NewDict()
	accountParams.Set("cloudaccount", jsonutils.NewString(account))
	accountParams.Set("user_id", jsonutils.NewInt(int64(id)))
	task.SetStage("on_wait_cloudprovider_ready", nil)
	task.OnWaitCloudproviderReady(ctx, bucket, accountParams)
}

func (task *BucketCreateTask) OnWaitCloudproviderReady(ctx context.Context, bucket *models.SBucket, params jsonutils.JSONObject) {

	account, _ := params.GetString("cloudaccount")
	id, _ := params.Int("user_id")
	subAccount, err := bucket.GetSubAccountById(ctx, task.UserCred, account, int(id))
	if err != nil {
		log.Infof("SubAccount user %d not ready!!", id)
		time.Sleep(time.Second * 2)
		task.ScheduleRun(params)
	} else {
		err := bucket.CreateSubAccountProvider(ctx, task.UserCred, account, subAccount)
		if err != nil {
			task.taskFailed(ctx, bucket, err)
			return
		}
		task.OnCreateBucket(ctx, bucket, params)
	}
}
