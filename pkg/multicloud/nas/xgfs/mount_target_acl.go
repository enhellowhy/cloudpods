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

package xgfs

import (
	"strconv"
	"yunion.io/x/onecloud/pkg/cloudprovider"
)

type SDfsNfsShareAcl struct {
	UserAccessType     string
	RootUserAccessType string
	RWAccessType       string
	Sync               bool
	Source             string
	Status             string
	Id                 int
	Name               string //fs_client name
}

func (self *SDfsNfsShareAcl) GetId() int {
	return self.Id
}

func (self *SDfsNfsShareAcl) GetName() string {
	return self.Name
}

func (self *SDfsNfsShareAcl) GetGlobalId() string {
	//return fmt.Sprintf("%s/%d", self.GetName(), self.Id)
	return strconv.Itoa(self.Id)
}

func (self *SDfsNfsShareAcl) GetRWAccessType() cloudprovider.TRWAccessType {
	if self.RWAccessType == "RW" {
		return cloudprovider.RWAccessTypeRW
	}
	return cloudprovider.RWAccessTypeR
}

func (self *SDfsNfsShareAcl) GetUserAccessType() cloudprovider.TUserAccessType {
	if self.UserAccessType == "all_squash" {
		return cloudprovider.UserAccessTypeAllSquash
	}
	return cloudprovider.UserAccessTypeNoAllSquash
}

func (self *SDfsNfsShareAcl) GetRootUserAccessType() cloudprovider.TUserAccessType {
	if self.RootUserAccessType == "root_squash" {
		return cloudprovider.UserAccessTypeRootSquash
	}
	return cloudprovider.UserAccessTypeNoRootSquash
}

func (self *SDfsNfsShareAcl) GetSource() string {
	return self.Source
}

func (self *SDfsNfsShareAcl) GetStatus() string {
	return self.Status
}

func (self *SDfsNfsShareAcl) GetSync() bool {
	return self.Sync
}

func (self *SDfsNfsShareAcl) Delete() error {
	//return self.region.DeleteAccessGroup(self.FileSystemType, self.AccessGroupName)
	return nil
}

func (self *SDfsNfsShareAcl) GetSupporedUserAccessTypes() []cloudprovider.TUserAccessType {
	return []cloudprovider.TUserAccessType{
		cloudprovider.UserAccessTypeAllSquash,
		cloudprovider.UserAccessTypeRootSquash,
		cloudprovider.UserAccessTypeNoRootSquash,
		cloudprovider.UserAccessTypeNoAllSquash,
	}
}
