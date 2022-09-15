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

import "time"

type DrsRecordCreateInput struct {
	//ModelBaseCreateInput

	ClusterId  string `json:"cluster_id"`
	ActivityId string `json:"activity_id"`
	GuestId    string `json:"guest_id"`
	GuestName  string `json:"guest_name"`
	Action     string `json:"action"`
	Notes      string `json:"notes"`

	FromHostId   string `json:"from_host_id"`
	FromHostName string `json:"from_host_name"`

	ToHostId   string `json:"to_host_id"`
	ToHostName string `json:"to_host_name"`

	UserId   string `json:"user_id"`
	User     string `json:"user"`
	Strategy string `json:"strategy"`
	//OpsTime   time.Time `json:"ops_time"`
	StartTime time.Time `json:"start_time"`
	Success   bool      `json:"success"`
}

type DrsRecordListInput struct {
	// filter by cluster ids
	ClusterIds []string `json:"cluster_id"`

	// filter by activity ids
	ActivityIds []string `json:"activity_id"`

	// filter by guest ids
	GuestIds []string `json:"guest_id"`
	// filter by guest name
	GuestNames []string `json:"guest_name"`
	// filter by strategy
	Strategy string `json:"strategy"`

	Success *bool `json:"success"`

	Since time.Time `json:"since"`

	Until time.Time `json:"until"`
}

type DrsRecordDetails struct {
	Id         int64  `json:"id"`
	ClusterId  string `json:"cluster_id"`
	ActivityId string `json:"activity_id"`
	GuestId    string `json:"guest_id"`
	GuestName  string `json:"guest_name"`
	Action     string `json:"action"`
	Notes      string `json:"notes"`

	FromHostId   string `json:"from_host_id"`
	FromHostName string `json:"from_host_name"`

	ToHostId   string `json:"to_host_id"`
	ToHostName string `json:"to_host_name"`

	UserId  string    `json:"user_id"`
	User    string    `json:"user"`
	OpsTime time.Time `json:"ops_time"`
	//Success bool      `json:"success"`
	Success *bool `json:"success"`
}
