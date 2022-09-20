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
	"database/sql"
	"fmt"
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/tristate"
	"yunion.io/x/pkg/utils"
	"yunion.io/x/sqlchemy"

	"yunion.io/x/onecloud/pkg/apis"
	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/util/rbacutils"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
)

const (
	STRATEGY_USAGE    = "usage"
	STRATEGY_ASSIGNED = "assigned"
)

var CLUSTER_STRATEGY_LIST = []string{STRATEGY_USAGE, STRATEGY_ASSIGNED}

type IClusterJointManager interface {
	db.IJointModelManager
	GetResourceIdKey(db.IJointModelManager) string
}

type IClusterJointModel interface {
	db.IJointModel
	GetClusterId() string
	GetResourceId() string
	GetDetails(base api.ClusterJointResourceDetails, resourceName string, isList bool) interface{}
}

type IModelWithCluster interface {
	db.IModel
	GetClusterJointManager() IClusterJointManager
	//ClearClusterDescCache() error
}

type SClusterManager struct {
	db.SStandaloneResourceBaseManager
	db.SScopedResourceBaseManager

	jointsManager map[string]IClusterJointManager
}

var ClusterManager *SClusterManager

func init() {
	ClusterManager = &SClusterManager{
		SStandaloneResourceBaseManager: db.NewStandaloneResourceBaseManager(
			SCluster{},
			"clusters_tbl",
			"cluster",
			"clusters",
		),
		jointsManager: make(map[string]IClusterJointManager),
	}
	ClusterManager.SetVirtualObject(ClusterManager)
}

func (manager *SClusterManager) InitializeData() error {
	// set old clusters resource_type to hosts
	clusters := []SCluster{}
	q := manager.Query().IsNullOrEmpty("resource_type")
	err := db.FetchModelObjects(manager, q, &clusters)
	if err != nil {
		return err
	}
	for _, cluster := range clusters {
		tmp := &cluster
		db.Update(tmp, func() error {
			tmp.ResourceType = HostManager.KeywordPlural()
			return nil
		})
	}
	manager.BindJointManagers(map[db.IModelManager]IClusterJointManager{
		HostManager: HostclusterManager,
	})
	return nil
}

func (manager *SClusterManager) NamespaceScope() rbacutils.TRbacScope {
	return rbacutils.ScopeSystem
}

func (manager *SClusterManager) BindJointManagers(ms map[db.IModelManager]IClusterJointManager) {
	for m, schedtagM := range ms {
		manager.jointsManager[m.KeywordPlural()] = schedtagM
	}
}

func (manager *SClusterManager) GetResourceTypes() []string {
	ret := []string{}
	for key := range manager.jointsManager {
		ret = append(ret, key)
	}
	return ret
}

func (manager *SClusterManager) GetJointManager(resTypePlural string) IClusterJointManager {
	return manager.jointsManager[resTypePlural]
}

type SCluster struct {
	db.SStandaloneResourceBase
	db.SScopedResourceBase

	ResourceType string `width:"16" charset:"ascii" nullable:"true" list:"user" create:"required"` // Column(VARCHAR(16, charset='ascii'), nullable=True, default='')
	// 是否开启DRS
	DrsEnabled tristate.TriState `nullable:"false" default:"false" create:"optional" list:"user" update:"user"`
	// DRS平衡策略
	Strategy string `width:"16" charset:"ascii" nullable:"true" default:"usage" list:"user" update:"admin" create:"admin_optional"` // Column(VARCHAR(16, charset='ascii'), nullable=True, default='')
}

func (manager *SClusterManager) AllowListItems(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject) bool {
	return true
}

