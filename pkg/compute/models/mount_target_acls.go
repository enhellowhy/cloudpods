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
	"strings"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/lockman"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/pkg/util/compare"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/util/regutils"
	"yunion.io/x/pkg/util/stringutils"
	"yunion.io/x/pkg/utils"
	"yunion.io/x/sqlchemy"

	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/validators"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/util/rbacutils"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
)

type SMountTargetAclManager struct {
	//db.SResourceBaseManager
	//db.SStatusResourceBaseManager
	db.SStatusStandaloneResourceBaseManager
	//db.SExternalizedResourceBaseManager
}

var MountTargetAclManager *SMountTargetAclManager

func init() {
	MountTargetAclManager = &SMountTargetAclManager{
		//SResourceBaseManager: db.NewResourceBaseManager(
		SStatusStandaloneResourceBaseManager: db.NewStatusStandaloneResourceBaseManager(
			SMountTargetAcl{},
			"mount_target_acls_tbl",
			"mount_target_acl",
			"mount_target_acls",
		),
	}
	MountTargetAclManager.SetVirtualObject(MountTargetAclManager)
}

type SMountTargetAcl struct {
	//db.SResourceBase
	//db.SStatusResourceBase
	//db.SExternalizedResourceBase
	db.SStatusStandaloneResourceBase

	//Id string `width:"128" charset:"ascii" primary:"true" list:"user"`
	//Priority       int    `default:"1" list:"user" update:"user" list:"user"`
	Source             string `width:"16" charset:"ascii" list:"user" update:"user" create:"required"`
	Sync               bool   `nullable:"false" default:"false" list:"user" update:"user" create:"required"`
	RWAccessType       string `width:"16" charset:"ascii" list:"user" update:"user" create:"required"`
	UserAccessType     string `width:"16" charset:"ascii" list:"user" update:"user" create:"required"`
	RootUserAccessType string `width:"16" charset:"ascii" list:"user" update:"user" create:"required"`
	//MountTargetId string `width:"36" charset:"ascii" nullable:"false" create:"required" index:"true" list:"user"`
	// FileSystemId
	FileSystemId string `width:"36" charset:"ascii" nullable:"false" create:"required" index:"true" list:"user"`
	// 云上Id, 对应云上资源自身Id
	ExternalId string `width:"256" charset:"utf8" index:"true" list:"user" create:"domain_optional" update:"admin" json:"external_id"`
	//Description string `width:"256" charset:"utf8" list:"user" update:"user" create:"optional"`
}

func (self *SMountTargetAcl) BeforeInsert() {
	if len(self.Id) == 0 {
		self.Id = stringutils.UUID4()
	}
}

func (self *SMountTargetAcl) GetId() string {
	return self.Id
}

func (manager *SMountTargetAclManager) CreateByInsertOrUpdate() bool {
	return false
}

func (manager *SMountTargetAclManager) ResourceScope() rbacutils.TRbacScope {
	return rbacutils.ScopeProject
}

func (manager *SMountTargetAclManager) FetchUniqValues(ctx context.Context, data jsonutils.JSONObject) jsonutils.JSONObject {
	groupId, _ := data.GetString("file_system_id")
	return jsonutils.Marshal(map[string]string{"file_system_id": groupId})
}

func (manager *SMountTargetAclManager) FilterByUniqValues(q *sqlchemy.SQuery, values jsonutils.JSONObject) *sqlchemy.SQuery {
	groupId, _ := values.GetString("file_system_id")
	if len(groupId) > 0 {
		q = q.Equals("file_system_id", groupId)
	}
	return q
}

func (manager *SMountTargetAclManager) FetchOwnerId(ctx context.Context, data jsonutils.JSONObject) (mcclient.IIdentityProvider, error) {
	fs, _ := data.GetString("file_system_id")
	if len(fs) > 0 {
		fileSystem, err := db.FetchById(FileSystemManager, fs)
		if err != nil {
			return nil, errors.Wrapf(err, "db.FetchById(%s)", fs)
		}
		return fileSystem.(*SFileSystem).GetOwnerId(), nil
	}
	return db.FetchDomainInfo(ctx, data)
}

