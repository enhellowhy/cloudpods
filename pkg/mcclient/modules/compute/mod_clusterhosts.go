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
	"yunion.io/x/onecloud/pkg/mcclient/modulebase"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
)

var (
	Clusterhosts modulebase.JointResourceManager
	Clusterzones modulebase.JointResourceManager
)

func newClusterJointManager(keyword, keywordPlural string, columns, adminColumns []string, slave modulebase.Manager) modulebase.JointResourceManager {
	columns = append(columns, "Cluster_ID", "Cluster")
	return modules.NewJointComputeManager(keyword, keywordPlural,
		columns, adminColumns, &Clusters, slave)
}

func init() {
	Clusterhosts = newClusterJointManager("clusterhost", "clusterhosts",
		[]string{"Host_ID", "Host"},
		[]string{},
		&Hosts)

	Clusterzones = newClusterJointManager("clusterzone", "clusterzones",
		[]string{"Zone_ID", "Zone"},
		[]string{},
		&Zones)

	for _, m := range []modulebase.IBaseManager{
		&Clusterhosts,
		&Clusterzones,
	} {
		modules.RegisterCompute(m)
	}
}