func (manager *SClusterManager) FilterByOwner(q *sqlchemy.SQuery, userCred mcclient.IIdentityProvider, scope rbacutils.TRbacScope) *sqlchemy.SQuery {
	if userCred == nil {
		return q
	}
	switch scope {
	case rbacutils.ScopeDomain:
		q = q.Filter(sqlchemy.OR(
			// share to system
			sqlchemy.AND(
				sqlchemy.IsNullOrEmpty(q.Field("domain_id")),
				//sqlchemy.IsNullOrEmpty(q.Field("tenant_id")),
			),
			// share to this domain or its sub-projects
			sqlchemy.Equals(q.Field("domain_id"), userCred.GetProjectDomainId()),
		))
	case rbacutils.ScopeProject:
		q = q.Filter(sqlchemy.OR(
			// share to system
			sqlchemy.AND(
				sqlchemy.IsNullOrEmpty(q.Field("domain_id")),
				sqlchemy.IsNullOrEmpty(q.Field("tenant_id")),
			),
			// share to project's parent domain
			sqlchemy.AND(
				sqlchemy.Equals(q.Field("domain_id"), userCred.GetProjectDomainId()),
				sqlchemy.IsNullOrEmpty(q.Field("tenant_id")),
			),
			// share to this project
			sqlchemy.AND(
				sqlchemy.Equals(q.Field("domain_id"), userCred.GetProjectDomainId()),
				sqlchemy.Equals(q.Field("tenant_id"), userCred.GetProjectId()),
			),
		))
	}
	return q
}

func (manager *SClusterManager) ListItemExportKeys(ctx context.Context, q *sqlchemy.SQuery, userCred mcclient.TokenCredential, keys stringutils2.SSortedStrings) (*sqlchemy.SQuery, error) {
	return db.ApplyListItemExportKeys(ctx, q, userCred, keys,
		&manager.SStandaloneResourceBaseManager,
		&manager.SScopedResourceBaseManager,
	)
}

func (manager *SClusterManager) ListItemFilter(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.ClusterListInput,
) (*sqlchemy.SQuery, error) {
	var err error
	q, err = manager.SStandaloneResourceBaseManager.ListItemFilter(ctx, q, userCred, query.StandaloneResourceListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SStandaloneResourceBaseManager.ListItemFilter")
	}
	q, err = manager.SScopedResourceBaseManager.ListItemFilter(ctx, q, userCred, query.ScopedResourceBaseListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SScopedResourceBaseManager.ListItemFilter")
	}

	if len(query.ResourceType) > 0 {
		q = q.In("resource_type", query.ResourceType)
	}

	if len(query.Strategy) > 0 {
		q = q.In("strategy", query.Strategy)
	}

	//if len(query.CloudproviderId) > 0 {
	//	hostSubq := HostManager.Query("id").Equals("manager_id", query.CloudproviderId).SubQuery()
	//	hostSchedtagQ := HostschedtagManager.Query("cluster_id")
	//	hostSchedtagSubq := hostSchedtagQ.Join(hostSubq, sqlchemy.Equals(hostSchedtagQ.Field("host_id"), hostSubq.Field("id"))).SubQuery()
	//	q = q.Join(hostSchedtagSubq, sqlchemy.Equals(q.Field("id"), hostSchedtagSubq.Field("cluster_id")))
	//}

	return q, nil
}

func (manager *SClusterManager) OrderByExtraFields(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.ClusterListInput,
) (*sqlchemy.SQuery, error) {
	var err error

	q, err = manager.SStandaloneResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.StandaloneResourceListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SStandaloneResourceBaseManager.OrderByExtraFields")
	}
	q, err = manager.SScopedResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.ScopedResourceBaseListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SScopedResourceBaseManager.OrderByExtraFields")
	}

	return q, nil
}

func (manager *SClusterManager) QueryDistinctExtraField(q *sqlchemy.SQuery, field string) (*sqlchemy.SQuery, error) {
	var err error

	q, err = manager.SStandaloneResourceBaseManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}
	q, err = manager.SScopedResourceBaseManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}

	return q, httperrors.ErrNotFound
}

func (self *SCluster) AllowGetDetails(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject) bool {
	return true
}

func validateClusterStrategy(strategy string) error {
	if !utils.IsInStringArray(strategy, CLUSTER_STRATEGY_LIST) {
		return httperrors.NewInputParameterError("Invalid stragegy %s", strategy)
	}
	return nil
}