func (manager *SMountTargetAclManager) FilterByOwner(q *sqlchemy.SQuery, userCred mcclient.IIdentityProvider, scope rbacutils.TRbacScope) *sqlchemy.SQuery {
	sq := FileSystemManager.Query("id")
	sq = db.SharableManagerFilterByOwner(FileSystemManager, sq, userCred, scope)
	return q.In("file_system_id", sq.SubQuery())
}

func (manager *SMountTargetAclManager) FilterById(q *sqlchemy.SQuery, idStr string) *sqlchemy.SQuery {
	return q.Equals("id", idStr)
}

func (manager *SMountTargetAclManager) ListItemFilter(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.MountTargetAclListInput,
) (*sqlchemy.SQuery, error) {
	var err error
	q, err = manager.SStatusStandaloneResourceBaseManager.ListItemFilter(ctx, q, userCred, query.StatusStandaloneResourceListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SResourceBaseManager.ListItemFilter")
	}
	if len(query.FileSystemId) > 0 {
		_, err := validators.ValidateModel(userCred, FileSystemManager, &query.FileSystemId)
		if err != nil {
			return nil, err
		}
		q = q.Equals("file_system_id", query.FileSystemId)
	} else {
		return nil, httperrors.NewMissingParameterError("file_system_id")
		//return nil, errors.Wrap(err, "SResourceBaseManager.ListItemFilter")
	}
	return q, nil
}

func (manager *SMountTargetAclManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []api.MountTargetAclDetails {
	rows := make([]api.MountTargetAclDetails, len(objs))
	bRows := manager.SStatusStandaloneResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)

	for i := range rows {
		rows[i] = api.MountTargetAclDetails{
			StatusStandaloneResourceDetails: bRows[i],
		}
	}

	return rows
}

func (manager *SMountTargetAclManager) OrderByExtraFields(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.MountTargetAclListInput,
) (*sqlchemy.SQuery, error) {
	var err error

	q, err = manager.SStatusStandaloneResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.StatusStandaloneResourceListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SResourceBaseManager.OrderByExtraFields")
	}

	return q, nil
}

func (manager *SMountTargetAclManager) QueryDistinctExtraField(q *sqlchemy.SQuery, field string) (*sqlchemy.SQuery, error) {
	var err error

	q, err = manager.SStatusStandaloneResourceBaseManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}

	return q, httperrors.ErrNotFound
}

//func (self *SMountTargetAcl) Delete(ctx context.Context, userCred mcclient.TokenCredential) error {
//	return db.DeleteModel(ctx, userCred, self)
//}

