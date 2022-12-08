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
	"time"
	"yunion.io/x/onecloud/pkg/multicloud/nas/xgfs"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"

	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/compute/models"
	"yunion.io/x/onecloud/pkg/util/logclient"
)

type MountTargetAclCreateTask struct {
	taskman.STask
}

func init() {
	taskman.RegisterTask(MountTargetAclCreateTask{})
}

func (self *MountTargetAclCreateTask) taskFailed(ctx context.Context, mta *models.SMountTargetAcl, err error) {
	mta.SetStatus(self.UserCred, api.MOUNT_TARGET_ACL_STATUS_ADD_FAILED, err.Error())
	logclient.AddActionLogWithStartable(self, mta, logclient.ACT_ALLOCATE, err, self.UserCred, false)
	self.SetStageFailed(ctx, jsonutils.NewString(err.Error()))
}

func (self *MountTargetAclCreateTask) OnInit(ctx context.Context, obj db.IStandaloneModel, body jsonutils.JSONObject) {
	mta := obj.(*models.SMountTargetAcl)
	if mta.Source == "" {
		self.taskFailed(ctx, mta, fmt.Errorf("SMountTargetAcl source is empty"))
		return
	}

	fs, err := mta.GetFileSystem()
	if err != nil {
		self.taskFailed(ctx, mta, errors.Wrapf(err, "GetFileSystem"))
		return
	}
	region, err := fs.GetIRegion(ctx)
	if err != nil {
		self.taskFailed(ctx, mta, errors.Wrapf(err, "fs.GetIRegion"))
		return
	}

	ifs, err := region.GetICloudFileSystemById(fs.ExternalId) //ExternalId is path
	if err != nil {
		self.taskFailed(ctx, mta, errors.Wrapf(err, "GetICloudFileSystemById"))
		return
	}

	fsClients, err := region.(*xgfs.SXgfsClient).GetFileSystemClientByIP(ctx, mta.Source)
	if err != nil {
		self.taskFailed(ctx, mta, errors.Wrapf(err, "GetFileSystemClientByIP"))
		return
	}
	fsClientId := -1
	if len(fsClients) == 0 {
		// create
		fsClient, err := region.(*xgfs.SXgfsClient).CreateFileSystemClient(ctx, mta.Name, mta.Source)
		if err != nil {
			self.taskFailed(ctx, mta, errors.Wrapf(err, "CreateFileSystemClient"))
			return
		}
		if fsClient.Ip == mta.Source {
			fsClientId = fsClient.Id
		}
	} else {
		if fsClients[0].Ip == mta.Source {
			fsClientId = fsClients[0].Id
		}
	}
	if fsClientId == -1 {
		self.taskFailed(ctx, mta, fmt.Errorf("fsClientId with value -1"))
		return
	}
	self.SetStage("on_wait_acl_ready", nil)
	self.OnSyncMountTargetAclComplete(ctx, fs, ifs, mta, fsClientId, nil)
}

//func (self *MountTargetAclCreateTask) OnSyncMountTargetAclCompleteFailed(ctx context.Context, mta *models.SMountTargetAcl, data jsonutils.JSONObject) {
//	self.taskFailed(ctx, mta, errors.Error(data.String()))
//}
//
//func (self *MountTargetAclCreateTask) OnSyncMountTargetAclComplete(ctx context.Context, mta *models.SMountTargetAcl, data jsonutils.JSONObject) {
//}

