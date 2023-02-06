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
	"reflect"
	"strconv"
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/pkg/errors"

	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/cloudcommon/notifyclient"
	"yunion.io/x/onecloud/pkg/compute/models"
	"yunion.io/x/onecloud/pkg/util/logclient"
)

type GuestMigrateStorageTask struct {
	SGuestBaseTask
}

func init() {
	taskman.RegisterTask(GuestMigrateStorageTask{})
}

func (self *GuestMigrateStorageTask) OnInit(ctx context.Context, obj db.IStandaloneModel, body jsonutils.JSONObject) {
	guest := obj.(*models.SGuest)
	//guest.SetStatus(self.UserCred, api.VM_DISK_CHANGE_STORAGE, "")
	self.StartChangeDisksStorage2Nfs(ctx, guest, body)
}

func (self *GuestMigrateStorageTask) OnGuestChangeDisksStorage2NfsReady(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	guest := obj.(*models.SGuest)
	if guest.Status == api.VM_DISK_CHANGE_STORAGE_START_STAGE1 {
		// 没有经历过NFS?
		//guest.SetStatus(self.UserCred, api.VM_READY, "")
	}
	self.SetStage("OnNormalMigrate", nil)
	// do migrate check
	if !guest.GetDriver().IsSupportMigrate() {
		msg := fmt.Sprintf("Not allow for hypervisor %s", guest.GetHypervisor())
		self.OnNormalMigrateFailed(ctx, guest, jsonutils.NewString(msg))
		return
	}
	migrateInput := new(api.GuestMigrateInput)
	migrateInput.AutoStart = false
	//migrateInput.IsRescueMode = false
	migrateInput.PreferHostId, _ = self.Params.GetString("prefer_host_id")
	migrateInput.IsRescueMode, _ = data.Bool("is_rescue_mode")
	//migrateInput.PreferHostId, _ = data.GetString("prefer_host_id")
	if guest.Status != api.VM_DISK_CHANGE_STORAGE_START_STAGE1 && guest.Status != api.VM_READY {
		//if !migrateInput.IsRescueMode && guest.Status != api.VM_READY {
		//	msg := fmt.Sprintf("Cannot normal migrate guest in status %s, try rescue mode?", guest.Status)
		msg := fmt.Sprintf("Cannot normal migrate guest in status %s.", guest.Status)
		self.OnNormalMigrateFailed(ctx, guest, jsonutils.NewString(msg))
		return
	}
	if err := guest.GetDriver().CheckMigrate(ctx, guest, self.GetUserCred(), *migrateInput); err != nil {
		msg := fmt.Sprintf("normal migrate guest %s err %v", guest.GetName(), err)
		self.OnNormalMigrateFailed(ctx, guest, jsonutils.NewString(msg))
		return
	}

	guest.StartMigrateTask(ctx, self.GetUserCred(), migrateInput.IsRescueMode, migrateInput.AutoStart, guest.Status, migrateInput.PreferHostId, self.GetId())
}

func (self *GuestMigrateStorageTask) OnNormalMigrateFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	guest := obj.(*models.SGuest)
	guest.SetStatus(self.UserCred, api.VM_MIGRATE_FAILED, data.String())
	guest.RemoveMetadata(ctx, api.VM_DISK_CHANGE_STORAGE, self.GetUserCred())
	db.OpsLog.LogEvent(guest, db.ACT_MIGRATE_FAIL, data, self.UserCred)
	logclient.AddActionLogWithStartable(self, guest, logclient.ACT_MIGRATE, data, self.UserCred, false)
	notifyclient.EventNotify(ctx, self.GetUserCred(), notifyclient.SEventNotifyParam{
		Obj:    guest,
		Action: notifyclient.ActionCreate,
		IsFail: true,
	})

	self.SetStageFailed(ctx, data)
}

func (self *GuestMigrateStorageTask) OnNormalMigrate(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	guest := obj.(*models.SGuest)
	//targetStorageId, _ := self.Params.GetString("target_storage_id")
	guest.SetStatus(self.UserCred, api.VM_DISK_CHANGE_STORAGE_START_STAGE2, "")
	self.StartChangeDisksStorage2Target(ctx, guest, data)
}

