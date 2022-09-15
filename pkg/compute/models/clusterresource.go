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

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/util/reflectutils"
	"yunion.io/x/sqlchemy"

	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
)

type SClusterResourceBase struct {
	// 归属cluster ID
	ClusterId string `width:"36" charset:"ascii" nullable:"false" list:"user" create:"required" update:"user" json:"cluster_id"`
}

type SClusterResourceBaseManager struct{}

func ValidateClusterResourceInput(userCred mcclient.TokenCredential, query api.ClusterResourceInput) (*SCluster, api.ClusterResourceInput, error) {
	clusterObj, err := ClusterManager.FetchByIdOrName(userCred, query.ClusterId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, query, errors.Wrapf(httperrors.ErrResourceNotFound, "%s %s", ClusterManager.Keyword(), query.ClusterId)
		} else {
			return nil, query, errors.Wrap(err, "ClusterManager.FetchByIdOrName")
		}
	}
	query.ClusterId = clusterObj.GetId()
	return clusterObj.(*SCluster), query, nil
}

func (self *SClusterResourceBase) GetCluster() *SCluster {
	obj, err := ClusterManager.FetchById(self.ClusterId)
	if err != nil {
		log.Errorf("fail to fetch cluster by id %s", err)
		return nil
	}
	return obj.(*SCluster)
}

func (manager *SClusterResourceBaseManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []api.ClusterResourceInfo {
	rows := make([]api.ClusterResourceInfo, len(objs))
	clusterIds := make([]string, len(objs))
	for i := range objs {
		var base *SClusterResourceBase
		err := reflectutils.FindAnonymouStructPointer(objs[i], &base)
		if err != nil {
			log.Errorf("Cannot find SClusterResourceBase in object %s", objs[i])
			continue
		}
		clusterIds[i] = base.ClusterId
	}
	clusters := make(map[string]SCluster)
	err := db.FetchStandaloneObjectsByIds(ClusterManager, clusterIds, clusters)
	if err != nil {
		log.Errorf("FetchStandaloneObjectsByIds fail %s", err)
		return rows
	}
	for i := range rows {
		rows[i] = api.ClusterResourceInfo{}
		if cluster, ok := clusters[clusterIds[i]]; ok {
			rows[i].Cluster = cluster.Name
			rows[i].ResourceType = cluster.ResourceType
		}
	}
	return rows
}

func (manager *SClusterResourceBaseManager) ListItemFilter(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.ClusterFilterListInput,
) (*sqlchemy.SQuery, error) {
	if len(query.ClusterId) > 0 {
		clusterObj, _, err := ValidateClusterResourceInput(userCred, query.ClusterResourceInput)
		if err != nil {
			return nil, errors.Wrap(err, "ValidateClusterResourceInput")
		}
		q = q.Equals("cluster_id", clusterObj.GetId())
	}
	return q, nil
}

func (manager *SClusterResourceBaseManager) OrderByExtraFields(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.ClusterFilterListInput,
) (*sqlchemy.SQuery, error) {
	if !db.NeedOrderQuery(manager.GetOrderByFields(query)) {
		return q, nil
	}
	orderQ := ClusterManager.Query("id")
	orderSubQ := orderQ.SubQuery()
	orderQ, orders, fields := manager.GetOrderBySubQuery(orderQ, orderSubQ, orderQ.Field("id"), userCred, query, nil, nil)
	q = q.LeftJoin(orderSubQ, sqlchemy.Equals(q.Field("cluster_id"), orderSubQ.Field("id")))
	q = db.OrderByFields(q, orders, fields)
	return q, nil
}

func (manager *SClusterResourceBaseManager) QueryDistinctExtraField(q *sqlchemy.SQuery, field string) (*sqlchemy.SQuery, error) {
	if field == "cluster" {
		clusterQuery := ClusterManager.Query("name", "id").Distinct().SubQuery()
		q.AppendField(clusterQuery.Field("name", field))
		q = q.Join(clusterQuery, sqlchemy.Equals(q.Field("cluster_id"), clusterQuery.Field("id")))
		q.GroupBy(clusterQuery.Field("name"))
		return q, nil
	}
	return q, httperrors.ErrNotFound
}

func (manager *SClusterResourceBaseManager) GetOrderBySubQuery(
	q *sqlchemy.SQuery,
	subq *sqlchemy.SSubQuery,
	joinField sqlchemy.IQueryField,
	userCred mcclient.TokenCredential,
	query api.ClusterFilterListInput,
	orders []string,
	fields []sqlchemy.IQueryField,
) (*sqlchemy.SQuery, []string, []sqlchemy.IQueryField) {
	if !db.NeedOrderQuery(manager.GetOrderByFields(query)) {
		return q, orders, fields
	}
	tagQ := ClusterManager.Query().SubQuery()
	q = q.LeftJoin(tagQ, sqlchemy.Equals(joinField, tagQ.Field("id")))
	q = q.AppendField(tagQ.Field("name").Label("cluster"))
	q = q.AppendField(tagQ.Field("resource_type").Label("resource_type"))
	orders = append(orders, query.OrderByCluster)
	fields = append(fields, subq.Field("cluster"), subq.Field("resource_type"))
	return q, orders, fields
}

func (manager *SClusterResourceBaseManager) GetOrderByFields(query api.ClusterFilterListInput) []string {
	return []string{query.OrderByCluster}
}

func InsertJointResourceCluster(ctx context.Context, jointMan IClusterJointManager, resourceId, clusterId string) (IClusterJointModel, error) {
	newTagObj, err := db.NewModelObject(jointMan)
	if err != nil {
		return nil, errors.Wrap(err, "NewModelObject")
	}

	objectKey := jointMan.GetResourceIdKey(jointMan)
	createData := jsonutils.NewDict()
	createData.Add(jsonutils.NewString(clusterId), "cluster_id")
	createData.Add(jsonutils.NewString(resourceId), objectKey)
	if err := createData.Unmarshal(newTagObj); err != nil {
		return nil, errors.Wrapf(err, "Create %s joint cluster", jointMan.Keyword())
	}
	if err := newTagObj.GetModelManager().TableSpec().Insert(ctx, newTagObj); err != nil {
		return nil, errors.Wrap(err, "Insert to database")
	}

	return newTagObj.(IClusterJointModel), nil
}
