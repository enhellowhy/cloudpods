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
	"fmt"
	"strings"
	"time"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
	"yunion.io/x/onecloud/pkg/mcclient/modules/identity"
	"yunion.io/x/onecloud/pkg/mcclient/modules/thirdparty"
	"yunion.io/x/pkg/gotypes"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/util/compare"
	"yunion.io/x/pkg/utils"
	"yunion.io/x/sqlchemy"

	billing_api "yunion.io/x/onecloud/pkg/apis/billing"
	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/lockman"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/cloudcommon/notifyclient"
	"yunion.io/x/onecloud/pkg/cloudcommon/validators"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/compute/options"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/util/billing"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
)

type SFileSystemManager struct {
	//db.SStatusInfrasResourceBaseManager
	db.SSharableVirtualResourceBaseManager
	db.SExternalizedResourceBaseManager
	SManagedResourceBaseManager
	SCloudregionResourceBaseManager
	SZoneResourceBaseManager

	SDeletePreventableResourceBaseManager
}

var FileSystemManager *SFileSystemManager

func init() {
	FileSystemManager = &SFileSystemManager{
		SSharableVirtualResourceBaseManager: db.NewSharableVirtualResourceBaseManager(
			SFileSystem{},
			"file_systems_tbl",
			"file_system",
			"file_systems",
		),
	}
	FileSystemManager.SetVirtualObject(FileSystemManager)
}

type SFileSystem struct {
	//db.SStatusInfrasResourceBase
	db.SSharableVirtualResourceBase
	db.SExternalizedResourceBase
	SManagedResourceBase
	SBillingResourceBase
	SCloudregionResourceBase
	SZoneResourceBase

	SDeletePreventableResourceBase

	// 文件系统类型
	// enmu: extreme, standard, cpfs
	FileSystemType string `width:"32" charset:"ascii" nullable:"false" list:"user" create:"required"`

	// 存储类型
	// enmu: performance, capacity, standard, advance, advance_100, advance_200
	StorageType string `width:"32" charset:"ascii" nullable:"false" list:"user" create:"required"`
	// 协议类型
	// enum: NFS, SMB, cpfs
	Protocol string `width:"32" charset:"ascii" nullable:"false" list:"user" create:"required"`
	// 容量, 单位Bytes
	Capacity int64 `nullable:"false" list:"user" create:"optional"`
	// 已使用容量, 单位Bytes
	UsedCapacity int64 `nullable:"false" list:"user"`
	// 文件数量
	Files int64 `nullable:"false" default:"0" list:"user"`
	// 文件配额
	FilesQuota int64 `nullable:"false" default:"0" list:"user"`

	// 最多支持挂载点数量, -1代表无限制
	MountTargetCountLimit int `nullable:"false" list:"user" default:"-1"`
}

func (manager *SFileSystemManager) GetContextManagers() [][]db.IModelManager {
	return [][]db.IModelManager{
		{CloudregionManager},
	}
}

func (self *SFileSystem) GetCloudproviderId() string {
	return self.ManagerId
}

func (manager *SFileSystemManager) ListItemFilter(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.FileSystemListInput,
) (*sqlchemy.SQuery, error) {
	var err error
	//q, err = manager.SStatusInfrasResourceBaseManager.ListItemFilter(ctx, q, userCred, query.StatusInfrasResourceBaseListInput)
	q, err = manager.SSharableVirtualResourceBaseManager.ListItemFilter(ctx, q, userCred, query.SharableVirtualResourceListInput)
	if err != nil {
		return nil, errors.Wrapf(err, "SStatusInfrasResourceBaseManager.ListItemFilter")
	}
	q, err = manager.SExternalizedResourceBaseManager.ListItemFilter(ctx, q, userCred, query.ExternalizedResourceBaseListInput)
	if err != nil {
		return nil, errors.Wrapf(err, "SExternalizedResourceBaseManager.ListItemFilter")
	}
	q, err = manager.SManagedResourceBaseManager.ListItemFilter(ctx, q, userCred, query.ManagedResourceListInput)
	if err != nil {
		return nil, errors.Wrapf(err, "SManagedResourceBaseManager.ListItemFilter")
	}
	q, err = manager.SCloudregionResourceBaseManager.ListItemFilter(ctx, q, userCred, query.RegionalFilterListInput)
	if err != nil {
		return nil, errors.Wrapf(err, "SCloudregionResourceBaseManager.ListItemFilter")
	}
	return q, nil
}