func (self *GuestMigrateStorageTask) OnGuestMigrateStorageComplete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	guest := obj.(*models.SGuest)
	log.Infof("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
	log.Infof("MigrateStorageComplete %s", guest.Name)
	log.Infof("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
	//guest.SetStatus(self.UserCred, api.VM_DEPLOYING, "")
	guest.RemoveMetadata(ctx, api.VM_DISK_CHANGE_STORAGE, self.GetUserCred())
	if jsonutils.QueryBoolean(self.GetParams(), "auto_start", false) {
		self.SetStage("on_auto_start_guest", nil)
		guest.StartGueststartTask(ctx, self.GetUserCred(), nil, self.GetTaskId())
	} else {
		self.SetStage("on_sync_status_complete", nil)
		guest.StartSyncstatus(ctx, self.GetUserCred(), self.GetTaskId())
	}
}

func (self *GuestMigrateStorageTask) OnGuestMigrateStorageCompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	guest := obj.(*models.SGuest)
	guest.SetStatus(self.UserCred, api.VM_MIGRATE_FAILED, "")
	guest.RemoveMetadata(ctx, api.VM_DISK_CHANGE_STORAGE, self.GetUserCred())
	db.OpsLog.LogEvent(guest, db.ACT_MIGRATE_FAIL, data, self.UserCred)
	logclient.AddActionLogWithStartable(self, guest, logclient.ACT_MIGRATE, data, self.UserCred, false)
	notifyclient.EventNotify(ctx, self.GetUserCred(), notifyclient.SEventNotifyParam{
		Obj:    guest,
		Action: notifyclient.ActionCreate,
		IsFail: true,
	})
	self.SetStageFailed(ctx, data)
}

func (self *GuestMigrateStorageTask) StartChangeDisksStorage2Nfs(ctx context.Context, guest *models.SGuest, data jsonutils.JSONObject) {
	self.SetStage("OnGuestChangeSysDiskStorageStage1Complete", nil)
	sysDisk, err := guest.GetSystemDisk()
	if err != nil {
		self.TaskFailed(ctx, guest, api.VM_MIGRATE_FAILED, jsonutils.NewString(fmt.Sprintf("GetSystemDisks error: %v", err)))
		return
	}
	//NFS storage
	nfsStorageId, _ := data.GetString("nfs_storage_id")

	input := &api.ServerChangeDiskStorageInput{
		DiskId:          sysDisk.Id,
		TargetStorageId: nfsStorageId,
		KeepOriginDisk:  false,
	}
	if sysDisk.StorageId == nfsStorageId {
		log.Warningf("system disk %s already exists in nfs storage %s", sysDisk.GetName(), nfsStorageId)
		self.OnGuestChangeSysDiskStorageStage1Complete(ctx, guest, data)
		return
	}
	//guest.SetStatus(self.GetUserCred(), api.VM_DISK_CHANGE_STORAGE, "")
	guest.SetMetadata(ctx, api.VM_DISK_CHANGE_STORAGE, "sys_stage1", self.GetUserCred())
	err = self.startChangeDiskStorage(ctx, self.GetUserCred(), guest, input)
	if err != nil {
		self.TaskFailed(ctx, guest, api.VM_MIGRATE_FAILED, jsonutils.NewString(fmt.Sprintf("start change disk %s to nfs storage error: %v", sysDisk.GetName(), err)))
		return
	}
	//if len(disks) == count {
	//	log.Infof("all %d disks already exists in nfs storage %s", count, nfsStorageId)
	//	//主动触发
	//	self.OnGuestChangeDisksStorage2NfsReady(ctx, guest, data)
	//}
}

