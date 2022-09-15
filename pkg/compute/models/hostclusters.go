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

	"yunion.io/x/pkg/errors"
	"yunion.io/x/sqlchemy"

	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/mcclient"
)

var (
	HostclusterManager *SHostclusterManager
	_                  IClusterJointModel = new(SHostcluster) //希望在代码中判断SHostcluster这个struct是否实现了IClusterJointModel这个interface，用作类型断言，如果没有实现，则会报编译错误
)

func init() {
	db.InitManager(func() {
		HostclusterManager = &SHostclusterManager{
			SClusterJointsManager: NewClusterJointsManager(
				SHostcluster{},
				"cluster_hosts_tbl",
				"clusterhost",
				"clusterhosts",
				HostManager,
			),
		}
		HostclusterManager.SetVirtualObject(HostclusterManager)
	})
}

type SHostclusterManager struct {
	*SClusterJointsManager
	resourceBaseManager SHostResourceBaseManager
}

type SHostcluster struct {
	SClusterJointsBase

	HostId string `width:"36" charset:"ascii" nullable:"false" list:"admin" create:"admin_required"` // Column(VARCHAR(36, charset='ascii'), nullable=False)
}

func (manager *SHostclusterManager) GetMasterFieldName() string {
	return "host_id"
}

func (self *SHostcluster) GetDetails(base api.ClusterJointResourceDetails, resourceName string, isList bool) interface{} {
	out := api.HostclusterDetails{
		ClusterJointResourceDetails: base,
	}
	out.Host = resourceName
	return out
}

func (self *SHostcluster) Delete(ctx context.Context, userCred mcclient.TokenCredential) error {
	return self.SClusterJointsBase.delete(self, ctx, userCred)
}

func (self *SHostcluster) Detach(ctx context.Context, userCred mcclient.TokenCredential) error {
	return self.SClusterJointsBase.detach(self, ctx, userCred)
}

func (self *SHostcluster) GetResourceId() string {
	return self.HostId
}

func (manager *SHostclusterManager) ListItemFilter(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.HostclusterListInput,
) (*sqlchemy.SQuery, error) {
	var err error

	q, err = manager.SClusterJointsManager.ListItemFilter(ctx, q, userCred, query.ClusterJointsListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SClusterJointsManager.ListItemFilter")
	}
	q, err = manager.resourceBaseManager.ListItemFilter(ctx, q, userCred, query.HostFilterListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SHostResourceBaseManager.ListItemFilter")
	}

	return q, nil
}

func (manager *SHostclusterManager) OrderByExtraFields(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.HostclusterListInput,
) (*sqlchemy.SQuery, error) {
	var err error

	q, err = manager.SClusterJointsManager.OrderByExtraFields(ctx, q, userCred, query.ClusterJointsListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SSchedtagJointsManager.OrderByExtraFields")
	}
	q, err = manager.resourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.HostFilterListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SHostResourceBaseManager.OrderByExtraFields")
	}

	return q, nil
}
