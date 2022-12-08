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
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/multicloud/nas"

	"yunion.io/x/pkg/errors"
)

type SXgfsFileSystem struct {
	*nas.SFileSystem

	client *SXgfsClient
}

func (self *SXgfsFileSystem) GetMountTargets() ([]cloudprovider.ICloudMountTarget, error) {
	shares, err := self.client.GetDfsShares(self)
	if err != nil {
		return nil, errors.Wrap(err, "GetDfsShares")
	}
	iShares := make([]cloudprovider.ICloudMountTarget, len(shares))
	for i := range shares {
		iShares[i] = shares[i]
	}
	return iShares, nil
}

func (self *SXgfsFileSystem) CreateMountTarget(opts *cloudprovider.SMountTargetCreateOptions) (cloudprovider.ICloudMountTarget, error) {
	return nil, nil
}

func (self *SXgfsFileSystem) Delete() error {
	err := self.client.DeleteFileSystem(self.FileSystemId)
	if err != nil {
		return errors.Wrapf(err, "DeleteFileSystem(%s)", self.FileSystemId)
	}
	return nil
}

func (self *SXgfsFileSystem) Refresh() error {
	fs, err := self.client.GetFileSystem(self.FileSystemId)
	if err != nil {
		if errors.Cause(err) == cloudprovider.ErrNotFound {
			return errors.Wrapf(cloudprovider.ErrNotFound, self.FileSystemId)
		}
		return errors.Wrapf(err, "GetFileSystem(%s)", self.FileSystemId)
	}
	return jsonutils.Update(self, fs)
}

func (self *SXgfsFileSystem) CreateDfsNfsShare(opts cloudprovider.SDfsNfsShareAclCreateOptions) (cloudprovider.ICloudMountTarget, error) {
	params := cloudprovider.SNfsShareCreateOptions{
		DfsNfsShareAcls: []cloudprovider.SDfsNfsShareAclCreateOptions{opts},
		Name:            self.GetName(),
		Path:            self.GetPath(),
	}
	mt, err := self.client.CreateDfsNfsShare(&params, self)
	if err != nil {
		return nil, errors.Wrapf(err, "CreateDfsNfsShare")
	}
	//mt.fs = self
	return mt, nil
}

func (self *SXgfsFileSystem) DeleteDfsNfsShare(shareId int) (cloudprovider.ICloudMountTarget, error) {
	mt, err := self.client.DeleteDfsNfsShare(shareId, self)
	if err != nil {
		return nil, errors.Wrapf(err, "DeleteDfsNfsShare")
	}
	//mt.fs = self
	return mt, nil
}

func (self *SXgfsFileSystem) GetMountTargetAcls(shareId int) ([]cloudprovider.ICloudMountTargetAcl, error) {
	acls, err := self.client.GetDfsShareAcls(shareId)
	if err != nil {
		return nil, errors.Wrapf(err, "GetDfsShareAcls")
	}
	iAcls := make([]cloudprovider.ICloudMountTargetAcl, len(acls))
	for i := range acls {
		iAcls[i] = acls[i]
	}
	return iAcls, nil
}

func (self *SXgfsFileSystem) GetMountTargetAclsById(shareId, id int) ([]cloudprovider.ICloudMountTargetAcl, error) {
	acls, err := self.client.GetDfsShareAclsById(shareId, id)
	if err != nil {
		return nil, errors.Wrapf(err, "GetDfsShareAclsById")
	}
	iAcls := make([]cloudprovider.ICloudMountTargetAcl, len(acls))
	for i := range acls {
		iAcls[i] = acls[i]
	}
	return iAcls, nil
}

func (self *SXgfsFileSystem) AddDfsNfsShareAcl(opts cloudprovider.SDfsNfsShareAclCreateOptions, shareId int) (cloudprovider.ICloudMountTarget, error) {
	params := cloudprovider.SNfsShareAclAddOptions{
		DfsNfsShareAcls: []cloudprovider.SDfsNfsShareAclCreateOptions{opts},
	}
	mt, err := self.client.AddDfsNfsShareAcl(&params, self, shareId)
	if err != nil {
		return nil, errors.Wrapf(err, "AddDfsNfsShareAcl")
	}
	return mt, nil
}

func (self *SXgfsFileSystem) UpdateDfsNfsShareAcl(opts cloudprovider.SDfsNfsShareAclUpdateOptions, shareId int) (cloudprovider.ICloudMountTarget, error) {
	params := cloudprovider.SNfsShareAclUpdateOptions{
		DfsNfsShareAcls: []cloudprovider.SDfsNfsShareAclUpdateOptions{opts},
	}
	mt, err := self.client.UpdateDfsNfsShareAcl(&params, self, shareId)
	if err != nil {
		return nil, errors.Wrapf(err, "UpdateDfsNfsShareAcl")
	}
	return mt, nil
}

func (self *SXgfsFileSystem) RemoveDfsNfsShareAcl(aclId, shareId int) (cloudprovider.ICloudMountTarget, error) {
	params := cloudprovider.SNfsShareAclRemoveOptions{
		DfsNfsShareAclIds: []int{aclId},
	}
	mt, err := self.client.RemoveDfsNfsShareAcl(&params, self, shareId)
	if err != nil {
		return nil, errors.Wrapf(err, "RemoveDfsNfsShareAcl")
	}
	return mt, nil
}