func (man *SFileSystemManager) ValidateCreateData(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, input api.FileSystemCreateInput) (api.FileSystemCreateInput, error) {
	var err error
	if len(input.NetworkId) > 0 {
		net, err := validators.ValidateModel(userCred, NetworkManager, &input.NetworkId)
		if err != nil {
			return input, err
		}
		network := net.(*SNetwork)
		vpc, _ := network.GetVpc()
		input.ManagerId = vpc.ManagerId
		if zone, _ := network.GetZone(); zone != nil {
			input.ZoneId = zone.Id
			input.CloudregionId = zone.CloudregionId
		}
	}
	if input.CloudEnv == "onpremise" {
		input.CloudregionId = "default"
	} else {
		if len(input.ZoneId) == 0 {
			return input, httperrors.NewMissingParameterError("zone_id")
		}
		_zone, err := validators.ValidateModel(userCred, ZoneManager, &input.ZoneId)
		if err != nil {
			return input, err
		}
		zone := _zone.(*SZone)
		region, _ := zone.GetRegion()
		input.CloudregionId = region.Id
	}

	if len(input.ManagerId) == 0 {
		return input, httperrors.NewMissingParameterError("manager_id")
	}

	if input.Capacity > 0 {
		input.Capacity = input.Capacity * 1024 * 1024 * 1024
	}

	if len(input.Duration) > 0 {
		billingCycle, err := billing.ParseBillingCycle(input.Duration)
		if err != nil {
			return input, httperrors.NewInputParameterError("invalid duration %s", input.Duration)
		}

		if !utils.IsInStringArray(input.BillingType, []string{billing_api.BILLING_TYPE_PREPAID, billing_api.BILLING_TYPE_POSTPAID}) {
			//input.BillingType = billing_api.BILLING_TYPE_PREPAID
			input.BillingType = billing_api.BILLING_TYPE_POSTPAID
		}

		//if input.BillingType == billing_api.BILLING_TYPE_PREPAID {
		//	if !region.GetDriver().IsSupportedBillingCycle(billingCycle, man.KeywordPlural()) {
		//		return input, httperrors.NewInputParameterError("unsupported duration %s", input.Duration)
		//	}
		//}
		tm := time.Time{}
		input.BillingCycle = billingCycle.String()
		input.ExpiredAt = billingCycle.EndAt(tm)
	}

	//input.StatusInfrasResourceBaseCreateInput, err = man.SStatusInfrasResourceBaseManager.ValidateCreateData(ctx, userCred, ownerId, query, input.StatusInfrasResourceBaseCreateInput)
	input.SharableVirtualResourceCreateInput, err = man.SSharableVirtualResourceBaseManager.ValidateCreateData(ctx, userCred, ownerId, query, input.SharableVirtualResourceCreateInput)
	if err != nil {
		return input, err
	}
	return input, nil
}

func (self *SFileSystem) PostCreate(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data jsonutils.JSONObject) {
	self.SSharableVirtualResourceBase.PostCreate(ctx, userCred, ownerId, query, data)
	userId, _ := data.GetString("user_id")
	if len(userId) > 0 {
		self.setUserId(ctx, userCred, userId)
	}

	// cloud_env is onpremise, network_id is empty
	self.StartCreateTask(ctx, userCred, jsonutils.GetAnyString(data, []string{"network_id"}), "")
}

func (self *SFileSystem) setUserId(ctx context.Context, userCred mcclient.TokenCredential, id string) error {
	err := self.SetMetadata(ctx, "user_id", id, userCred)
	if err != nil {
		return err
	}
	return nil
}