// 创建权限组规则
func (manager *SMountTargetAclManager) ValidateCreateData(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, input api.MountTargetAclCreateInput) (api.MountTargetAclCreateInput, error) {
	//if len(input.MountTargetId) == 0 {
	//	return input, httperrors.NewMissingParameterError("mount_target_id")
	//}
	if len(input.FileSystemId) == 0 {
		return input, httperrors.NewMissingParameterError("file_system_id")
	}
	_, err := validators.ValidateModel(userCred, FileSystemManager, &input.FileSystemId)
	if err != nil {
		return input, err
	}
	if len(input.Source) == 0 {
		return input, httperrors.NewMissingParameterError("source")
	}
	if !regutils.MatchCIDR(input.Source) && !regutils.MatchIP4Addr(input.Source) {
		return input, httperrors.NewInputParameterError("invalid source %s", input.Source)
	}
	if strings.HasPrefix(input.Source, "0.0.0.0") {
		return input, httperrors.NewInputParameterError("Can not assign source %s", input.Source)
	}
	{
		q := manager.Query().Equals("source", input.Source).Equals("file_system_id", input.FileSystemId)
		count, err := q.CountWithError()
		if err != nil {
			return input, httperrors.NewInternalServerError("checkout source %s duplicate error: %v", input.Source, err)
		}
		if count > 0 {
			return input, httperrors.NewDuplicateResourceError("Duplicate source %s", input.Source)
		}
	}
	if input.Sync == nil {
		return input, httperrors.NewMissingParameterError("sync")
	}
	if len(input.RWAccessType) == 0 {
		return input, httperrors.NewMissingParameterError("rw_access_type")
	}
	if isIn, _ := utils.InArray(cloudprovider.TRWAccessType(input.RWAccessType), []cloudprovider.TRWAccessType{
		cloudprovider.RWAccessTypeR,
		cloudprovider.RWAccessTypeRW,
	}); !isIn {
		return input, httperrors.NewInputParameterError("invalid rw_access_type %s", input.RWAccessType)
	}
	if len(input.UserAccessType) == 0 {
		return input, httperrors.NewMissingParameterError("user_access_type")
	}
	if len(input.RootUserAccessType) == 0 {
		return input, httperrors.NewMissingParameterError("root_user_access_type")
	}
	if isIn, _ := utils.InArray(cloudprovider.TUserAccessType(input.UserAccessType), []cloudprovider.TUserAccessType{
		cloudprovider.UserAccessTypeAllSquash,
		cloudprovider.UserAccessTypeNoAllSquash,
		//cloudprovider.UserAccessTypeRootSquash,
		//cloudprovider.UserAccessTypeNoRootSquash,
	}); !isIn {
		return input, httperrors.NewInputParameterError("invalid user_access_type %s", input.UserAccessType)
	}
	if isIn, _ := utils.InArray(cloudprovider.TUserAccessType(input.RootUserAccessType), []cloudprovider.TUserAccessType{
		cloudprovider.UserAccessTypeRootSquash,
		cloudprovider.UserAccessTypeNoRootSquash,
	}); !isIn {
		return input, httperrors.NewInputParameterError("invalid root_user_access_type %s", input.UserAccessType)
	}

	if len(input.Name) == 0 {
		//input.Name = fmt.Sprintf("%x", md5.Sum([]byte(input.Source)))
		input.Name = stringutils.UUID4()
	}
	input.StatusStandaloneResourceCreateInput, err = manager.SStatusStandaloneResourceBaseManager.ValidateCreateData(ctx, userCred, ownerId, query, input.StatusStandaloneResourceCreateInput)
	if err != nil {
		return input, err
	}

	return input, nil
}

func (self *SMountTargetAcl) PostCreate(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data jsonutils.JSONObject) {
	self.SStatusStandaloneResourceBase.PostCreate(ctx, userCred, ownerId, query, data)

	log.Debugf("POST Create %s", data)

	self.StartCreateTask(ctx, userCred, "")
}

func (self *SMountTargetAcl) StartCreateTask(ctx context.Context, userCred mcclient.TokenCredential, parentTaskId string) error {
	var err = func() error {
		task, err := taskman.TaskManager.NewTask(ctx, "MountTargetAclCreateTask", self, userCred, nil, parentTaskId, "", nil)
		if err != nil {
			return errors.Wrapf(err, "NewTask")
		}
		return task.ScheduleRun(nil)
	}()
	if err != nil {
		self.SetStatus(userCred, api.MOUNT_TARGET_ACL_STATUS_ADD_FAILED, err.Error())
		return nil
	}
	self.SetStatus(userCred, api.MOUNT_TARGET_ACL_STATUS_ADDING, "")
	return nil
}