// sys disk
func (self *GuestMigrateStorageTask) OnGuestChangeSysDiskStorageStage1Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage1(ctx, obj, data, 1)
	//guest := obj.(*models.SGuest)
	//guest.SetStatus(self.UserCred, api.VM_START_MIGRATE, "")
	//disks, err := guest.GetDisks()
	//if err != nil {
	//	self.TaskFailed(ctx, guest, "OnGuestChangeSysDiskStorageStage1Complete", jsonutils.NewString(fmt.Sprintf("GetDisks error: %v", err)))
	//	return
	//}

	//if len(disks) < 2 {
	//	self.OnGuestChangeDisksStorage2NfsReady(ctx, guest, data)
	//	return
	//}
	//self.SetStage("OnGuestChangeDataDisk1StorageStage1Complete", nil)
	//disk1 := disks[1]
	//if disk1.DiskType != "data" {
	//	self.TaskFailed(ctx, guest, "OnGuestChangeDataDisk1StorageStage1Complete", jsonutils.NewString(fmt.Sprintf("disk %s is not data type, is type %s", disk1.GetName(), disk1.DiskType)))
	//	return
	//}
	////NFS storage
	//nfsStorageId, _ := data.GetString("nfs_storage_id")
	//input := &api.ServerChangeDiskStorageInput{
	//	DiskId:          disk1.Id,
	//	TargetStorageId: nfsStorageId,
	//	KeepOriginDisk:  false,
	//}
	//if disk1.StorageId == nfsStorageId {
	//	log.Warningf("data disk %s already exists in nfs storage %s", disk1.GetName(), nfsStorageId)
	//	self.OnGuestChangeDataDisk1StorageStage1Complete(ctx, guest, data)
	//	return
	//}
	//guest.SetMetadata(ctx, api.VM_DISK_CHANGE_STORAGE, "data1_stage1", self.GetUserCred())
	//err = self.startChangeDiskStorage(ctx, self.GetUserCred(), guest, input)
	//if err != nil {
	//	guest.RemoveMetadata(ctx, api.VM_DISK_CHANGE_STORAGE, self.GetUserCred())
	//	self.TaskFailed(ctx, guest, "OnGuestChangeDataDisk1StorageStage1Complete", jsonutils.NewString(fmt.Sprintf("start change disk %s to nfs storage error: %v", disk.GetName(), err)))
	//	return
	//}
}

//data disk1 stage 1
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk1StorageStage1Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage1(ctx, obj, data, 2)
}

//data disk2 stage 1
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk2StorageStage1Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage1(ctx, obj, data, 3)
}

//data disk3 stage 1
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk3StorageStage1Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage1(ctx, obj, data, 4)
}

//data disk4 stage 1
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk4StorageStage1Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage1(ctx, obj, data, 5)
}

//data disk5 stage 1
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk5StorageStage1Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage1(ctx, obj, data, 6)
}

//data disk6 stage 1
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk6StorageStage1Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage1(ctx, obj, data, 7)
}

//data disk7 stage 1
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk7StorageStage1Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage1(ctx, obj, data, 8)
}

//data disk8 stage 1
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk8StorageStage1Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage1(ctx, obj, data, 9)
}

//data disk9 stage 1
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk9StorageStage1Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage1(ctx, obj, data, 10)
}

//data disk10 stage 1
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk10StorageStage1Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage1(ctx, obj, data, 11)
}

//data disk11 stage 1
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk11StorageStage1Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage1(ctx, obj, data, 12)
}

//data disk12 stage 1
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk12StorageStage1Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDisksStorage2NfsReady(ctx, obj, data)
}

func (self *GuestMigrateStorageTask) OnGuestChangeDataDiskXStorageStage1(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject, index int) {
	//currentMethod := "OnGuestChangeDataDisk" + strconv.Itoa(index-1) + "StorageStage1Complete"
	//if index == 1 {
	//	currentMethod = "OnGuestChangeSysDiskStorageStage1Complete"
	//}
	stage := "OnGuestChangeDataDisk" + strconv.Itoa(index) + "StorageStage1Complete"
	//nextStage := "OnGuestChangeDataDisk" + strconv.Itoa(index+1) + "StorageStage1Complete"
	guest := obj.(*models.SGuest)
	disks, err := guest.GetDisks()
	if err != nil {
		self.TaskFailed(ctx, guest, api.VM_MIGRATE_FAILED, jsonutils.NewString(fmt.Sprintf("GetDisks error: %v", err)))
		return
	}

	if len(disks) < index+1 {
		self.OnGuestChangeDisksStorage2NfsReady(ctx, guest, data)
		return
	}
	self.SetStage(stage, nil)
	diskX := disks[index]
	if diskX.DiskType != "data" {
		self.TaskFailed(ctx, guest, api.VM_MIGRATE_FAILED, jsonutils.NewString(fmt.Sprintf("disk %s is not data type???, is type %s", diskX.GetName(), diskX.DiskType)))
		return
	}
	//NFS storage
	//nfsStorageId, _ := data.GetString("nfs_storage_id")
	nfsStorageId, _ := self.Params.GetString("nfs_storage_id")
	input := &api.ServerChangeDiskStorageInput{
		DiskId:          diskX.Id,
		TargetStorageId: nfsStorageId,
		KeepOriginDisk:  false,
	}
	if diskX.StorageId == nfsStorageId {
		log.Warningf("data disk %s already exists in nfs storage %s", diskX.GetName(), nfsStorageId)
		//self.OnGuestChangeDataDisk1StorageStage1Complete(ctx, guest, data)
		reflect.ValueOf(self).MethodByName(stage).Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(obj), reflect.ValueOf(data)})
		return
	}
	metaValue := fmt.Sprintf("data%d_stage1", index)
	guest.SetStatus(self.GetUserCred(), api.VM_DISK_CHANGE_STORAGE, "")
	guest.SetMetadata(ctx, api.VM_DISK_CHANGE_STORAGE, metaValue, self.GetUserCred())
	err = self.startChangeDiskStorage(ctx, self.GetUserCred(), guest, input)
	if err != nil {
		//guest.RemoveMetadata(ctx, api.VM_DISK_CHANGE_STORAGE, self.GetUserCred())
		self.TaskFailed(ctx, guest, api.VM_MIGRATE_FAILED, jsonutils.NewString(fmt.Sprintf("start change disk %s to nfs storage error: %v", diskX.GetName(), err)))
		return
	}
}