func (self *SFileSystem) StartCreateTask(ctx context.Context, userCred mcclient.TokenCredential, networkId string, parentTaskId string) error {
	var err = func() error {
		params := jsonutils.NewDict()
		params.Add(jsonutils.NewString(networkId), "network_id")
		task, err := taskman.TaskManager.NewTask(ctx, "FileSystemCreateTask", self, userCred, params, parentTaskId, "", nil)
		if err != nil {
			return errors.Wrapf(err, "NewTask")
		}
		return task.ScheduleRun(nil)
	}()
	if err != nil {
		self.SetStatus(userCred, api.NAS_STATUS_CREATE_FAILED, err.Error())
		return err
	}
	self.SetStatus(userCred, api.NAS_STATUS_CREATING, "")
	return nil
}

func (manager SFileSystemManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []api.FileSystemDetails {
	rows := make([]api.FileSystemDetails, len(objs))
	//stdRows := manager.SStatusInfrasResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)
	stdRows := manager.SSharableVirtualResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)
	regionRows := manager.SCloudregionResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)
	mRows := manager.SManagedResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)
	zoneIds := make([]string, len(objs))
	for i := range rows {
		rows[i] = api.FileSystemDetails{
			SharableVirtualResourceDetails: stdRows[i],
			CloudregionResourceInfo:        regionRows[i],
			ManagedResourceInfo:            mRows[i],
		}
		nas := objs[i].(*SFileSystem)
		zoneIds[i] = nas.ZoneId
		//nas.GetCloudaccount().Options.
		account := nas.GetCloudaccount()
		if account == nil {
			log.Errorf("file system %s get cloud account nil", nas.Name)
			rows[i].MountTargetDomainName = ""
		} else {
			if account.Options == nil {
				rows[i].MountTargetDomainName = ""
			} else {
				domain, _ := account.Options.GetString("mount_target_domain_name")
				rows[i].MountTargetDomainName = domain
			}
		}
	}

	zoneMaps, err := db.FetchIdNameMap2(ZoneManager, zoneIds)
	if err != nil {
		return rows
	}
	for i := range rows {
		rows[i].Zone, _ = zoneMaps[zoneIds[i]]
	}
	return rows
}

func (manager *SFileSystemManager) ListItemExportKeys(ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	keys stringutils2.SSortedStrings,
) (*sqlchemy.SQuery, error) {
	var err error
	//q, err = manager.SStatusInfrasResourceBaseManager.ListItemExportKeys(ctx, q, userCred, keys)
	q, err = manager.SSharableVirtualResourceBaseManager.ListItemExportKeys(ctx, q, userCred, keys)
	if err != nil {
		return nil, errors.Wrap(err, "SStatusInfrasResourceBaseManager.ListItemExportKeys")
	}
	q, err = manager.SCloudregionResourceBaseManager.ListItemExportKeys(ctx, q, userCred, keys)
	if err != nil {
		return nil, errors.Wrap(err, "SCloudregionResourceBaseManager.ListItemExportKeys")
	}
	return q, nil
}

func (manager *SFileSystemManager) QueryDistinctExtraField(q *sqlchemy.SQuery, field string) (*sqlchemy.SQuery, error) {
	var err error

	//q, err = manager.SStatusInfrasResourceBaseManager.QueryDistinctExtraField(q, field)
	q, err = manager.SSharableVirtualResourceBaseManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}
	q, err = manager.SCloudregionResourceBaseManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}
	q, err = manager.SManagedResourceBaseManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}

	return q, httperrors.ErrNotFound
}

func (manager *SFileSystemManager) OrderByExtraFields(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.FileSystemListInput,
) (*sqlchemy.SQuery, error) {
	var err error

	//q, err = manager.SStatusInfrasResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.StatusInfrasResourceBaseListInput)
	q, err = manager.SSharableVirtualResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.SharableVirtualResourceListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SStatusInfrasResourceBaseManager.OrderByExtraFields")
	}
	q, err = manager.SManagedResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.ManagedResourceListInput)
	if err != nil {
		return nil, errors.Wrapf(err, "SManagedResourceBaseManager.OrderByExtraFields")
	}
	q, err = manager.SCloudregionResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.RegionalFilterListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SCloudregionResourceBaseManager.OrderByExtraFields")
	}

	return q, nil
}