func (manager *SClusterManager) ValidateCreateData(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, input api.ClusterCreateInput) (*jsonutils.JSONDict, error) {
	if len(input.Strategy) > 0 {
		err := validateClusterStrategy(input.Strategy)
		if err != nil {
			return nil, err
		}
	}
	// set resourceType to hosts if not provided by client
	if input.ResourceType == "" {
		input.ResourceType = HostManager.KeywordPlural()
	}
	if !utils.IsInStringArray(input.ResourceType, manager.GetResourceTypes()) {
		return nil, httperrors.NewInputParameterError("Not support resource_type %s", input.ResourceType)
	}

	var err error
	input.ScopedResourceCreateInput, err = manager.SScopedResourceBaseManager.ValidateCreateData(manager, ctx, userCred, ownerId, query, input.ScopedResourceCreateInput)
	if err != nil {
		return nil, err
	}

	input.StandaloneResourceCreateInput, err = manager.SStandaloneResourceBaseManager.ValidateCreateData(ctx, userCred, ownerId, query, input.StandaloneResourceCreateInput)
	if err != nil {
		return nil, err
	}

	return input.JSON(input), nil
}

func (manager *SClusterManager) GetResourceClusters(resType string) ([]SCluster, error) {
	jointMan := manager.jointsManager[resType]
	if jointMan == nil {
		return nil, fmt.Errorf("Not found joint manager by resource type: %s", resType)
	}
	clusters := make([]SCluster, 0)
	if err := manager.Query().Equals("resource_type", resType).All(&clusters); err != nil {
		return nil, err
	}
	return clusters, nil
}

func (self *SCluster) CustomizeCreate(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data jsonutils.JSONObject) error {
	return self.SScopedResourceBase.CustomizeCreate(ctx, userCred, ownerId, query, data)
}

func (self *SCluster) ValidateUpdateData(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data *jsonutils.JSONDict) (*jsonutils.JSONDict, error) {
	strategy, _ := data.GetString("strategy")
	if len(strategy) > 0 {
		err := validateClusterStrategy(strategy)
		if err != nil {
			return nil, err
		}
	}
	input := apis.StandaloneResourceBaseUpdateInput{}
	err := data.Unmarshal(&input)
	if err != nil {
		return nil, errors.Wrap(err, "Unmarshal")
	}
	input, err = self.SStandaloneResourceBase.ValidateUpdateData(ctx, userCred, query, input)
	if err != nil {
		return nil, errors.Wrap(err, "SStandaloneResourceBase.ValidateUpdateData")
	}
	data.Update(jsonutils.Marshal(input))

	return data, nil
}

func (self *SCluster) ValidateDeleteCondition(ctx context.Context, info jsonutils.JSONObject) error {
	cnt, err := self.GetObjectCount()
	if err != nil {
		return httperrors.NewInternalServerError("GetObjectCount fail %s", err)
	}
	if cnt > 0 {
		return httperrors.NewNotEmptyError("Cluster is associated with %s", self.ResourceType)
	}

	return self.SStandaloneResourceBase.ValidateDeleteCondition(ctx, nil)
}

func (self *SCluster) GetResources() ([]IModelWithCluster, error) {
	objs := make([]interface{}, 0)
	q := self.GetObjectQuery()
	masterMan, err := self.GetResourceManager()
	if err != nil {
		return nil, err
	}
	if err := db.FetchModelObjects(masterMan, q, &objs); err != nil {
		return nil, err
	}

	ret := make([]IModelWithCluster, len(objs))
	for i := range objs {
		obj := objs[i]
		ret[i] = GetObjectPtr(obj).(IModelWithCluster)
	}
	return ret, nil
}

