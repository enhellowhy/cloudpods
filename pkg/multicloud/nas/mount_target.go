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

	api "yunion.io/x/onecloud/pkg/apis/compute"
)

type SMountTarget struct {
	Status            string
	NetworkType       string
	FileSystemId      string
	VpcId             string
	MountTargetDomain string
	AccessGroup       string

	Id   int
	Name string
}

func (self *SMountTarget) GetGlobalId() string {
	//return self.MountTargetDomain
	//return fmt.Sprintf("%s%s", self.VpcId, self.FileSystemId)
	return self.Name
}

func (self *SMountTarget) GetId() int {
	return self.Id
}

func (self *SMountTarget) GetName() string {
	return self.Name
}

func (self *SMountTarget) GetNetworkType() string {
	return strings.ToLower(self.NetworkType)
}

func (self *SMountTarget) GetStatus() string {
	switch self.Status {
	case "active":
		return api.MOUNT_TARGET_STATUS_AVAILABLE
	case "inactive":
		return api.MOUNT_TARGET_STATUS_UNAVAILABLE
	case "pending":
		return api.MOUNT_TARGET_STATUS_CREATING
	case "Deleting":
		return api.MOUNT_TARGET_STATUS_DELETING
	default:
		return strings.ToLower(self.Status)
	}
}

func (self *SMountTarget) GetVpcId() string {
	return self.VpcId
}

func (self *SMountTarget) GetNetworkId() string {
	return self.FileSystemId
}

func (self *SMountTarget) GetDomainName() string {
	return self.MountTargetDomain
}

func (self *SMountTarget) GetAccessGroupId() string {
	//return fmt.Sprintf("%s/%s", self.fs.FileSystemType, self.AccessGroup)
	return "default"
}

func (self *SMountTarget) Delete() error {
	return nil
	//return self.fs.region.DeleteMountTarget(self.fs.FileSystemId, self.MountTargetDomain)
}
