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

package modules

import (
	"sync"

	"yunion.io/x/jsonutils"

	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modulebase"
)

type ClusterManager struct {
	modulebase.ResourceManager
}

var (
	Clusters ClusterManager
)

func (this *ClusterManager) DoBatchClusterHostAddRemove(s *mcclient.ClientSession, params jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	var wg sync.WaitGroup
	ret := jsonutils.NewDict()
	hosts, e := params.GetArray("hosts")
	if e != nil {
		return ret, e
	}
	clusters, e := params.GetArray("clusters")
	if e != nil {
		return ret, e
	}
	action, e := params.GetString("action")
	if e != nil {
		return ret, e
	}

	wg.Add(len(clusters) * len(hosts))

	for _, host := range hosts {
		for _, cluster := range clusters {
			go func(host, cluster jsonutils.JSONObject) {
				defer wg.Done()
				_host, _ := host.GetString()
				_cluster, _ := cluster.GetString()
				if action == "remove" {
					Clusterhosts.Detach(s, _cluster, _host, nil)
				} else if action == "add" {
					Clusterhosts.Attach(s, _cluster, _host, nil)
				}
			}(host, cluster)
		}
	}

	wg.Wait()
	return ret, nil
}

func init() {
	Clusters = ClusterManager{NewComputeManager("cluster", "clusters",
		[]string{"ID", "Name", "Resource_type", "Domain_id", "Project_id", "Metadata"},
		[]string{})}

	registerCompute(&Clusters)
}