func (self *GuestMigrateStorageTask) StartChangeDisksStorage2Target(ctx context.Context, guest *models.SGuest, data jsonutils.JSONObject) {
	self.SetStage("OnGuestChangeSysDiskStorageStage2Complete", nil)
	sysDisk, err := guest.GetSystemDisk()
	if err != nil {
		self.TaskFailed(ctx, guest, api.VM_DISK_CHANGE_STORAGE_FAIL, jsonutils.NewString(fmt.Sprintf("GetSystemDisks error: %v", err)))
		return
	}
	// target storage
	targetStorageId, _ := self.Params.GetString("target_storage_id")
	input := &api.ServerChangeDiskStorageInput{
		DiskId:          sysDisk.Id,
		TargetStorageId: targetStorageId,
		KeepOriginDisk:  false,
	}
	if sysDisk.StorageId == targetStorageId {
		log.Warningf("system disk %s already exists in target storage %s", sysDisk.GetName(), targetStorageId)
		self.OnGuestChangeSysDiskStorageStage2Complete(ctx, guest, data)
		return
	}
	//guest.SetStatus(self.GetUserCred(), api.VM_DISK_CHANGE_STORAGE, "")
	guest.SetMetadata(ctx, api.VM_DISK_CHANGE_STORAGE, "sys_stage2", self.GetUserCred())
	err = self.startChangeDiskStorage(ctx, self.GetUserCred(), guest, input)
	if err != nil {
		//guest.RemoveMetadata(ctx, api.VM_DISK_CHANGE_STORAGE, self.GetUserCred())
		self.TaskFailed(ctx, guest, api.VM_DISK_CHANGE_STORAGE_FAIL, jsonutils.NewString(fmt.Sprintf("start change disk %s to nfs storage error: %v", sysDisk.GetName(), err)))
		return
	}
}

// sys disk
func (self *GuestMigrateStorageTask) OnGuestChangeSysDiskStorageStage2Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage2(ctx, obj, data, 1)
}

//data disk1 stage 2
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk1StorageStage2Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage2(ctx, obj, data, 2)
}

//data disk2 stage 2
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk2StorageStage2Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage2(ctx, obj, data, 3)
}

//data disk3 stage 2
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk3StorageStage2Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage2(ctx, obj, data, 4)
}

//data disk4 stage 2
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk4StorageStage2Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage2(ctx, obj, data, 5)
}

//data disk5 stage 2
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk5StorageStage2Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage2(ctx, obj, data, 6)
}

//data disk6 stage 2
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk6StorageStage2Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage2(ctx, obj, data, 7)
}

//data disk7 stage 2
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk7StorageStage2Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage2(ctx, obj, data, 8)
}

//data disk8 stage 2
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk8StorageStage2Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage2(ctx, obj, data, 9)
}

//data disk9 stage 2
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk9StorageStage2Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage2(ctx, obj, data, 10)
}

