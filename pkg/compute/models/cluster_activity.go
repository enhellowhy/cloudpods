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
	"time"

	"yunion.io/x/jsonutils"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/sqlchemy"

	"yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/util/rbacutils"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
)

type SClusterActivityManager struct {
	db.SStatusStandaloneResourceBaseManager
	SClusterResourceBaseManager
}

type SClusterActivity struct {
	db.SStatusStandaloneResourceBase

	SClusterResourceBase
	MigrateNumber int `list:"user" get:"user" default:"-1"`
	// 起因描述
	TriggerDesc string `width:"256" charset:"ascii" get:"user" list:"user"`
	Strategy    string `width:"16" charset:"ascii" nullable:"false" default:"" list:"user" update:"admin" create:"required"`
	// 行为描述
	ActionDesc string    `width:"1024" charset:"ascii" get:"user" list:"user"`
	StartTime  time.Time `list:"user" get:"user"`
	EndTime    time.Time `list:"user" get:"user"`
	Reason     string    `width:"1024" charset:"ascii" get:"user" list:"user"`
}

var ClusterActivityManager *SClusterActivityManager

func init() {
	ClusterActivityManager = &SClusterActivityManager{
		SStatusStandaloneResourceBaseManager: db.NewStatusStandaloneResourceBaseManager(
			SClusterActivity{},
			"clusteractivities_tbl",
			"clusteractivity",
			"clusteractivities",
		),
	}
	ClusterActivityManager.SetVirtualObject(ClusterActivityManager)
}

func (cam *SClusterActivityManager) FetchByStatus(ctx context.Context, clusterIds, status []string, action string) (ids []string, err error) {
	q := cam.Query("id").In("id", clusterIds)
	if action == "not" {
		q = q.NotIn("status", status)
	} else {
		q = q.In("status", status)
	}
	rows, err := q.Rows()
	if err != nil {
		return nil, errors.Wrap(err, "sQuery.Rows")
	}
	defer rows.Close()
	var id string
	for rows.Next() {
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return
}

func (ca *SClusterActivity) SetFailed(actionDesc, reason string) error {
	return ca.SetResult(actionDesc, compute.SA_STATUS_FAILED, reason, -1)
}

func (ca *SClusterActivity) SetResult(actionDesc, status, reason string, migrateNum int) error {
	_, err := db.Update(ca, func() error {
		if len(actionDesc) != 0 {
			ca.ActionDesc = actionDesc
		}
		ca.EndTime = time.Now()
		ca.Status = status
		if len(reason) != 0 {
			ca.Reason = reason
		}
		if migrateNum != -1 {
			ca.MigrateNumber = migrateNum
		}
		return nil
	})
	return err
}

func (ca *SClusterActivity) SetReject(action string, reason string) error {
	return ca.SetResult(action, compute.SA_STATUS_REJECT, reason, -1)
}

func (cam *SClusterActivityManager) CreateClusterActivity(ctx context.Context, clusterId, triggerDesc, strategy, status string) (*SClusterActivity, error) {
	clusterActivity := &SClusterActivity{
		TriggerDesc: triggerDesc,
		StartTime:   time.Now(),
	}
	clusterActivity.ClusterId = clusterId
	clusterActivity.Name = "drs"
	clusterActivity.Status = status
	clusterActivity.Strategy = strategy
	clusterActivity.SetModelManager(cam, clusterActivity)
	return clusterActivity, cam.TableSpec().Insert(ctx, clusterActivity)
}

func (ca *SClusterActivity) StartToBalance(triggerDesc string) (*SClusterActivity, error) {
	_, err := db.Update(ca, func() error {
		ca.TriggerDesc = triggerDesc
		ca.Status = compute.SA_STATUS_EXEC
		return nil
	})
	return ca, err
}

func (ca *SClusterActivity) SimpleDelete() error {
	_, err := db.Update(ca, func() error {
		ca.MarkDelete()
		return nil
	})
	return err
}

func (cam *SClusterActivityManager) QueryDistinctExtraField(q *sqlchemy.SQuery, field string) (*sqlchemy.SQuery, error) {
	q, err := cam.SStatusStandaloneResourceBaseManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}
	return cam.SClusterResourceBaseManager.QueryDistinctExtraField(q, field)
}

func (cam *SClusterActivityManager) OrderByExtraFields(ctx context.Context, q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential, query compute.ClusterActivityListInput) (*sqlchemy.SQuery, error) {
	return cam.SStatusStandaloneResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.StatusStandaloneResourceListInput)
}

func (cam *SClusterActivityManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []compute.ClusterActivityDetails {
	rows := make([]compute.ClusterActivityDetails, len(objs))
	statusRows := cam.SStatusStandaloneResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)
	clusterRows := cam.SClusterResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)
	for i := range rows {
		rows[i].StatusStandaloneResourceDetails = statusRows[i]
		rows[i].ClusterResourceInfo = clusterRows[i]
	}
	return rows
}

func (cam *SClusterActivityManager) ListItemFilter(ctx context.Context, q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential, input compute.ClusterActivityListInput) (*sqlchemy.SQuery, error) {

	q, err := cam.SStatusStandaloneResourceBaseManager.ListItemFilter(ctx, q, userCred, input.StatusStandaloneResourceListInput)
	if err != nil {
		return nil, err
	}
	q, err = cam.SClusterResourceBaseManager.ListItemFilter(ctx, q, userCred, input.ClusterFilterListInput)
	if err != nil {
		return nil, err
	}
	q = q.Desc("start_time").Desc("end_time")
	return q, nil
}

func (cam *SClusterActivityManager) NamespaceScope() rbacutils.TRbacScope {
	return rbacutils.ScopeProject
}

func (cam *SClusterActivityManager) ResourceScope() rbacutils.TRbacScope {
	return rbacutils.ScopeProject
}

func (cam *SClusterActivityManager) FilterByOwner(q *sqlchemy.SQuery, owner mcclient.IIdentityProvider, scope rbacutils.TRbacScope) *sqlchemy.SQuery {
	if owner != nil {
		switch scope {
		case rbacutils.ScopeProject, rbacutils.ScopeDomain:
			scalingGroupQ := ClusterManager.Query("id", "domain_id").SubQuery()
			q = q.Join(scalingGroupQ, sqlchemy.Equals(q.Field("scaling_group_id"), scalingGroupQ.Field("id")))
			q = q.Filter(sqlchemy.Equals(scalingGroupQ.Field("domain_id"), owner.GetProjectDomainId()))
		}
	}
	return q
}

func (cam *SClusterActivityManager) FetchOwnerId(ctx context.Context, data jsonutils.JSONObject) (mcclient.IIdentityProvider, error) {
	return db.FetchDomainInfo(ctx, data)
}

func (ca *SClusterActivity) GetOwnerId() mcclient.IIdentityProvider {
	cluster := ca.GetCluster()
	if cluster != nil {
		return cluster.GetOwnerId()
	}
	return nil
}