func (self *SCloudregion) GetFileSystems() ([]SFileSystem, error) {
	ret := []SFileSystem{}
	q := FileSystemManager.Query().Equals("cloudregion_id", self.Id)
	err := db.FetchModelObjects(FileSystemManager, q, &ret)
	if err != nil {
		return nil, errors.Wrapf(err, "db.FetchModelObjects")
	}
	return ret, nil
}

func (self *SCloudregion) GetFileSystemsByManagerId(managerId string) ([]SFileSystem, error) {
	ret := []SFileSystem{}
	q := FileSystemManager.Query().Equals("cloudregion_id", self.Id).Equals("manager_id", managerId)
	err := db.FetchModelObjects(FileSystemManager, q, &ret)
	if err != nil {
		return nil, errors.Wrapf(err, "db.FetchModelObjects")
	}
	return ret, nil
}

func (self *SCloudregion) SyncFileSystems(ctx context.Context, userCred mcclient.TokenCredential, provider *SCloudprovider, filesystems []cloudprovider.ICloudFileSystem) ([]SFileSystem, []cloudprovider.ICloudFileSystem, compare.SyncResult) {
	lockman.LockRawObject(ctx, self.Id, "filesystems")
	defer lockman.ReleaseRawObject(ctx, self.Id, "filesystems")

	result := compare.SyncResult{}

	localFSs := []SFileSystem{}
	remoteFSs := []cloudprovider.ICloudFileSystem{}

	dbFSs, err := self.GetFileSystemsByManagerId(provider.GetId())
	if err != nil {
		result.Error(errors.Wrapf(err, "self.GetFileSystemsByManagerId"))
		return localFSs, remoteFSs, result
	}

	removed := make([]SFileSystem, 0)
	commondb := make([]SFileSystem, 0)
	commonext := make([]cloudprovider.ICloudFileSystem, 0)
	added := make([]cloudprovider.ICloudFileSystem, 0)
	err = compare.CompareSets(dbFSs, filesystems, &removed, &commondb, &commonext, &added)
	if err != nil {
		result.Error(errors.Wrapf(err, "compare.CompareSets"))
		return localFSs, remoteFSs, result
	}

	for i := 0; i < len(removed); i += 1 {
		err = removed[i].syncRemove(ctx, userCred)
		if err != nil {
			result.DeleteError(err)
			continue
		}
		result.Delete()
	}
	for i := 0; i < len(commondb); i += 1 {
		err = commondb[i].SyncWithCloudFileSystem(ctx, userCred, commonext[i], false)
		if err != nil {
			result.UpdateError(err)
			continue
		}
		localFSs = append(localFSs, commondb[i])
		remoteFSs = append(remoteFSs, commonext[i])
		result.Update()
	}
	for i := 0; i < len(added); i += 1 {
		newFs, err := self.newFromCloudFileSystem(ctx, userCred, provider, added[i])
		if err != nil {
			result.AddError(err)
			continue
		}
		syncMetadata(ctx, userCred, newFs, added[i])
		localFSs = append(localFSs, *newFs)
		remoteFSs = append(remoteFSs, added[i])
		result.Add()
	}

	return localFSs, remoteFSs, result
}

func (self *SFileSystem) syncRemove(ctx context.Context, userCred mcclient.TokenCredential) error {
	lockman.LockObject(ctx, self)
	defer lockman.ReleaseObject(ctx, self)

	self.DeletePreventionOff(self, userCred)

	err := self.ValidateDeleteCondition(ctx, nil)
	if err != nil { // cannot delete
		return self.SetStatus(userCred, api.NAS_STATUS_UNKNOWN, "sync to delete")
	}

	err = self.RealDelete(ctx, userCred)
	if err != nil {
		return err
	}
	notifyclient.EventNotify(ctx, userCred, notifyclient.SEventNotifyParam{
		Obj:    self,
		Action: notifyclient.ActionSyncDelete,
	})
	return nil
}

func (self *SFileSystem) CustomizeDelete(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) error {

	return self.StartDeleteTask(ctx, userCred, "")
}

