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

const (
	MOUNT_TARGET_ACL_STATUS_AVAILABLE     = "available"
	MOUNT_TARGET_ACL_STATUS_UNAVAILABLE   = "unavailable"
	MOUNT_TARGET_ACL_STATUS_ADDING        = "adding"
	MOUNT_TARGET_ACL_STATUS_ADD_FAILED    = "add_failed"
	MOUNT_TARGET_ACL_STATUS_UPDATING      = "updating"
	MOUNT_TARGET_ACL_STATUS_UPDATE_FAILED = "update_failed"
	MOUNT_TARGET_ACL_STATUS_REMOVING      = "removing"
	MOUNT_TARGET_ACL_STATUS_REMOVE_FAILED = "remove_failed"
	MOUNT_TARGET_ACL_STATUS_UNKNOWN       = "unknown"
)

type MountTargetAclListInput struct {
	apis.StatusStandaloneResourceListInput

	// MountId
	//MountTargetId string `json:"mount_target_id"`
	// MountId
	FileSystemId string `json:"file_system_id"`
}

type MountTargetAclDetails struct {
	apis.StatusStandaloneResourceDetails
}

type MountTargetAclCreateInput struct {
	apis.StatusStandaloneResourceCreateInput

	Source       string
	RWAccessType string
	Description  string

	Sync               *bool
	UserAccessType     string
	RootUserAccessType string
	//MountTargetId      string
	FileSystemId string
	Name         string
}

type MountTargetAclUpdateInput struct {
	//Source      string
	Sync        *bool
	Description string

	RWAccessType       string
	UserAccessType     string
	RootUserAccessType string
}