func (self *SMountTargetAcl) ValidateUpdateData(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, input api.MountTargetAclUpdateInput) (api.MountTargetAclUpdateInput, error) {
	//if len(input.Source) > 0 && !regutils.MatchCIDR(input.Source) {
	//	return input, httperrors.NewInputParameterError("invalid source %s", input.Source)
	//}
	if input.Sync == nil {
		return input, httperrors.NewMissingParameterError("sync")
	}
	if isIn, _ := utils.InArray(cloudprovider.TRWAccessType(input.RWAccessType), []cloudprovider.TRWAccessType{
		cloudprovider.RWAccessTypeR,
		cloudprovider.RWAccessTypeRW,
	}); !isIn && len(input.RWAccessType) > 0 {
		return input, httperrors.NewInputParameterError("invalid rw_access_type %s", input.RWAccessType)
	}
	if isIn, _ := utils.InArray(cloudprovider.TUserAccessType(input.UserAccessType), []cloudprovider.TUserAccessType{
		cloudprovider.UserAccessTypeAllSquash,
		cloudprovider.UserAccessTypeNoAllSquash,
	}); !isIn && len(input.UserAccessType) > 0 {
		return input, httperrors.NewInputParameterError("invalid user_access_type %s", input.UserAccessType)
	}
	if isIn, _ := utils.InArray(cloudprovider.TUserAccessType(input.RootUserAccessType), []cloudprovider.TUserAccessType{
		cloudprovider.UserAccessTypeRootSquash,
		cloudprovider.UserAccessTypeNoRootSquash,
	}); !isIn && len(input.RootUserAccessType) > 0 {
		return input, httperrors.NewInputParameterError("invalid root_user_access_type %s", input.UserAccessType)
	}
	return input, nil
}

//func (self *SMountTargetAcl) PreUpdate(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) {
//	//self.SStatusStandaloneResourceBase.PreUpdate(ctx, userCred, query, data)
//
//	log.Debugf("Pre Update %s", data)
//
//	self.StartUpdateTask(ctx, userCred, "")
//}
//
func (self *SMountTargetAcl) PostUpdate(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) {
	self.SStatusStandaloneResourceBase.PostUpdate(ctx, userCred, query, data)

	log.Debugf("POST Update %s", data)

	self.StartUpdateTask(ctx, userCred, "")
}

func (self *SMountTargetAcl) StartUpdateTask(ctx context.Context, userCred mcclient.TokenCredential, parentTaskId string) error {
	var err = func() error {
		task, err := taskman.TaskManager.NewTask(ctx, "MountTargetAclUpdateTask", self, userCred, nil, parentTaskId, "", nil)
		if err != nil {
			return errors.Wrapf(err, "NewTask")
		}
		return task.ScheduleRun(nil)
	}()
	if err != nil {
		self.SetStatus(userCred, api.MOUNT_TARGET_ACL_STATUS_UPDATE_FAILED, err.Error())
		return nil
	}
	self.SetStatus(userCred, api.MOUNT_TARGET_ACL_STATUS_UPDATING, "")
	return nil
}

func (self *SMountTargetAcl) GetOwnerId() mcclient.IIdentityProvider {
	fs, err := self.GetFileSystem()
	if err != nil {
		return nil
	}
	return fs.GetOwnerId()
}

func (self SMountTargetAcl) GetExternalId() string {
	return self.ExternalId
}

func (self SMountTargetAcl) GetSource() string {
	return self.Source
}

//func (self *SMountTargetAcl) GetMountTarget() (*SMountTarget, error) {
//	mt, err := MountTargetManager.FetchById(self.MountTargetId)
//	if err != nil {
//		return nil, errors.Wrapf(err, "MountTargetManager.FetchById(%s)", self.MountTargetId)
//	}
//	return mt.(*SMountTarget), nil
//}
//
func (self *SMountTargetAcl) GetFileSystem() (*SFileSystem, error) {
	fs, err := FileSystemManager.FetchById(self.FileSystemId)
	if err != nil {
		return nil, errors.Wrapf(err, "FileSystemManager.FetchById(%s)", self.FileSystemId)
	}
	return fs.(*SFileSystem), nil
}

func (manager *SMountTargetAclManager) ListItemExportKeys(ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	keys stringutils2.SSortedStrings,
) (*sqlchemy.SQuery, error) {
	var err error
	q, err = manager.SStatusStandaloneResourceBaseManager.ListItemExportKeys(ctx, q, userCred, keys)
	if err != nil {
		return nil, errors.Wrap(err, "SResourceBaseManager.ListItemExportKeys")
	}

	return q, nil
}