func (self *MountTargetAclCreateTask) OnSyncMountTargetAclComplete(ctx context.Context, fs *models.SFileSystem, iFs cloudprovider.ICloudFileSystem, mta *models.SMountTargetAcl, fsClientId int, data jsonutils.JSONObject) {
	mountTargets, err := iFs.GetMountTargets()
	if err != nil {
		self.taskFailed(ctx, mta, errors.Wrapf(err, "ifs.GetMountTargets"))
		return
	}
	// opts
	opts := cloudprovider.SDfsNfsShareAclCreateOptions{}
	if cloudprovider.TRWAccessType(mta.RWAccessType) == cloudprovider.RWAccessTypeRW {
		opts.Permission = "RW"
	} else {
		opts.Permission = "RO"
	}
	if cloudprovider.TUserAccessType(mta.UserAccessType) == cloudprovider.UserAccessTypeAllSquash {
		opts.AllSquash = true
	} else {
		opts.AllSquash = false
	}
	if cloudprovider.TUserAccessType(mta.RootUserAccessType) == cloudprovider.UserAccessTypeRootSquash {
		opts.RootSquash = true
	} else {
		opts.RootSquash = false
	}
	opts.Sync = mta.Sync
	opts.FsClientId = fsClientId
	opts.Type = "client"

	shareId := -1
	if len(mountTargets) == 0 {
		// create dfs shares
		iMt, err := iFs.(*xgfs.SXgfsFileSystem).CreateDfsNfsShare(opts)
		if err != nil {
			self.taskFailed(ctx, mta, errors.Wrapf(err, "iFs.CreateDfsNfsShare"))
			return
		}
		shareId = iMt.(*xgfs.SDfsNfsShare).GetId()
		cloudprovider.Wait(time.Second*3, time.Minute*10, func() (bool, error) {
			mts, err := iFs.GetMountTargets()
			if err != nil {
				return false, errors.Wrapf(err, "iFs.GetMountTargets")
			}
			for i := range mts {
				if mts[i].GetGlobalId() == iMt.GetGlobalId() {
					status := mts[i].GetStatus()
					log.Infof("expect mount point status %s current is %s", api.MOUNT_TARGET_STATUS_AVAILABLE, status)
					if status == api.MOUNT_TARGET_STATUS_AVAILABLE {
						iMt = mts[i]
						return true, nil
					}
				}
			}
			return false, nil
		})
		//err = mta.SyncWithMountTargetAcl(ctx, self.GetUserCred(), iMt)
		//if err != nil {
		//	self.taskFailed(ctx, mta, errors.Wrapf(err, "mta.SyncWithMountTargetAcl"))
		//	return
		//}
	} else {
		// add dfs shares acl
		iMt, err := iFs.(*xgfs.SXgfsFileSystem).AddDfsNfsShareAcl(opts, mountTargets[0].(*xgfs.SDfsNfsShare).GetId())
		if err != nil {
			self.taskFailed(ctx, mta, errors.Wrapf(err, "iFs.AddDfsNfsShareAcl"))
			return
		}
		shareId = iMt.(*xgfs.SDfsNfsShare).GetId()
		cloudprovider.Wait(time.Second*3, time.Minute*10, func() (bool, error) {
			mts, err := iFs.GetMountTargets()
			if err != nil {
				return false, errors.Wrapf(err, "iFs.GetMountTargets")
			}
			for i := range mts {
				if mts[i].GetGlobalId() == iMt.GetGlobalId() {
					actionStatus := mts[i].(*xgfs.SDfsNfsShare).GetActionStatus() // dfs_share_acl_adding
					log.Infof("expect action status %s current is %s", "active", actionStatus)
					if actionStatus == "active" {
						iMt = mts[i]
						return true, nil
					}
				}
			}
			return false, nil
		})
	}
	if shareId == -1 {
		self.taskFailed(ctx, mta, fmt.Errorf("share id with value -1"))
		return
	}
	acls, err := iFs.(*xgfs.SXgfsFileSystem).GetMountTargetAcls(shareId)
	if err != nil {
		self.taskFailed(ctx, mta, errors.Wrapf(err, "iFs.GetMountTargetAcls"))
		return
	}
	result := fs.SyncMountTargetAclsBySource(ctx, self.GetUserCred(), acls)
	log.Infof("SyncMountTargetAclsBySource for FileSystem %s in adding acl, result: %s", fs.Name, result.Result())

	mta.SetStatus(self.UserCred, api.MOUNT_TARGET_ACL_STATUS_AVAILABLE, "")
	self.SetStageComplete(ctx, nil)
}