func (self *SFileSystem) StartDeleteTask(ctx context.Context, userCred mcclient.TokenCredential, parentTaskId string) error {
	var err = func() error {
		task, err := taskman.TaskManager.NewTask(ctx, "FileSystemDeleteTask", self, userCred, nil, parentTaskId, "", nil)
		if err != nil {
			return errors.Wrapf(err, "NewTask")
		}
		return task.ScheduleRun(nil)
	}()
	if err != nil {
		self.SetStatus(userCred, api.NAS_STATUS_DELETE_FAILED, err.Error())
		return nil
	}
	self.SetStatus(userCred, api.NAS_STATUS_DELETING, "")
	return nil
}

func (self *SFileSystem) Delete(ctx context.Context, userCred mcclient.TokenCredential) error {
	return nil
}

func (self *SFileSystem) RealDelete(ctx context.Context, userCred mcclient.TokenCredential) error {
	//mts, err := self.GetMountTargets()
	//if err != nil {
	//	return errors.Wrapf(err, "GetMountTargets")
	//}
	//for i := range mts {
	//	err = mts[i].RealDelete(ctx, userCred)
	//	if err != nil {
	//		return errors.Wrapf(err, "mount target %s real delete", mts[i].DomainName)
	//	}
	//}

	mtas, err := self.GetMountTargetAcls()
	if err != nil {
		return errors.Wrapf(err, "GetMountTargetAcls")
	}
	for i := range mtas {
		err = mtas[i].RealDelete(ctx, userCred)
		if err != nil {
			return errors.Wrapf(err, "mount target acl %s real delete", mtas[i].Name)
		}
	}
	//return self.SInfrasResourceBase.Delete(ctx, userCred)
	return self.SSharableVirtualResourceBase.Delete(ctx, userCred)
}

func (self *SFileSystem) ValidateDeleteCondition(ctx context.Context, info jsonutils.JSONObject) error {
	if self.DisableDelete.IsTrue() {
		return httperrors.NewInvalidStatusError("FileSystem is locked, cannot delete")
	}
	//return self.SStatusInfrasResourceBase.ValidateDeleteCondition(ctx, nil)
	return self.SSharableVirtualResourceBase.ValidateDeleteCondition(ctx, nil)
}

func (self *SFileSystem) SyncAllWithCloudFileSystem(ctx context.Context, userCred mcclient.TokenCredential, fs cloudprovider.ICloudFileSystem) error {
	//syncFileSystemMountTargets(ctx, userCred, self, fs)
	if self.GetProviderName() == api.CLOUD_PROVIDER_XGFS {
		syncFileSystemMountTargetAcls(ctx, userCred, self, fs)
	} else {
		syncFileSystemMountTargets(ctx, userCred, self, fs)
	}
	return self.SyncWithCloudFileSystem(ctx, userCred, fs, false)
}

func (self *SFileSystem) SyncWithCloudFileSystem(ctx context.Context, userCred mcclient.TokenCredential, fs cloudprovider.ICloudFileSystem, statsOnly bool) error {
	diff, err := db.UpdateWithLock(ctx, self, func() error {
		self.UsedCapacity = fs.GetUsedCapacityGb()
		self.Files = fs.GetFileCount()

		if !statsOnly {
			self.Status = fs.GetStatus()
			self.StorageType = fs.GetStorageType()
			self.Protocol = fs.GetProtocol()
			self.Capacity = fs.GetCapacityGb()
			self.FilesQuota = fs.GetFileQuota()
			self.FileSystemType = fs.GetFileSystemType()
			self.MountTargetCountLimit = fs.GetMountTargetCountLimit()
			if zoneId := fs.GetZoneId(); len(zoneId) > 0 {
				region, err := self.GetRegion()
				if err != nil {
					return errors.Wrapf(err, "self.GetRegion")
				}
				self.ZoneId, _ = region.getZoneIdBySuffix(zoneId)
			}
		}
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "db.UpdateWithLock")
	}
	if len(diff) > 0 {
		notifyclient.EventNotify(ctx, userCred, notifyclient.SEventNotifyParam{
			Obj:    self,
			Action: notifyclient.ActionSyncUpdate,
		})
	}
	syncMetadata(ctx, userCred, self, fs)
	return nil
}