func (self *SMountTargetAcl) PreDelete(ctx context.Context, userCred mcclient.TokenCredential) {
	self.SStatusStandaloneResourceBase.PreDelete(ctx, userCred)

	//group, err := self.GetAccessGroup()
	//if err == nil {
	//	logclient.AddSimpleActionLog(group, logclient.ACT_DELETE, jsonutils.Marshal(self), userCred, true)
	//	group.DoSync(ctx, userCred)
	//}
}

func (self *SMountTargetAcl) ValidateDeleteCondition(ctx context.Context, info jsonutils.JSONObject) error {
	//if self.AccessGroupId == api.DEFAULT_ACCESS_GROUP {
	//	return httperrors.NewProtectedResourceError("not allow to delete default access group rule")
	//}
	return self.SStatusStandaloneResourceBase.ValidateDeleteCondition(ctx, nil)
}

func (self *SMountTargetAcl) Delete(ctx context.Context, userCred mcclient.TokenCredential) error {
	return nil
}

func (self *SMountTargetAcl) RealDelete(ctx context.Context, userCred mcclient.TokenCredential) error {
	return self.SStatusStandaloneResourceBase.Delete(ctx, userCred)
}

func (self *SMountTargetAcl) CustomizeDelete(ctx context.Context, userCred mcclient.TokenCredential, query api.ServerDeleteInput, input api.NatgatewayDeleteInput) error {
	return self.StartDeleteTask(ctx, userCred, "")
}

func (self *SMountTargetAcl) StartDeleteTask(ctx context.Context, userCred mcclient.TokenCredential, parentTaskId string) error {
	var err = func() error {
		task, err := taskman.TaskManager.NewTask(ctx, "MountTargetAclDeleteTask", self, userCred, nil, parentTaskId, "", nil)
		if err != nil {
			return errors.Wrapf(err, "NewTask")
		}
		return task.ScheduleRun(nil)
	}()
	if err != nil {
		self.SetStatus(userCred, api.MOUNT_TARGET_ACL_STATUS_REMOVE_FAILED, err.Error())
		return err
	}
	return self.SetStatus(userCred, api.MOUNT_TARGET_ACL_STATUS_REMOVING, "")
}

func (self *SFileSystem) GetMountTargetAcls() ([]SMountTargetAcl, error) {
	acls := []SMountTargetAcl{}
	q := MountTargetAclManager.Query().Equals("file_system_id", self.Id)
	err := db.FetchModelObjects(MountTargetAclManager, q, &acls)
	if err != nil {
		return nil, errors.Wrapf(err, "db.FetchModelObjects")
	}
	return acls, nil
}

func (self *SFileSystem) SyncMountTargetAclsBySource(ctx context.Context, userCred mcclient.TokenCredential, extAcls []cloudprovider.ICloudMountTargetAcl) compare.SyncResult {
	lockman.LockRawObject(ctx, self.Id, MountTargetAclManager.KeywordPlural())
	lockman.ReleaseRawObject(ctx, self.Id, MountTargetAclManager.KeywordPlural())

	result := compare.SyncResult{}

	dbAcls, err := self.GetMountTargetAcls()
	if err != nil {
		result.Error(errors.Wrapf(err, "self.GetMountTargetAcls"))
		return result
	}

	removed := make([]SMountTargetAcl, 0)
	commondb := make([]SMountTargetAcl, 0)
	commonext := make([]cloudprovider.ICloudMountTargetAcl, 0)
	added := make([]cloudprovider.ICloudMountTargetAcl, 0)

	set := compare.SCompareSet{
		DBFunc:  "GetSource",
		DBSet:   dbAcls,
		ExtFunc: "GetSource",
		ExtSet:  extAcls,
	}
	err = compare.CompareSetsFunc(set, &removed, &commondb, &commonext, &added)
	if err != nil {
		result.Error(errors.Wrapf(err, "compare.CompareSetsFunc"))
		return result
	}
	//err = compare.CompareSets(dbAcls, extAcls, &removed, &commondb, &commonext, &added)
	//if err != nil {
	//	result.Error(errors.Wrapf(err, "compare.CompareSets"))
	//	return result
	//}

	for i := 0; i < len(removed); i += 1 {
		err = removed[i].RealDelete(ctx, userCred)
		if err != nil {
			result.DeleteError(err)
			continue
		}
		result.Delete()
	}
	for i := 0; i < len(commondb); i += 1 {
		err = commondb[i].SyncWithMountTargetAcl(ctx, userCred, commonext[i])
		if err != nil {
			result.UpdateError(err)
			continue
		}
		result.Update()
	}
	for i := 0; i < len(added); i += 1 {
		err := self.newFromCloudMountTargetAcl(ctx, userCred, added[i])
		if err != nil {
			result.AddError(err)
			continue
		}
		result.Add()
	}

	return result
}