func (self *SCluster) GetObjectQuery() *sqlchemy.SQuery {
	jointMan := self.GetJointManager()
	masterMan := jointMan.GetMasterManager()
	objs := masterMan.Query().SubQuery()
	objclusters := jointMan.Query().SubQuery()
	q := objs.Query()
	q = q.Join(objclusters, sqlchemy.AND(sqlchemy.Equals(objclusters.Field(jointMan.GetResourceIdKey(jointMan)), objs.Field("id")),
		sqlchemy.IsFalse(objclusters.Field("deleted"))))
	// q = q.Filter(sqlchemy.IsTrue(objs.Field("enabled")))
	q = q.Filter(sqlchemy.Equals(objclusters.Field("cluster_id"), self.Id))
	return q
}

func (self *SCluster) GetJointManager() IClusterJointManager {
	return ClusterManager.GetJointManager(self.ResourceType)
}

func (s *SCluster) GetResourceManager() (db.IStandaloneModelManager, error) {
	jResMan := s.GetJointManager()
	if jResMan == nil {
		return nil, errors.Errorf("Not found bind joint resource manager by type %q", s.ResourceType)
	}

	return jResMan.GetMasterManager(), nil
}

func (self *SCluster) GetObjectCount() (int, error) {
	q := self.GetObjectQuery()
	return q.CountWithError()
}

func (self *SCluster) getMoreColumns(out api.ClusterDetails) api.ClusterDetails {
	out.ProjectId = self.SScopedResourceBase.ProjectId
	cnt, _ := self.GetObjectCount()
	keyword := self.GetJointManager().GetMasterManager().Keyword()
	switch keyword {
	case HostManager.Keyword():
		out.HostCount = cnt
	case GuestManager.Keyword():
		out.ServerCount = cnt
	default:
		out.OtherCount = cnt
		out.JoinModelKeyword = keyword
	}

	// resource_count = row.host_count || row.other_count || '0'
	if out.HostCount > 0 {
		out.ResourceCount = out.HostCount
	} else if out.OtherCount > 0 {
		out.ResourceCount = out.OtherCount
	} else {
		out.ResourceCount = 0
	}
	return out
}

func (manager *SClusterManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []api.ClusterDetails {
	rows := make([]api.ClusterDetails, len(objs))

	stdRows := manager.SStandaloneResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)
	scopedRows := manager.SScopedResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)
	for i := range rows {
		rows[i] = api.ClusterDetails{
			StandaloneResourceDetails: stdRows[i],
			ScopedResourceBaseInfo:    scopedRows[i],
		}
		rows[i] = objs[i].(*SCluster).getMoreColumns(rows[i])
	}

	return rows
}

func (self *SCluster) GetShortDesc(ctx context.Context) *jsonutils.JSONDict {
	desc := self.SStandaloneResourceBase.GetShortDesc(ctx)
	desc.Add(jsonutils.NewString(self.Strategy), "strategy")
	return desc
}

func (self *SCluster) GetShortDescV2(ctx context.Context) api.ClusterShortDescDetails {
	desc := api.ClusterShortDescDetails{}
	desc.StandaloneResourceShortDescDetail = self.SStandaloneResourceBase.GetShortDescV2(ctx)
	return desc
}

func GetResourceJointClusters(obj IModelWithCluster) ([]IClusterJointModel, error) {
	jointMan := obj.GetClusterJointManager()
	q := jointMan.Query().Equals(jointMan.GetResourceIdKey(jointMan), obj.GetId())
	jointClusters := make([]IClusterJointModel, 0)
	rows, err := q.Rows()
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		item, err := db.NewModelObject(jointMan)
		if err != nil {
			return nil, err
		}
		err = q.Row2Struct(rows, item)
		if err != nil {
			return nil, err
		}
		jointClusters = append(jointClusters, item.(IClusterJointModel))
	}

	return jointClusters, nil
}