func (self *SCloudregion) getZoneIdBySuffix(zoneId string) (string, error) {
	zones, err := self.GetZones()
	if err != nil {
		return "", errors.Wrapf(err, "region.GetZones")
	}
	for _, zone := range zones {
		if strings.HasSuffix(zone.ExternalId, zoneId) {
			return zone.Id, nil
		}
	}
	return "", errors.Wrapf(cloudprovider.ErrNotFound, zoneId)
}

func (self *SCloudregion) newFromCloudFileSystem(ctx context.Context, userCred mcclient.TokenCredential, provider *SCloudprovider, fs cloudprovider.ICloudFileSystem) (*SFileSystem, error) {
	nas := SFileSystem{}
	nas.SetModelManager(FileSystemManager, &nas)
	nas.ExternalId = fs.GetGlobalId()
	nas.CloudregionId = self.Id
	nas.ManagerId = provider.Id
	nas.Status = fs.GetStatus()
	nas.CreatedAt = fs.GetCreatedAt()
	nas.StorageType = fs.GetStorageType()
	nas.Protocol = fs.GetProtocol()
	nas.Capacity = fs.GetCapacityGb()
	nas.UsedCapacity = fs.GetUsedCapacityGb()
	nas.Files = fs.GetFileCount()
	nas.FilesQuota = fs.GetFileQuota()
	nas.FileSystemType = fs.GetFileSystemType()
	nas.MountTargetCountLimit = fs.GetMountTargetCountLimit()
	if zoneId := fs.GetZoneId(); len(zoneId) > 0 {
		nas.ZoneId, _ = self.getZoneIdBySuffix(zoneId)
	}
	fileSystem, err := func() (*SFileSystem, error) {
		lockman.LockRawObject(ctx, FileSystemManager.Keyword(), "name")
		defer lockman.ReleaseRawObject(ctx, FileSystemManager.Keyword(), "name")

		var err error
		nas.Name, err = db.GenerateName(ctx, FileSystemManager, userCred, fs.GetName())
		if err != nil {
			return nil, errors.Wrapf(err, "db.GenerateName")
		}

		return &nas, FileSystemManager.TableSpec().Insert(ctx, &nas)
	}()
	if err != nil {
		return nil, err
	}
	notifyclient.EventNotify(ctx, userCred, notifyclient.SEventNotifyParam{
		Obj:    &nas,
		Action: notifyclient.ActionSyncCreate,
	})
	return fileSystem, nil
}

// 同步NAS状态
func (self *SFileSystem) PerformSyncstatus(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, input api.FileSystemSyncstatusInput) (jsonutils.JSONObject, error) {
	var openTask = true
	count, err := taskman.TaskManager.QueryTasksOfObject(self, time.Now().Add(-3*time.Minute), &openTask).CountWithError()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, httperrors.NewBadRequestError("Nas has %d task active, can't sync status", count)
	}

	return nil, self.StartSyncstatus(ctx, userCred, "")
}

func (self *SFileSystem) StartSyncstatus(ctx context.Context, userCred mcclient.TokenCredential, parentTaskId string) error {
	return StartResourceSyncStatusTask(ctx, userCred, self, "FileSystemSyncstatusTask", parentTaskId)
}

func (self *SFileSystem) GetIRegion(ctx context.Context) (cloudprovider.ICloudRegion, error) {
	provider, err := self.GetDriver(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "self.GetDriver")
	}
	if provider.GetFactory().IsOnPremise() {
		return provider.GetOnPremiseIRegion()
	} else {
		region, err := self.GetRegion()
		if err != nil {
			return nil, errors.Wrapf(err, "self.GetRegion")
		}
		return provider.GetIRegionById(region.GetExternalId())
	}
}