//data disk10 stage 2
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk10StorageStage2Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage2(ctx, obj, data, 11)
}

//data disk11 stage 2
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk11StorageStage2Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestChangeDataDiskXStorageStage2(ctx, obj, data, 12)
}

//data disk12 stage 2
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk12StorageStage2Complete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageComplete(ctx, obj, data)
}

func (self *GuestMigrateStorageTask) OnGuestChangeDataDiskXStorageStage2(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject, index int) {
	//currentMethod := "OnGuestChangeDataDisk" + strconv.Itoa(index-1) + "StorageStage2Complete"
	//if index == 1 {
	//	currentMethod = "OnGuestChangeSysDiskStorageStage2Complete"
	//}
	stage := "OnGuestChangeDataDisk" + strconv.Itoa(index) + "StorageStage2Complete"
	guest := obj.(*models.SGuest)
	disks, err := guest.GetDisks()
	if err != nil {
		self.TaskFailed(ctx, guest, api.VM_DISK_CHANGE_STORAGE_FAIL, jsonutils.NewString(fmt.Sprintf("GetDisks error: %v", err)))
		return
	}

	if len(disks) < index+1 {
		self.OnGuestMigrateStorageComplete(ctx, guest, data)
		return
	}
	self.SetStage(stage, nil)
	diskX := disks[index]
	if diskX.DiskType != "data" {
		self.TaskFailed(ctx, guest, api.VM_DISK_CHANGE_STORAGE_FAIL, jsonutils.NewString(fmt.Sprintf("disk %s is not data type???, is type %s", diskX.GetName(), diskX.DiskType)))
		return
	}
	// target storage
	targetStorageId, _ := self.Params.GetString("target_storage_id")
	input := &api.ServerChangeDiskStorageInput{
		DiskId:          diskX.Id,
		TargetStorageId: targetStorageId,
		KeepOriginDisk:  false,
	}
	if diskX.StorageId == targetStorageId {
		log.Warningf("data disk %s already exists in target storage %s", diskX.GetName(), targetStorageId)
		//self.OnGuestChangeDataDisk1StorageStage1Complete(ctx, guest, data)
		reflect.ValueOf(self).MethodByName(stage).Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(obj), reflect.ValueOf(data)})
		return
	}
	metaValue := fmt.Sprintf("data%d_stage2", index)
	guest.SetStatus(self.GetUserCred(), api.VM_DISK_CHANGE_STORAGE, "")
	guest.SetMetadata(ctx, api.VM_DISK_CHANGE_STORAGE, metaValue, self.GetUserCred())
	//reason := fmt.Sprintf("Change disk %s to storage %s", input.DiskId, input.TargetStorageId)
	err = self.startChangeDiskStorage(ctx, self.GetUserCred(), guest, input)
	if err != nil {
		//guest.RemoveMetadata(ctx, api.VM_DISK_CHANGE_STORAGE, self.GetUserCred())
		self.TaskFailed(ctx, guest, api.VM_DISK_CHANGE_STORAGE_FAIL, jsonutils.NewString(fmt.Sprintf("start change disk %s to nfs storage error: %v", diskX.GetName(), err)))
		return
	}
}

func (self *GuestMigrateStorageTask) startChangeDiskStorage(ctx context.Context, userCred mcclient.TokenCredential, guest *models.SGuest, input *api.ServerChangeDiskStorageInput) error {
	// validate input
	if input.DiskId == "" {
		return httperrors.NewNotEmptyError("Disk id is empty")
	}
	if input.TargetStorageId == "" {
		return httperrors.NewNotEmptyError("Storage id is empty")
	}

	// validate disk
	//var srcDisk *SDisk
	srcDiskObj, err := models.DiskManager.FetchById(input.DiskId)
	if err != nil {
		return httperrors.NewNotFoundError("Disk %s not found on server %s", input.DiskId, self.GetName())
	}
	srcDisk := srcDiskObj.(*models.SDisk)

	// validate storage
	storageObj, err := models.StorageManager.FetchByIdOrName(userCred, input.TargetStorageId)
	if err != nil {
		return errors.Wrapf(err, "Found storage by %s", input.TargetStorageId)
	}
	storage := storageObj.(*models.SStorage)
	input.TargetStorageId = storage.GetId()

	// driver validate
	drv := guest.GetDriver()
	if err := drv.ValidateChangeDiskStorage(ctx, userCred, guest, input); err != nil {
		return err
	}

	// create a disk on target storage from source disk
	diskConf := &api.DiskConfig{
		Index:    -1,
		ImageId:  srcDisk.TemplateId,
		SizeMb:   srcDisk.DiskSize,
		Fs:       srcDisk.FsFormat,
		DiskType: srcDisk.DiskType,
	}

	targetDisk, err := guest.CreateDiskOnStorage(ctx, userCred, storage, diskConf, nil, true, true)
	if err != nil {
		return errors.Wrapf(err, "Create target disk on storage %s", storage.GetName())
	}

	internalInput := &api.ServerChangeDiskStorageInternalInput{
		ServerChangeDiskStorageInput: *input,
		StorageId:                    srcDisk.StorageId,
		TargetDiskId:                 targetDisk.GetId(),
		GuestRunning:                 guest.Status == api.VM_RUNNING,
	}

	return guest.StartChangeDiskStorageTask(ctx, userCred, internalInput, self.GetTaskId())
}

