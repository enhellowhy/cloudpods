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

package compute

import "yunion.io/x/onecloud/pkg/apis"

type ClusterShortDescDetails struct {
	*apis.StandaloneResourceShortDescDetail
	Strategy string `json:"strategy"`
}

type ClusterCreateInput struct {
	apis.StandaloneResourceCreateInput
	apis.ScopedResourceCreateInput

	// drs平衡策略
	// enum: usage, assigned
	Strategy string `json:"strategy"`
	// 资源类型
	// enum: servers, hosts, .....
	// default: hosts
	ResourceType string `json:"resource_type"`
}

type ClusterResourceInput struct {
	// 以关联的集群（ID或Name）过滤列表
	ClusterId string `json:"cluster_id"`
	// swagger:ignore
	// Deprecated
	// filter by cluster_id
	Cluster string `json:"cluster" yunion-deprecated-by:"cluster_id"`
}

type ClusterFilterListInput struct {
	ClusterResourceInput

	// 按名称排序
	// pattern:asc|desc
	OrderByCluster string `json:"order_by_cluster"`
}

type ClusterListInput struct {
	apis.StandaloneResourceListInput
	apis.ScopedResourceBaseListInput
	CloudproviderResourceInput

	// fitler by resource_type
	ResourceType []string `json:"resource_type"`
	// swagger:ignore
	// Deprecated
	// filter by type, alias for resource_type
	Type string `json:"type" yunion-deprecated-by:"resource_type"`

	Strategy []string `json:"strategy"`
}

type ClusterDetails struct {
	apis.StandaloneResourceDetails
	apis.ScopedResourceBaseInfo

	SCluster

	// ProjectId            string `json:"project_id"`
	HostCount        int    `json:"host_count"`
	ServerCount      int    `json:"server_count"`
	OtherCount       int    `json:"other_count"`
	ResourceCount    int    `json:"resource_count"`
	JoinModelKeyword string `json:"join_model_keyword"`
}

type ClusterResourceInfo struct {

	// 调度标签名称
	Cluster string `json:"cluster"`

	// 集群管理的资源类型
	ResourceType string `json:"resource_type"`
}

type ClusterJointResourceDetails struct {
	apis.JointResourceBaseDetails

	// 集群名称
	Cluster string `json:"cluster"`

	// 管理的资源类型
	ResourceType string `json:"resource_type"`
}

type ClusterJointsListInput struct {
	apis.JointResourceBaseListInput
	ClusterFilterListInput
}

type ClusterSetResourceInput struct {
	ResourceIds []string `json:"resource_ids"`
}

type HostclusterDetails struct {
	ClusterJointResourceDetails

	HostJointResourceDetailsBase

	SHostcluster
}

type HostclusterListInput struct {
	ClusterJointsListInput
	HostFilterListInput
}