func (self *SFileSystem) SyncMountTargetAcls(ctx context.Context, userCred mcclient.TokenCredential, extAcls []cloudprovider.ICloudMountTargetAcl) compare.SyncResult {
	lockman.LockRawObject(ctx, self.Id, MountTargetAclManager.KeywordPlural())
	lockman.ReleaseRawObject(ctx, self.Id, MountTargetAclManager.KeywordPlural())

	result := compare.SyncResult{}

	dbAcls, err := self.GetMountTargetAcls()
	if err != nil {
		result.Error(errors.Wrapf(err, "self.GetMountTargetAcls"))
		return result
	}

	removed := make([]SMountTargetAcl, 0)
	commondb := make([]SMountTargetAcl, 0)
	commonext := make([]cloudprovider.ICloudMountTargetAcl, 0)
	added := make([]cloudprovider.ICloudMountTargetAcl, 0)
	err = compare.CompareSets(dbAcls, extAcls, &removed, &commondb, &commonext, &added)
	if err != nil {
		result.Error(errors.Wrapf(err, "compare.CompareSets"))
		return result
	}

	for i := 0; i < len(removed); i += 1 {
		err = removed[i].RealDelete(ctx, userCred)
		if err != nil {
			result.DeleteError(err)
			continue
		}
		result.Delete()
	}
	for i := 0; i < len(commondb); i += 1 {
		err = commondb[i].SyncWithMountTargetAcl(ctx, userCred, commonext[i])
		if err != nil {
			result.UpdateError(err)
			continue
		}
		result.Update()
	}
	for i := 0; i < len(added); i += 1 {
		err := self.newFromCloudMountTargetAcl(ctx, userCred, added[i])
		if err != nil {
			result.AddError(err)
			continue
		}
		result.Add()
	}

	return result
}

func (self *SMountTargetAcl) SyncWithMountTargetAcl(ctx context.Context, userCred mcclient.TokenCredential, m cloudprovider.ICloudMountTargetAcl) error {
	_, err := db.Update(self, func() error {
		self.Status = m.GetStatus()
		self.Name = m.GetName()
		self.ExternalId = m.GetGlobalId()
		self.Sync = m.GetSync()
		self.RWAccessType = string(m.GetRWAccessType())
		self.RootUserAccessType = string(m.GetRootUserAccessType())
		self.UserAccessType = string(m.GetUserAccessType())
		self.Source = m.GetSource()

		return nil
	})
	return errors.Wrapf(err, "db.Update")
}

func (self *SFileSystem) newFromCloudMountTargetAcl(ctx context.Context, userCred mcclient.TokenCredential, m cloudprovider.ICloudMountTargetAcl) error {
	acl := &SMountTargetAcl{}
	acl.SetModelManager(MountTargetAclManager, acl)
	acl.FileSystemId = self.Id
	acl.Name = m.GetName()
	acl.Status = m.GetStatus()
	acl.ExternalId = m.GetGlobalId()
	acl.RWAccessType = string(m.GetRWAccessType())
	acl.Sync = m.GetSync()
	acl.RootUserAccessType = string(m.GetRootUserAccessType())
	acl.UserAccessType = string(m.GetUserAccessType())
	acl.Source = m.GetSource()

	return MountTargetAclManager.TableSpec().Insert(ctx, acl)
}