func (self *SFileSystem) GetICloudFileSystem(ctx context.Context) (cloudprovider.ICloudFileSystem, error) {
	if len(self.ExternalId) == 0 {
		return nil, errors.Wrapf(cloudprovider.ErrNotFound, "empty externalId")
	}
	iRegion, err := self.GetIRegion(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "self.GetIRegion")
	}
	return iRegion.GetICloudFileSystemById(self.ExternalId)
}

func (manager *SFileSystemManager) getExpiredPostpaids() ([]SFileSystem, error) {
	q := ListExpiredPostpaidResources(manager.Query(), options.Options.ExpiredPrepaidMaxCleanBatchSize)

	fs := make([]SFileSystem, 0)
	err := db.FetchModelObjects(manager, q, &fs)
	if err != nil {
		return nil, errors.Wrapf(err, "db.FetchModelObjects")
	}
	return fs, nil
}

func (self *SFileSystem) doExternalSync(ctx context.Context, userCred mcclient.TokenCredential) error {
	iFs, err := self.GetICloudFileSystem(ctx)
	if err != nil {
		return errors.Wrapf(err, "GetICloudFileSystem")
	}
	return self.SyncWithCloudFileSystem(ctx, userCred, iFs, false)
}

func (manager *SFileSystemManager) DeleteExpiredPostpaids(ctx context.Context, userCred mcclient.TokenCredential, isStart bool) {
	fss, err := manager.getExpiredPostpaids()
	if err != nil {
		log.Errorf("FileSystem getExpiredPostpaids error: %v", err)
		return
	}
	for i := 0; i < len(fss); i += 1 {
		if len(fss[i].ExternalId) > 0 {
			err := fss[i].doExternalSync(ctx, userCred)
			if err == nil && fss[i].IsValidPostPaid() {
				continue
			}
		}
		fss[i].DeletePreventionOff(&fss[i], userCred)
		fss[i].StartDeleteTask(ctx, userCred, "")
	}
}

func (self *SFileSystem) PerformRemoteUpdate(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, input api.FileSystemRemoteUpdateInput) (jsonutils.JSONObject, error) {
	return nil, self.StartRemoteUpdateTask(ctx, userCred, (input.ReplaceTags != nil && *input.ReplaceTags), "")
}

func (self *SFileSystem) StartRemoteUpdateTask(ctx context.Context, userCred mcclient.TokenCredential, replaceTags bool, parentTaskId string) error {
	data := jsonutils.NewDict()
	if replaceTags {
		data.Add(jsonutils.JSONTrue, "replace_tags")
	}
	task, err := taskman.TaskManager.NewTask(ctx, "FileSystemRemoteUpdateTask", self, userCred, data, parentTaskId, "", nil)
	if err != nil {
		return errors.Wrap(err, "NewTask")
	}
	self.SetStatus(userCred, api.NAS_UPDATE_TAGS, "StartRemoteUpdateTask")
	return task.ScheduleRun(nil)
}

func (self *SFileSystem) OnMetadataUpdated(ctx context.Context, userCred mcclient.TokenCredential) {
	if len(self.ExternalId) == 0 {
		return
	}
	self.StartRemoteUpdateTask(ctx, userCred, true, "")
}

func (self *SFileSystem) AllowPerformSync(ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	data jsonutils.JSONObject,
) bool {
	return self.IsOwner(userCred)
}

func (self *SFileSystem) PerformSync(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	data jsonutils.JSONObject,
) (jsonutils.JSONObject, error) {
	if len(self.ExternalId) == 0 {
		return nil, httperrors.NewInvalidStatusError("no external filesystem")
	}

	statsOnly := jsonutils.QueryBoolean(data, "stats_only", false)

	iFs, err := self.GetICloudFileSystem(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "GetICloudFileSystem")
	}

	err = self.SyncWithCloudFileSystem(ctx, userCred, iFs, statsOnly)
	if err != nil {
		return nil, httperrors.NewInternalServerError("SyncWithCloudFileSystem error %s", err)
	}

	return nil, nil
}

