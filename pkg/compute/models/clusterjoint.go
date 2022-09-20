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

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/utils"
	"yunion.io/x/sqlchemy"

	"yunion.io/x/onecloud/pkg/apis"
	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
)

type SClusterJointsManager struct {
	db.SJointResourceBaseManager
	SClusterResourceBaseManager
}

func NewClusterJointsManager(
	dt interface{},
	tableName string,
	keyword string,
	keywordPlural string,
	master db.IStandaloneModelManager,
) *SClusterJointsManager {
	return &SClusterJointsManager{
		SJointResourceBaseManager: db.NewJointResourceBaseManager(
			dt,
			tableName,
			keyword,
			keywordPlural,
			master,
			ClusterManager,
		),
	}
}

type SClusterJointsBase struct {
	db.SJointResourceBase

	ClusterId string `width:"36" charset:"ascii" nullable:"false" list:"admin" create:"admin_required"` // =Column(VARCHAR(36, charset='ascii'), nullable=False)
}

func (manager *SClusterJointsManager) GetSlaveFieldName() string {
	return "cluster_id"
}

func (man *SClusterJointsManager) FetchClusterById(id string) *SCluster {
	clusterObj, _ := ClusterManager.FetchById(id)
	if clusterObj == nil {
		return nil
	}
	return clusterObj.(*SCluster)
}

func (man *SClusterJointsManager) ValidateCreateData(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data *jsonutils.JSONDict) (*jsonutils.JSONDict, error) {
	clusterId, err := data.GetString("cluster_id")
	if err != nil || clusterId == "" {
		return nil, httperrors.NewInputParameterError("cluster_id not provide")
	}
	resourceType := man.GetMasterManager().KeywordPlural()
	if !utils.IsInStringArray(resourceType, ClusterManager.GetResourceTypes()) {
		return nil, httperrors.NewInputParameterError("Not support resource_type %s", resourceType)
	}
	cluster := man.FetchClusterById(clusterId)
	if cluster == nil {
		return nil, httperrors.NewNotFoundError("Cluster %s", clusterId)
	}
	if resourceType != cluster.ResourceType {
		return nil, httperrors.NewInputParameterError("Cluster %s resource_type mismatch: %s != %s", cluster.GetName(), cluster.ResourceType, resourceType)
	}

	input := apis.JoinResourceBaseCreateInput{}
	err = data.Unmarshal(&input)
	if err != nil {
		return nil, httperrors.NewInternalServerError("unmarshal JointResourceCreateInput fail %s", err)
	}
	input, err = man.SJointResourceBaseManager.ValidateCreateData(ctx, userCred, ownerId, query, input)
	if err != nil {
		return nil, err
	}
	data.Update(jsonutils.Marshal(input))
	return data, nil
}

func (man *SClusterJointsManager) GetResourceIdKey(m db.IJointModelManager) string {
	return fmt.Sprintf("%s_id", m.GetMasterManager().Keyword())
}

func (joint *SClusterJointsBase) GetClusterId() string {
	return joint.ClusterId
}

func (manager *SClusterJointsManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []interface{} {
	rows := make([]interface{}, len(objs))

	jointRows := manager.SJointResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)

	clusterIds := make([]string, len(rows))
	resIds := make([]string, len(rows))
	for i := range rows {
		rows[i] = api.ClusterJointResourceDetails{
			JointResourceBaseDetails: jointRows[i],
		}
		obj := objs[i].(IClusterJointModel)
		clusterIds[i] = obj.GetClusterId()
		resIds[i] = obj.GetResourceId()
	}

	clusters := make(map[string]SCluster)
	err := db.FetchStandaloneObjectsByIds(ClusterManager, clusterIds, &clusters)
	if err != nil {
		log.Errorf("FetchStandaloneObjectsByIds fail %s", err)
		return rows
	}

	for i := range rows {
		if cluster, ok := clusters[clusterIds[i]]; ok {
			out := rows[i].(api.ClusterJointResourceDetails)
			out.Cluster = cluster.Name
			out.ResourceType = cluster.ResourceType
			rows[i] = out
		}
	}

	resIdMaps, err := db.FetchIdNameMap2(manager.GetMasterManager(), resIds)
	if err != nil {
		log.Errorf("FetchIdNameMap2 %sIds error: %v", manager.GetMasterManager().Keyword(), err)
		return rows
	}

	for idx := range objs {
		obj := objs[idx].(IClusterJointModel)
		baseDetail := rows[idx].(api.ClusterJointResourceDetails)
		out := obj.GetDetails(baseDetail, resIdMaps[resIds[idx]], isList)
		rows[idx] = out
	}

	return rows
}

func (joint *SClusterJointsBase) Delete(ctx context.Context, userCred mcclient.TokenCredential) error {
	return fmt.Errorf("Delete must be override")
}

func (joint *SClusterJointsBase) delete(obj db.IJointModel, ctx context.Context, userCred mcclient.TokenCredential) error {
	return db.DeleteModel(ctx, userCred, obj)
}

func (joint *SClusterJointsBase) Detach(ctx context.Context, userCred mcclient.TokenCredential) error {
	return fmt.Errorf("Detach must be override")
}

func (joint *SClusterJointsBase) detach(obj db.IJointModel, ctx context.Context, userCred mcclient.TokenCredential) error {
	return db.DetachJoint(ctx, userCred, obj)
}

func (manager *SClusterJointsManager) ListItemFilter(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.ClusterJointsListInput,
) (*sqlchemy.SQuery, error) {
	var err error

	q, err = manager.SJointResourceBaseManager.ListItemFilter(ctx, q, userCred, query.JointResourceBaseListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SJointResourceBaseManager.ListItemFilter")
	}
	q, err = manager.SClusterResourceBaseManager.ListItemFilter(ctx, q, userCred, query.ClusterFilterListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SClusterResourceBaseManager.ListItemFilter")
	}

	return q, nil
}

func (manager *SClusterJointsManager) OrderByExtraFields(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.ClusterJointsListInput,
) (*sqlchemy.SQuery, error) {
	var err error

	q, err = manager.SJointResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.JointResourceBaseListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SJointResourceBaseManager.OrderByExtraFields")
	}
	q, err = manager.SClusterResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.ClusterFilterListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SClusterResourceBaseManager.OrderByExtraFields")
	}

	return q, nil
}
