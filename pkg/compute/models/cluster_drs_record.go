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
	"strconv"
	"time"
	"yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/util/reflectutils"
	"yunion.io/x/pkg/util/stringutils"
	"yunion.io/x/sqlchemy"

	"yunion.io/x/onecloud/pkg/cloudcommon/consts"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
)

type SDrsRecordManager struct {
	db.SModelBaseManager
}

type SDrsRecord struct {
	db.SModelBase

	Id           int64  `primary:"true" auto_increment:"true" list:"user"`
	ClusterId    string `width:"128" charset:"ascii" nullable:"false" list:"user" create:"required" index:"true"`
	ActivityId   string `width:"128" charset:"ascii" nullable:"false" list:"user" create:"required" index:"true"`
	GuestId      string `width:"128" charset:"ascii" nullable:"false" list:"user" create:"required"`
	GuestName    string `width:"128" charset:"utf8" nullable:"false" list:"user" create:"required"`
	FromHostId   string `width:"128" charset:"ascii" nullable:"false" list:"user" create:"required"`
	FromHostName string `width:"128" charset:"utf8" nullable:"false" list:"user" create:"required"`
	ToHostId     string `width:"128" charset:"ascii" nullable:"false" list:"user" create:"required"`
	ToHostName   string `width:"128" charset:"utf8" nullable:"false" list:"user" create:"required"`
	Action       string `width:"32" charset:"utf8" nullable:"false" list:"user" create:"required"`
	Notes        string `charset:"utf8" list:"user" create:"optional"`

	UserId  string    `width:"128" charset:"ascii" list:"user" create:"optional"`
	User    string    `width:"128" charset:"utf8" list:"user" create:"optional"`
	OpsTime time.Time `nullable:"false" list:"user"`
	// 命中策略
	Strategy  string    `width:"16" charset:"ascii" nullable:"false" default:"" list:"user" update:"admin" create:"required"`
	StartTime time.Time `nullable:"true" list:"user" create:"optional"`
	Success   bool      `list:"user" create:"required"`
}

var DrsRecordManager *SDrsRecordManager

var _ db.IModelManager = (*SDrsRecordManager)(nil)
var _ db.IModel = (*SDrsRecord)(nil)

func init() {
	DrsRecordManager = &SDrsRecordManager{
		SModelBaseManager: db.NewModelBaseManager(
			SDrsRecord{},
			"cluster_drs_record_tbl",
			"drsrecord",
			"drsrecords",
		),
	}
	DrsRecordManager.SetVirtualObject(DrsRecordManager)
}

func (r *SDrsRecord) GetId() string {
	return fmt.Sprintf("%d", r.Id)
}

func (r *SDrsRecord) GetName() string {
	return fmt.Sprintf("%d-%s", r.Id, r.Action)
}

func (r *SDrsRecord) GetUpdatedAt() time.Time {
	return r.OpsTime
}

func (r *SDrsRecord) GetUpdateVersion() int {
	return 1
}

func (r *SDrsRecord) GetModelManager() db.IModelManager {
	return DrsRecordManager
}

func (manager *SDrsRecordManager) LogEvent(model db.IModel, action string, notes interface{}, userCred mcclient.TokenCredential) {
	if !consts.OpsLogEnabled() {
		return
	}

	var (
		objId   = model.GetId()
		objName = model.GetName()
	)

	if action == db.ACT_UPDATE {
		// skip empty diff
		if notes == nil {
			return
		}
		if uds, ok := notes.(sqlchemy.UpdateDiffs); ok && len(uds) == 0 {
			return
		}
	}
	r := &SDrsRecord{
		OpsTime: time.Now().UTC(),
		//ObjId:   objId,
		//ObjName: objName,
		Action: action,
		Notes:  stringutils.Interface2String(notes),
	}
	if userCred == nil {
		log.Warningf("Log event with empty userCred: objType=%s objId=%s objName=%s action=%s", model.Keyword(), objId, objName, action)
		const unknown = "unknown"
		r.UserId = unknown
		r.User = unknown
	} else {
		r.UserId = userCred.GetUserId()
		r.User = userCred.GetUserName()
	}
	r.SetModelManager(DrsRecordManager, r)

	if virtualModel, ok := model.(db.IVirtualModel); ok {
		ownerId := virtualModel.GetOwnerId()
		if ownerId != nil {
			//r.OwnerProjectId = ownerId.GetProjectId()
			//r.OwnerDomainId = ownerId.GetProjectDomainId()
		}
	}

	err := manager.TableSpec().Insert(context.Background(), r)
	if err != nil {
		log.Errorf("fail to insert drs record: %s", err)
	}
}