func GetClusters(jointMan IClusterJointManager, masterId string) []SCluster {
	tags := make([]SCluster, 0)
	clusters := ClusterManager.Query().SubQuery()
	objclusters := jointMan.Query().SubQuery()
	q := clusters.Query()
	q = q.Join(objclusters, sqlchemy.AND(sqlchemy.Equals(objclusters.Field("cluster_id"), clusters.Field("id")),
		sqlchemy.IsFalse(objclusters.Field("deleted"))))
	q = q.Filter(sqlchemy.Equals(objclusters.Field(jointMan.GetResourceIdKey(jointMan)), masterId))
	err := db.FetchModelObjects(ClusterManager, q, &tags)
	if err != nil {
		log.Errorf("GetClusters error: %s", err)
		return nil
	}
	return tags
}

func PerformSetResourceCluster(obj IModelWithCluster, ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	clusters := jsonutils.GetArrayOfPrefix(data, "cluster")
	setClustersId := []string{}
	for idx := 0; idx < len(clusters); idx++ {
		clusterIdent, _ := clusters[idx].GetString()
		tag, err := ClusterManager.FetchByIdOrName(userCred, clusterIdent)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, httperrors.NewNotFoundError("Cluster %s not found", clusterIdent)
			}
			return nil, httperrors.NewGeneralError(err)
		}
		cluster := tag.(*SCluster)
		if cluster.ResourceType != obj.KeywordPlural() {
			return nil, httperrors.NewInputParameterError("Cluster %s ResourceType is %s, not match %s", cluster.GetName(), cluster.ResourceType, obj.KeywordPlural())
		}
		setClustersId = append(setClustersId, cluster.GetId())
	}
	oldClusters, err := GetResourceJointClusters(obj)
	if err != nil {
		return nil, httperrors.NewGeneralError(fmt.Errorf("Get old joint clusters: %v", err))
	}
	for _, oldCluster := range oldClusters {
		if !utils.IsInStringArray(oldCluster.GetClusterId(), setClustersId) {
			if err := oldCluster.Detach(ctx, userCred); err != nil {
				return nil, httperrors.NewGeneralError(err)
			}
		}
	}
	var oldClusterIds []string
	for _, cluster := range oldClusters {
		oldClusterIds = append(oldClusterIds, cluster.GetClusterId())
	}
	jointMan := obj.GetClusterJointManager()
	for _, setClusterId := range setClustersId {
		if !utils.IsInStringArray(setClusterId, oldClusterIds) {
			if _, err := InsertJointResourceCluster(ctx, jointMan, obj.GetId(), setClusterId); err != nil {
				return nil, errors.Wrapf(err, "InsertJointResourceCluster %s %s", obj.GetId(), setClusterId)
			}
		}
	}
	//if err := obj.ClearClusterDescCache(); err != nil {
	//	log.Errorf("Resource %s/%s ClearClusterDescCache error: %v", obj.Keyword(), obj.GetId(), err)
	//}
	return nil, nil
}

func DeleteResourceJointClusters(obj IModelWithCluster, ctx context.Context, userCred mcclient.TokenCredential) error {
	jointClusters, err := GetResourceJointClusters(obj)
	if err != nil {
		return fmt.Errorf("Get %s clusters error: %v", obj.Keyword(), err)
	}
	for _, cluster := range jointClusters {
		cluster.Delete(ctx, userCred)
	}
	return nil
}

func GetClustersDetailsToResource(obj IModelWithCluster, ctx context.Context, extra *jsonutils.JSONDict) *jsonutils.JSONDict {
	clusters := GetClusters(obj.GetClusterJointManager(), obj.GetId())
	if clusters != nil && len(clusters) > 0 {
		info := make([]jsonutils.JSONObject, len(clusters))
		for i := 0; i < len(clusters); i += 1 {
			info[i] = clusters[i].GetShortDesc(ctx)
		}
		extra.Add(jsonutils.NewArray(info...), "clusters")
	}
	return extra
}

func GetClustersDetailsToResourceV2(obj IModelWithCluster, ctx context.Context) []api.ClusterShortDescDetails {
	info := []api.ClusterShortDescDetails{}
	clusters := GetClusters(obj.GetClusterJointManager(), obj.GetId())
	if clusters != nil && len(clusters) > 0 {
		for i := 0; i < len(clusters); i += 1 {
			desc := clusters[i].GetShortDescV2(ctx)
			info = append(info, desc)
		}
	}
	return info
}

