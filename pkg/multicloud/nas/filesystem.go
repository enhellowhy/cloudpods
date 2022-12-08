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

package nas

import (
	"strings"
	"time"
	billing_api "yunion.io/x/onecloud/pkg/apis/billing"
	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/multicloud"
)

type MountTargets struct {
	MountTarget []SMountTarget
}

type SFileSystem struct {
	multicloud.SNasBase
	multicloud.STagBase
	//multicloud.AliyunTags

	client INasProvider

	Status                string
	Description           string
	StorageType           string
	MountTargetCountLimit int
	Shared                bool
	//ZoneId                string
	//RegionId             string
	// 2021-03-22T10:08:15CST
	CreateTime           string
	ModifyTime           time.Time
	MountTargets         MountTargets
	AutoSnapshotPolicyId string
	Size                 int64
	EncryptType          int
	Capacity             int64
	Files                int64
	FileQuota            int64
	Path                 string
	ProtocolType         string
	ChargeType           string
	ExpiredTime          string
	FileSystemType       string
	FileSystemId         string
	FileSystemName       string
}

func (self *SFileSystem) GetINasProvider() INasProvider {
	return self.client
}

func (self *SFileSystem) GetProjectId() string {
	return ""
}

func (self *SFileSystem) GetPath() string {
	return self.Path
}

func (self *SFileSystem) GetZoneId() string {
	return ""
}

func (self *SFileSystem) GetId() string {
	return self.FileSystemId
}

func (self *SFileSystem) GetGlobalId() string {
	return self.FileSystemId
}

func (self *SFileSystem) GetName() string {
	return self.FileSystemName
}

func (self *SFileSystem) GetFileSystemType() string {
	return self.FileSystemType
}

func (self *SFileSystem) GetMountTargetCountLimit() int {
	return self.MountTargetCountLimit
}

func (self *SFileSystem) GetStatus() string {
	switch self.Status {
	case "", "Running":
		return api.NAS_STATUS_AVAILABLE
	case "Extending":
		return api.NAS_STATUS_EXTENDING
	case "Stopping", "Stopped":
		return api.NAS_STATUS_UNAVAILABLE
	case "Pending":
		return api.NAS_STATUS_CREATING
	default:
		return api.NAS_STATUS_UNKNOWN
	}
}

func (self *SFileSystem) GetBillintType() string {
	return billing_api.BILLING_TYPE_POSTPAID
}

func (self *SFileSystem) GetStorageType() string {
	return strings.ToLower(self.StorageType)
}

func (self *SFileSystem) GetProtocol() string {
	return self.ProtocolType
}

func (self *SFileSystem) GetCapacityGb() int64 {
	return self.Capacity
}

func (self *SFileSystem) GetUsedCapacityGb() int64 {
	return self.Size
}

func (self *SFileSystem) GetFileQuota() int64 {
	return self.FileQuota
}

func (self *SFileSystem) GetFileCount() int64 {
	return self.Files
}

func (self *SFileSystem) IsShared() bool {
	return self.Shared
}

func (self *SFileSystem) Delete() error {
	//return self.client.DeleteFileSystem(self.FileSystemId)
	return nil
}

func (self *SFileSystem) GetLastModified() time.Time {
	return self.ModifyTime
}

func (self *SFileSystem) GetCreatedAt() time.Time {
	ret, _ := time.Parse("2006-01-02T15:04:05CST", self.CreateTime)
	if !ret.IsZero() {
		ret = ret.Add(time.Hour * 8)
	}
	return ret
}

func (self *SFileSystem) GetExpiredAt() time.Time {
	ret, _ := time.Parse("2006-01-02T15:04:05CST", self.ExpiredTime)
	if !ret.IsZero() {
		ret = ret.Add(time.Hour * 8)
	}
	return ret
}