func (self *GuestMigrateStorageTask) OnDeployEipCompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	guest := obj.(*models.SGuest)
	guest.SetStatus(self.UserCred, api.INSTANCE_ASSOCIATE_EIP_FAILED, "deploy_failed")
	db.OpsLog.LogEvent(guest, db.ACT_EIP_ATTACH, data, self.UserCred)
	logclient.AddActionLogWithStartable(self, guest, logclient.ACT_EIP_ASSOCIATE, data, self.UserCred, false)
	notifyclient.NotifySystemErrorWithCtx(ctx, guest.Id, guest.Name, api.INSTANCE_ASSOCIATE_EIP_FAILED, data.String())
	self.SetStageFailed(ctx, data)
}

func (self *GuestMigrateStorageTask) OnAutoStartGuest(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	guest := obj.(*models.SGuest)
	self.TaskComplete(ctx, guest)
}

func (self *GuestMigrateStorageTask) OnSyncStatusComplete(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	guest := obj.(*models.SGuest)
	self.TaskComplete(ctx, guest)
}

func (self *GuestMigrateStorageTask) TaskComplete(ctx context.Context, guest *models.SGuest) {
	guest.RemoveMetadata(ctx, api.VM_DISK_CHANGE_STORAGE, self.GetUserCred())
	db.OpsLog.LogEvent(guest, db.ACT_ALLOCATE, "", self.UserCred)
	logclient.AddActionLogWithContext(ctx, guest, logclient.ACT_ALLOCATE, "", self.UserCred, true)
	self.SetStageComplete(ctx, guest.GetShortDesc(ctx))
}

func (self *GuestMigrateStorageTask) TaskFailed(ctx context.Context, guest *models.SGuest, status string, reason jsonutils.JSONObject) {
	guest.SetStatus(self.GetUserCred(), status, reason.String())
	guest.RemoveMetadata(ctx, api.VM_DISK_CHANGE_STORAGE, self.GetUserCred())
	logclient.AddActionLogWithStartable(self, guest, logclient.ACT_MIGRATE, reason, self.GetUserCred(), false)
	self.SetStageFailed(ctx, reason)
}

// Stage 1 failed
func (self *GuestMigrateStorageTask) OnGuestChangeSysDiskStorageStage1CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk1StorageStage1CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk2StorageStage1CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk3StorageStage1CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk4StorageStage1CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk5StorageStage1CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk6StorageStage1CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk7StorageStage1CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk8StorageStage1CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk9StorageStage1CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk10StorageStage1CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk11StorageStage1CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk12StorageStage1CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}

// Stage 2 failed
func (self *GuestMigrateStorageTask) OnGuestChangeSysDiskStorageStage2CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk1StorageStage2CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk2StorageStage2CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk3StorageStage2CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk4StorageStage2CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk5StorageStage2CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk6StorageStage2CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk7StorageStage2CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk8StorageStage2CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk9StorageStage2CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk10StorageStage2CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk11StorageStage2CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
func (self *GuestMigrateStorageTask) OnGuestChangeDataDisk12StorageStage2CompleteFailed(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	self.OnGuestMigrateStorageCompleteFailed(ctx, obj, data)
}