//获取FileSystem的文件列表
func (self *SFileSystem) GetDetailsFiles(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	input api.FileSystemGetFilesInput,
) (api.FileSystemGetFilesOutput, error) {
	output := api.FileSystemGetFilesOutput{}
	if len(self.ExternalId) == 0 {
		return output, httperrors.NewInvalidStatusError("no external file system")
	}
	prefix := input.Prefix
	path := self.ExternalId + "/" + input.Path
	marker := input.PagingMarker
	limit := 0
	if input.Limit != nil {
		limit = *input.Limit
	}
	if limit <= 0 {
		limit = 50
	} else if limit > 1000 {
		limit = 1000
	}

	iRegion, err := self.GetIRegion(ctx)
	if err != nil {
		return output, errors.Wrap(err, "self.GetIRegion")
	}
	files, eof, err := iRegion.GetICloudFiles(path, prefix, marker, limit)
	if err != nil {
		return output, httperrors.NewInternalServerError("fail to get files: %s", err)
	}
	for i := range files {
		output.Data = append(output.Data, cloudprovider.ICloudFile2Struct(files[i]))
	}
	output.MarkerField = "name"
	output.MarkerOrder = "ASC"
	if !eof {
		output.NextMarker = files[len(files)-1].GetName()
	}
	return output, nil
}

func (self *SFileSystem) NotifyInitiatorFeishuFileSystemEvent(ctx context.Context, userCred mcclient.TokenCredential) {
	contentTemplate := "**IT运维通知**\n您在私有云平台申请的文件系统资源开通成功\n名称: %s\nNFS挂载地址: %s\n私有云控制台地址: [https://cloud.it.lixiangoa.com/](https://cloud.it.lixiangoa.com/)\n___\n如果您使用中遇到任何问题\n可以通过【IT机器人】进行反馈"
	address := "-"
	account := self.GetCloudaccount()
	if account == nil {
		log.Errorf("file system %s get cloud account nil", self.Name)
	} else {
		if account.Options == nil {
			log.Errorf("file system %s get cloud account options nil", self.Name)
		} else {
			domain, _ := account.Options.GetString("mount_target_domain_name")
			address = domain + ":/dfs/DistributedFileSystem" + self.ExternalId
		}
	}
	content := fmt.Sprintf(contentTemplate, self.Name, address)
	kwargs := jsonutils.NewDict()
	kwargs.Add(jsonutils.NewString(content), "content")
	kwargs.Add(jsonutils.NewInt(2), "platform")

	s := auth.GetAdminSession(ctx, options.Options.Region)
	var user jsonutils.JSONObject
	//get user id
	meta, _ := self.GetAllMetadata(ctx, userCred)
	userId := meta["user_id"]
	if len(userId) > 0 {
		user, _ = identity.UsersV3.Get(s, userId, nil)
	} else {
		user, _ = identity.UsersV3.Get(s, userCred.GetUserId(), nil)
	}
	if user == nil {
		log.Errorln("NotifyInitiatorFeishuFileSystemEvent user is empty.")
		return
	}

	//feishuUserId
	var feishuUserId string
	extra, _ := user.Get(modules.Extra)
	if gotypes.IsNil(extra) {
		log.Errorln("NotifyInitiatorFeishuFileSystemEvent user extra is empty.")
		return
	} else {
		staffId, _ := extra.GetString("staff_id")
		if staffId == "" {
			log.Errorln("NotifyInitiatorFeishuFileSystemEvent user staff_id is empty.")
			return
		} else {
			coaUser, _ := thirdparty.CoaUsers.Get(s, staffId, nil)
			if gotypes.IsNil(coaUser) {
				log.Errorln("NotifyInitiatorFeishuFileSystemEvent coa user is empty.")
				return
			} else {
				feishuUserId, _ = coaUser.GetString("feishu_user_id")
				if feishuUserId == "" {
					log.Errorln("NotifyInitiatorFeishuFileSystemEvent feishuUserId is empty.")
					return
				}
			}
		}
	}

	kwargs.Add(jsonutils.NewString(feishuUserId), "feishu_user_id")
	_, err := thirdparty.CoaUsers.SendMarkdownMessage(s, kwargs)
	if err != nil {
		log.Errorf("NotifyInitiatorFeishuFileSystemEvent send message error %v.", err)
	}
}