// 操作日志列表
func (manager *SDrsRecordManager) ListItemFilter(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	input compute.DrsRecordListInput,
) (*sqlchemy.SQuery, error) {
	if len(input.ClusterIds) > 0 {
		if len(input.ClusterIds) == 1 {
			q = q.Filter(sqlchemy.Equals(q.Field("cluster_id"), input.ClusterIds[0]))
		} else {
			q = q.Filter(sqlchemy.In(q.Field("cluster_id"), input.ClusterIds))
		}
	}
	if len(input.ActivityIds) > 0 {
		if len(input.ActivityIds) == 1 {
			q = q.Filter(sqlchemy.Equals(q.Field("activity_id"), input.ActivityIds[0]))
		} else {
			q = q.Filter(sqlchemy.In(q.Field("activity_id"), input.ActivityIds))
		}
	}
	if len(input.GuestIds) > 0 {
		if len(input.GuestIds) == 1 {
			q = q.Filter(sqlchemy.Equals(q.Field("guest_id"), input.GuestIds[0]))
		} else {
			q = q.Filter(sqlchemy.In(q.Field("guest_id"), input.GuestIds))
		}
	}
	if len(input.GuestNames) > 0 {
		if len(input.GuestNames) == 1 {
			q = q.Filter(sqlchemy.Equals(q.Field("guest_name"), input.GuestNames[0]))
		} else {
			q = q.Filter(sqlchemy.In(q.Field("guest_name"), input.GuestNames))
		}
	}
	if len(input.Strategy) > 0 {
		q = q.Equals("strategy", input.Strategy)
	}
	if input.Success != nil {
		q = q.Equals("success", *input.Success)
	}
	if !input.Since.IsZero() {
		q = q.GT("ops_time", input.Since)
	}
	if !input.Until.IsZero() {
		q = q.LE("ops_time", input.Until)
	}
	return q, nil
}

func (manager *SDrsRecordManager) AllowListItems(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject) bool {
	return true
}

func (manager *SDrsRecordManager) AllowCreateItem(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) bool {
	return false
}

func (self *SDrsRecord) AllowGetDetails(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject) bool {
	return true
}

func (self *SDrsRecord) AllowUpdateItem(ctx context.Context, userCred mcclient.TokenCredential) bool {
	return false
}

func (self *SDrsRecord) AllowDeleteItem(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) bool {
	return false
}

func (self *SDrsRecord) ValidateDeleteCondition(ctx context.Context, info jsonutils.JSONObject) error {
	return httperrors.NewForbiddenError("not allow to delete drs record")
}

func (self *SDrsRecordManager) FilterById(q *sqlchemy.SQuery, idStr string) *sqlchemy.SQuery {
	id, _ := strconv.Atoi(idStr)
	return q.Equals("id", id)
}

func (self *SDrsRecordManager) FilterByNotId(q *sqlchemy.SQuery, idStr string) *sqlchemy.SQuery {
	id, _ := strconv.Atoi(idStr)
	return q.NotEquals("id", id)
}

func (self *SDrsRecordManager) FilterByName(q *sqlchemy.SQuery, name string) *sqlchemy.SQuery {
	return q
}

//func (self *SDrsRecordManager) FilterByOwner(q *sqlchemy.SQuery, ownerId mcclient.IIdentityProvider, scope rbacutils.TRbacScope) *sqlchemy.SQuery {
//	if ownerId != nil {
//		switch scope {
//		case rbacutils.ScopeUser:
//			if len(ownerId.GetUserId()) > 0 {
//				/*
//				 * 默认只能查看本人发起的操作
//				 */
//				q = q.Filter(sqlchemy.Equals(q.Field("user_id"), ownerId.GetUserId()))
//			}
//		case rbacutils.ScopeProject:
//			if len(ownerId.GetProjectId()) > 0 {
//				/*
//				 * 项目视图可以查看本项目人员发起的操作，或者对本项目资源实施的操作, QIU Jian
//				 */
//				q = q.Filter(sqlchemy.OR(
//					sqlchemy.Equals(q.Field("tenant_id"), ownerId.GetProjectId()),
//					sqlchemy.Equals(q.Field("owner_tenant_id"), ownerId.GetProjectId()),
//				))
//			}
//		case rbacutils.ScopeDomain:
//			if len(ownerId.GetProjectDomainId()) > 0 {
//				/*
//				 * 域视图可以查看本域人员发起的操作，或者对本域资源实施的操作, QIU Jian
//				 */
//				q = q.Filter(sqlchemy.OR(
//					sqlchemy.Equals(q.Field("project_domain_id"), ownerId.GetProjectDomainId()),
//					sqlchemy.Equals(q.Field("owner_domain_id"), ownerId.GetProjectDomainId()),
//				))
//			}
//		default:
//			// systemScope, no filter
//		}
//	}
//	return q
//}

func (manager *SDrsRecordManager) GetPagingConfig() *db.SPagingConfig {
	return &db.SPagingConfig{
		Order:        sqlchemy.SQL_ORDER_DESC,
		MarkerFields: []string{"id"},
		DefaultLimit: 20,
	}
}

func (manager *SDrsRecordManager) ValidateCreateData(ctx context.Context,
	userCred mcclient.TokenCredential,
	ownerId mcclient.IIdentityProvider,
	query jsonutils.JSONObject,
	data compute.DrsRecordCreateInput,
) (compute.DrsRecordCreateInput, error) {
	data.User = ownerId.GetUserName()
	return data, nil
}

func (r *SDrsRecord) CustomizeCreate(ctx context.Context,
	userCred mcclient.TokenCredential,
	ownerId mcclient.IIdentityProvider,
	query jsonutils.JSONObject,
	data jsonutils.JSONObject) error {
	r.User = ownerId.GetUserName()
	r.UserId = ownerId.GetUserId()
	r.OpsTime = time.Now().UTC()
	return r.SModelBase.CustomizeCreate(ctx, userCred, ownerId, query, data)
}

func (manager *SDrsRecordManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []compute.DrsRecordDetails {
	rows := make([]compute.DrsRecordDetails, len(objs))

	for i := range rows {
		var base *SDrsRecord
		err := reflectutils.FindAnonymouStructPointer(objs[i], &base)
		if err != nil {
			log.Errorf("Cannot find OpsLog in %#v: %s", objs[i], err)
		}
	}

	return rows
}
