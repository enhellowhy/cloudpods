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

import (
	"time"
	"yunion.io/x/onecloud/pkg/cloudprovider"

	"yunion.io/x/onecloud/pkg/apis"
)

const (
	// 可用
	NAS_STATUS_AVAILABLE = "available"
	// 不可用
	NAS_STATUS_UNAVAILABLE = "unavailable"
	// 扩容中
	NAS_STATUS_EXTENDING = "extending"
	// 创建中
	NAS_STATUS_CREATING = "creating"
	// 权限添加中
	NAS_ACL_STATUS_ADDING = "acl_adding"
	// 创建失败
	NAS_STATUS_CREATE_FAILED = "create_failed"
	// 未知
	NAS_STATUS_UNKNOWN = "unknown"
	// 删除中
	NAS_STATUS_DELETING = "deleting"
	// 权限删除中
	NAS_ACL_STATUS_DELETING  = "acl_deleting"
	NAS_STATUS_DELETE_FAILED = "delete_failed"

	// storage type
	NAS_STORAGE_TYPE_STANDARD    = "standard"
	NAS_STORAGE_TYPE_PERFORMANCE = "performance"
	NAS_STORAGE_TYPE_CAPACITY    = "capacity"

	NAS_UPDATE_TAGS        = "update_tags"
	NAS_UPDATE_TAGS_FAILED = "update_tags_fail"
)

type FileSystemListInput struct {
	//apis.StatusInfrasResourceBaseListInput
	apis.SharableVirtualResourceListInput
	apis.ExternalizedResourceBaseListInput
	ManagedResourceListInput

	RegionalFilterListInput
}

type FileSystemCreateInput struct {
	//apis.StatusInfrasResourceBaseCreateInput
	apis.SharableVirtualResourceCreateInput
	// 协议类型
	// enum: NFS, SMB, CPFS
	Protocol string `json:"protocol"`

	// 文件系统类型
	// enmu: extreme, standard, cpfs
	FileSystemType string `json:"file_system_type"`

	// 云环境
	// enmu: onpremise, public
	CloudEnv string `json:"cloud_env"`

	// 容量大小
	Capacity int64 `json:"capacity"`

	// IP子网Id
	NetworkId string `json:"network_id"`

	// 存储类型
	// enmu: performance, capacity, standard, advance, advance_100, advance_200
	StorageType string `json:"storage_type"`

	// 可用区Id, 若不指定IP子网，此参数必填
	ZoneId string `json:"zone_id"`

	//swagger:ignore
	CloudregionId string `json:"cloudregion_id"`

	// 订阅Id, 若传入network_id此参数可忽略
	ManagerId string `json:"manager_id"`

	// 包年包月时间周期
	Duration string `json:"duration"`

	// 是否自动续费(仅包年包月时生效)
	// default: false
	AutoRenew bool `json:"auto_renew"`

	// 到期释放时间，仅后付费支持
	ExpiredAt time.Time `json:"expired_at"`

	// 计费方式
	// enum: postpaid, prepaid
	BillingType string `json:"billing_type"`
	// swagger:ignore
	BillingCycle string `json:"billing_cycle"`
}

type FileSystemSyncstatusInput struct {
}

type FileSystemDetails struct {
	//apis.StatusInfrasResourceBaseDetails
	apis.SharableVirtualResourceDetails
	ManagedResourceInfo
	CloudregionResourceInfo

	Vpc                   string
	Network               string
	MountTargetDomainName string
	Zone                  string
}

type FileSystemRemoteUpdateInput struct {
	// 是否覆盖替换所有标签
	ReplaceTags *bool `json:"replace_tags" help:"replace all remote tags"`
}

type FileSystemGetFilesInput struct {
	// Prefix
	Prefix string `json:"prefix"`
	// Prefix
	Path string `json:"path"`
	// 分页标识
	PagingMarker string `json:"paging_marker"`
	// 最大输出条目数
	Limit *int `json:"limit"`
}

type FileSystemGetFilesOutput struct {
	// 对象列表
	Data []cloudprovider.SCloudFile `json:"data"`
	// 排序字段，总是name
	// example: name
	MarkerField string `json:"marker_field"`
	// 排序顺序，总是降序
	// example: ASC
	MarkerOrder string `json:"marker_order"`
	// 下一页请求的paging_marker标识
	NextMarker string `json:"next_marker"`
}
