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
	"strconv"
	"time"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/multicloud/nas/xgfs"

	"yunion.io/x/jsonutils"
	"yunion.io/x/pkg/errors"

	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/compute/models"
	"yunion.io/x/onecloud/pkg/util/logclient"
)

type MountTargetAclDeleteTask struct {
	taskman.STask
}

func init() {
	taskman.RegisterTask(MountTargetAclDeleteTask{})
}

func (self *MountTargetAclDeleteTask) taskFailed(ctx context.Context, mta *models.SMountTargetAcl, err error) {
	mta.SetStatus(self.UserCred, api.MOUNT_TARGET_ACL_STATUS_REMOVE_FAILED, err.Error())
	logclient.AddActionLogWithStartable(self, mta, logclient.ACT_DELOCATE, err, self.UserCred, false)
	self.SetStageFailed(ctx, jsonutils.NewString(err.Error()))
}

func (self *MountTargetAclDeleteTask) OnInit(ctx context.Context, obj db.IStandaloneModel, body jsonutils.JSONObject) {
	mta := obj.(*models.SMountTargetAcl)

	if len(mta.ExternalId) == 0 {
		self.taskComplete(ctx, nil, mta)
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

	self.SetStage("on_wait_acl_ready", nil)
	self.OnSyncMountTargetAclComplete(ctx, fs, ifs, mta, nil)
	//self.taskComplete(ctx, fs, mt)
}

func (self *MountTargetAclDeleteTask) taskComplete(ctx context.Context, fs *models.SFileSystem, mta *models.SMountTargetAcl) {
	if fs != nil {
		logclient.AddActionLogWithStartable(self, fs, logclient.ACT_DELOCATE, mta, self.UserCred, true)
	}
	mta.RealDelete(ctx, self.GetUserCred())
	self.SetStageComplete(ctx, nil)
}

func (self *MountTargetAclDeleteTask) OnSyncMountTargetAclComplete(ctx context.Context, fs *models.SFileSystem, iFs cloudprovider.ICloudFileSystem, mta *models.SMountTargetAcl, data jsonutils.JSONObject) {
	mountTargets, err := iFs.GetMountTargets()
	if err != nil {
		self.taskFailed(ctx, mta, errors.Wrapf(err, "ifs.GetMountTargets"))
		return
	}
	// opts

	if len(mountTargets) == 0 {
		self.taskComplete(ctx, fs, mta)
		return
	}

	shareId := mountTargets[0].(*xgfs.SDfsNfsShare).GetId()
	acls, err := iFs.(*xgfs.SXgfsFileSystem).GetMountTargetAcls(shareId)
	if err != nil {
		self.taskFailed(ctx, mta, errors.Wrapf(err, "iFs.GetMountTargetAcls"))
		return
	}
	// impossible in xgfs
	if len(acls) == 0 {
		self.taskComplete(ctx, fs, mta)
		return
	}

	if len(acls) == 1 {
		if acls[0].GetSource() == mta.Source {
			// the last one acl, so delete dfs share
			_, err := iFs.(*xgfs.SXgfsFileSystem).DeleteDfsNfsShare(shareId)
			if err != nil {
				self.taskFailed(ctx, mta, errors.Wrapf(err, "iFs.DeleteDfsNfsShare"))
				return
			}
			cloudprovider.Wait(time.Second*3, time.Minute*10, func() (bool, error) {
				mts, err := iFs.GetMountTargets()
				if err != nil {
					return false, errors.Wrapf(err, "iFs.GetMountTargets")
				}
				if len(mts) == 0 {
					return true, nil
				}
				//for i := range mts {
				//	if mts[i].GetGlobalId() == iMt.GetGlobalId() {
				//		status := mts[i].GetStatus()
				//		log.Infof("expect mount point status %s current is %s", api.MOUNT_TARGET_STATUS_DELETING, status)
				//		if status == api.MOUNT_TARGET_STATUS_DELETING {
				//			iMt = mts[i]
				//			return true, nil
				//		}
				//	}
				//}
				return false, nil
			})
		} else {
			log.Warningf("the last acl not %s ???, is %s", mta.Source, acls[0].GetSource())
			self.taskComplete(ctx, fs, mta)
			return
		}
	} else {
		// remove dfs shares acl
		aclId, err := strconv.Atoi(mta.ExternalId)
		if err != nil {
			self.taskFailed(ctx, mta, errors.Wrapf(err, "%s strconv.Atoi err", mta.ExternalId))
			return
		}
		iMt, err := iFs.(*xgfs.SXgfsFileSystem).RemoveDfsNfsShareAcl(aclId, shareId)
		if err != nil {
			self.taskFailed(ctx, mta, errors.Wrapf(err, "iFs.RemoveDfsNfsShareAcl"))
			return
		}
		cloudprovider.Wait(time.Second*3, time.Minute*10, func() (bool, error) {
			mts, err := iFs.GetMountTargets()
			if err != nil {
				return false, errors.Wrapf(err, "iFs.GetMountTargets")
			}
			for i := range mts {
				if mts[i].GetGlobalId() == iMt.GetGlobalId() {
					actionStatus := mts[i].(*xgfs.SDfsNfsShare).GetActionStatus() // dfs_share_acl_removing
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
	//if shareId == -1 {
	//	self.taskFailed(ctx, mta, fmt.Errorf("share id with value -1"))
	//	return
	//}
	//acls, err := iFs.(*xgfs.SXgfsFileSystem).GetMountTargetAcls(shareId)
	//if err != nil {
	//	self.taskFailed(ctx, mta, errors.Wrapf(err, "iFs.GetMountTargetAcls"))
	//	return
	//}
	//result := fs.SyncMountTargetAcls(ctx, self.GetUserCred(), acls)
	//log.Infof("SyncMountTargetAcls for FileSystem %s in adding acl, result: %s", fs.Name, result.Result())

	//mta.SetStatus(self.UserCred, api.NAS_STATUS_AVAILABLE, "")
	self.taskComplete(ctx, fs, mta)
}
