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
	api "yunion.io/x/onecloud/pkg/apis/compute"
)

type SNasSku struct {
	Name        string
	Id          string
	StorageType string
	Protocol    string
	Provider    string
	SyncOnly    bool
}

func (self *SNasSku) GetName() string {
	return self.Name
}

func (self *SNasSku) GetGlobalId() string {
	return self.Id
}

func (self *SNasSku) GetStorageType() string {
	return self.StorageType
}

func (self *SNasSku) GetFileSystemType() string {
	//return self.StorageType
	return api.NAS_STORAGE_TYPE_STANDARD
}

func (self *SNasSku) GetProvider() string {
	return self.Provider
}

func (self *SNasSku) GetNetworkTypes() string {
	return api.NETWORK_TYPE_CLASSIC
}

func (self *SNasSku) GetDiskSizeStep() int {
	switch self.StorageType {
	case api.NAS_STORAGE_TYPE_STANDARD:
		return 1
	case api.NAS_STORAGE_TYPE_PERFORMANCE:
		return 1
	case api.NAS_STORAGE_TYPE_CAPACITY:
		return 10
	}
	return 1
}

func (self *SNasSku) GetMinDiskSizeGb() int {
	switch self.StorageType {
	case api.NAS_STORAGE_TYPE_STANDARD:
		return 10
	case api.NAS_STORAGE_TYPE_PERFORMANCE:
		return 10
	case api.NAS_STORAGE_TYPE_CAPACITY:
		return 100
	}
	return 10
}

func (self *SNasSku) GetMaxDiskSizeGb() int {
	switch self.StorageType {
	case api.NAS_STORAGE_TYPE_STANDARD:
		return 102400
	case api.NAS_STORAGE_TYPE_PERFORMANCE:
		return 102400
	case api.NAS_STORAGE_TYPE_CAPACITY:
		return 102400
	}
	return 10240
}

func (self *SNasSku) GetProtocol() string {
	return self.Protocol
}

func (self *SNasSku) GetDesc() string {
	switch self.StorageType {
	case api.NAS_STORAGE_TYPE_STANDARD:
		return "标准型"
	case api.NAS_STORAGE_TYPE_PERFORMANCE:
		return "性能型"
	case api.NAS_STORAGE_TYPE_CAPACITY:
		return "容量型"
	}
	return ""
}

func (self *SNasSku) GetPostpaidStatus() string {
	if self.SyncOnly {
		return api.NAS_SKU_SOLDOUT
	}
	return api.NAS_SKU_AVAILABLE
}

func (self *SNasSku) GetPrepaidStatus() string {
	if self.SyncOnly {
		return api.NAS_SKU_SOLDOUT
	}
	return api.NAS_SKU_AVAILABLE
}