//func (self *SRegion) CreateMountTarget(opts *cloudprovider.SMountTargetCreateOptions) (*SMountTarget, error) {
//	params := map[string]string{
//		"RegionId":        self.RegionId,
//		"FileSystemId":    opts.FileSystemId,
//		"AccessGroupName": strings.TrimPrefix(strings.TrimPrefix(opts.AccessGroupId, "extreme/"), "standard/"),
//		"NetworkType":     utils.Capitalize(opts.NetworkType),
//		"VpcId":           opts.VpcId,
//		"VSwitchId":       opts.NetworkId,
//	}
//	resp, err := self.nasRequest("CreateMountTarget", params)
//	if err != nil {
//		return nil, errors.Wrapf(err, "CreateMountTarget")
//	}
//	ret := struct {
//		MountTargetDomain string
//	}{}
//	err = resp.Unmarshal(&ret)
//	if err != nil {
//		return nil, errors.Wrapf(err, "resp.Unmarshal")
//	}
//	mts, _, err := self.GetMountTargets(opts.FileSystemId, ret.MountTargetDomain, 10, 1)
//	if err != nil {
//		return nil, errors.Wrapf(err, "self.GetMountTargets")
//	}
//	for i := range mts {
//		if mts[i].MountTargetDomain == ret.MountTargetDomain {
//			return &mts[i], nil
//		}
//	}
//	return nil, errors.Wrapf(cloudprovider.ErrNotFound, "afeter create with mount domain %s", ret.MountTargetDomain)
//}
//

//
//func (self *SRegion) DeleteFileSystem(id string) error {
//	params := map[string]string{
//		"RegionId":     self.RegionId,
//		"FileSystemId": id,
//	}
//	_, err := self.nasRequest("DeleteFileSystem", params)
//	return errors.Wrapf(err, "DeleteFileSystem")
//}

//func (self *SRegion) CreateICloudFileSystem(opts *cloudprovider.FileSystemCraeteOptions) (cloudprovider.ICloudFileSystem, error) {
//	fs, err := self.CreateFileSystem(opts)
//	if err != nil {
//		return nil, errors.Wrapf(err, "self.CreateFileSystem")
//	}
//	return fs, nil
//}
//
//func (self *SRegion) CreateFileSystem(opts *cloudprovider.FileSystemCraeteOptions) (*SFileSystem, error) {
//	params := map[string]string{
//		"RegionId":       self.RegionId,
//		"ProtocolType":   opts.Protocol,
//		"ZoneId":         opts.ZoneId,
//		"EncryptType":    "0",
//		"FileSystemType": opts.FileSystemType,
//		"StorageType":    opts.StorageType,
//		"ClientToken":    utils.GenRequestId(20),
//		"Description":    opts.Name,
//	}
//
//	if self.GetCloudEnv() == ALIYUN_FINANCE_CLOUDENV {
//		if opts.FileSystemType == "standard" {
//			opts.ZoneId = strings.Replace(opts.ZoneId, "cn-shanghai-finance-1", "jr-cn-shanghai-", 1)
//			opts.ZoneId = strings.Replace(opts.ZoneId, "cn-shenzhen-finance-1", "jr-cn-shenzhen-", 1)
//			params["ZoneId"] = opts.ZoneId
//		}
//	}
//
//	switch opts.FileSystemType {
//	case "standard":
//		params["StorageType"] = utils.Capitalize(opts.StorageType)
//	case "cpfs":
//		params["ProtocolType"] = "cpfs"
//		switch opts.StorageType {
//		case "advance_100":
//			params["Bandwidth"] = "100"
//		case "advance_200":
//			params["Bandwidth"] = "200"
//		}
//	case "extreme":
//		params["Capacity"] = fmt.Sprintf("%d", opts.Capacity)
//	}
//	if len(opts.VpcId) > 0 {
//		params["VpcId"] = opts.VpcId
//	}
//	if len(opts.NetworkId) > 0 {
//		params["VSwitchId"] = opts.NetworkId
//	}
//	if opts.BillingCycle != nil {
//		params["ChargeType"] = "Subscription"
//		params["Duration"] = fmt.Sprintf("%d", opts.BillingCycle.GetMonths())
//	}
//	resp, err := self.nasRequest("CreateFileSystem", params)
//	if err != nil {
//		return nil, errors.Wrapf(err, "CreateFileSystem")
//	}
//	fsId, _ := resp.GetString("FileSystemId")
//	return self.GetFileSystem(fsId)
//}
//
//func (self *SFileSystem) SetTags(tags map[string]string, replace bool) error {
//	return self.region.SetResourceTags(ALIYUN_SERVICE_NAS, "filesystem", self.FileSystemId, tags, replace)
//}
