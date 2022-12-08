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

package cloudprovider

type SNfsShareCreateOptions struct {
	DfsNfsShareAcls []SDfsNfsShareAclCreateOptions
	Name            string
	Path            string
	DfsRootfsId     int
}

type SNfsShareAclAddOptions struct {
	DfsNfsShareAcls []SDfsNfsShareAclCreateOptions
}

type SNfsShareAclUpdateOptions struct {
	DfsNfsShareAcls []SDfsNfsShareAclUpdateOptions
}

type SNfsShareAclRemoveOptions struct {
	DfsNfsShareAclIds []int
}

type SDfsNfsShareAclCreateOptions struct {
	AllSquash  bool   `json:"all_squash"`
	Permission string `json:"permission"`
	Sync       bool   `json:"sync"`
	FsClientId int    `json:"fs_client_id"`
	RootSquash bool   `json:"root_squash"`
	Type       string `json:"type"` //client
	//Id              int    `json:"id"`
	//FsClientGroupId int    `json:"fs_client_group_id"`
}

type SDfsNfsShareAclUpdateOptions struct {
	AllSquash  bool   `json:"all_squash"`
	Permission string `json:"permission"`
	Sync       bool   `json:"sync"`
	RootSquash bool   `json:"root_squash"`
	Id         int    `json:"id"`
}

type MountTargetAcl struct {
	Id                 string
	ExternalId         string
	RWAccessType       TRWAccessType
	UserAccessType     TUserAccessType
	RootUserAccessType TUserAccessType
	Source             string
	Sync               bool
}