func (s *SCluster) AllowPerformSetScope(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) bool {
	return true
}

func (s *SCluster) PerformSetScope(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	return db.PerformSetScope(ctx, s, userCred, data)
}

func (s *SCluster) PerformDrsEnabled(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	data jsonutils.JSONObject,
) (jsonutils.JSONObject, error) {
	drsEnabled := jsonutils.QueryBoolean(data, "drs_enabled", false)
	if s.DrsEnabled.IsFalse() && drsEnabled {
		diff, err := db.Update(s, func() error {
			s.DrsEnabled = tristate.True
			return nil
		})
		if err != nil {
			return nil, httperrors.NewGeneralError(err)
		}
		db.OpsLog.LogEvent(s, db.ACT_UPDATE, diff, userCred)
	} else if s.DrsEnabled.IsTrue() && !drsEnabled {
		diff, err := db.Update(s, func() error {
			s.DrsEnabled = tristate.False
			return nil
		})
		if err != nil {
			return nil, httperrors.NewGeneralError(err)
		}
		db.OpsLog.LogEvent(s, db.ACT_UPDATE, diff, userCred)
	}
	return nil, nil
}

func (s *SCluster) GetJointResourceCluster(resId string) (IClusterJointModel, error) {
	jMan := s.GetJointManager()
	jObj, err := db.FetchJointByIds(jMan, resId, s.GetId(), nil)
	if err != nil {
		return nil, err
	}
	return jObj.(IClusterJointModel), nil
}

func (s *SCluster) PerformSetResource(ctx context.Context, userCred mcclient.TokenCredential, _ jsonutils.JSONObject, input *api.ClusterSetResourceInput) (jsonutils.JSONObject, error) {
	if input == nil {
		return nil, nil
	}

	setResIds := make(map[string]IModelWithCluster, 0)
	resMan, err := s.GetResourceManager()
	if err != nil {
		return nil, errors.Wrap(err, "get resource manager")
	}

	// get need set resource ids
	for i := 0; i < len(input.ResourceIds); i++ {
		resId := input.ResourceIds[i]
		res, err := resMan.FetchByIdOrName(userCred, resId)
		if err != nil {
			if errors.Cause(err) == sql.ErrNoRows {
				return nil, httperrors.NewNotFoundError("Resource %s %s not found", s.ResourceType, resId)
			}
			return nil, errors.Wrapf(err, "Fetch resource %s by id or name", s.ResourceType)
		}
		setResIds[res.GetId()] = res.(IModelWithCluster)
	}

	// unbind current resources if not in input
	curRess, err := s.GetResources()
	if err != nil {
		return nil, errors.Wrap(err, "Get current bind resources")
	}
	curResIds := make([]string, 0)
	for i := range curRess {
		res := curRess[i]
		if _, ok := setResIds[res.GetId()]; ok {
			curResIds = append(curResIds, res.GetId())
			continue
		}
		jObj, err := s.GetJointResourceCluster(res.GetId())
		if err != nil {
			return nil, errors.Wrapf(err, "Get joint resource cluster by id %s", res.GetId())
		}
		if err := jObj.Detach(ctx, userCred); err != nil {
			return nil, errors.Wrap(err, "detach joint cluster")
		}
	}

	// bind input resources
	jointMan := s.GetJointManager()
	for resId := range setResIds {
		if utils.IsInStringArray(resId, curResIds) {
			// already binded
			continue
		}

		if _, err := InsertJointResourceCluster(ctx, jointMan, resId, s.GetId()); err != nil {
			return nil, errors.Wrapf(err, "InsertJointResourceCluster %s %s", resId, s.GetId())
		}

		//res := setResIds[resId]
		//if err := res.ClearClusterDescCache(); err != nil {
		//	log.Errorf("Resource %s/%s ClearClusterDescCache error: %v", res.Keyword(), res.GetId(), err)
		//}
	}

	return nil, nil
}
