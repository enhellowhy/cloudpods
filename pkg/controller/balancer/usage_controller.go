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

package balancer

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"sort"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/apis/compute"
	computeapi "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/apis/monitor"
	"yunion.io/x/onecloud/pkg/compute/models"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/pkg/errors"
)

const (
	USAGE_THRESHOLD = 90
)

func (bc *SBalancerController) CheckUsageBalance(ctx context.Context, userCred mcclient.TokenCredential, isStart bool) {
	session := auth.GetSession(ctx, userCred, "")
	clusterUsages, err := bc.UsageClustersNeedBalance(session)
	if err != nil {
		log.Errorf("UsageClustersNeedBalance: %s", err.Error())
		return
	}
	if len(clusterUsages) == 0 {
		log.Infoln("No usage cluster to balance")
		return
	}
	for _, cluster := range clusterUsages {
		insert := bc.clusterSet.CheckAndInsert(cluster.ID)
		if !insert {
			log.Warningf("A balance activity of cluster %s is in progress, so current balance activity was rejected.", cluster.ID)
			continue
		}
		bc.usageQueue <- struct{}{}
		go bc.UsageBalance(ctx, userCred, session, cluster)
		log.Infof("%v", cluster)
	}
}

func (bc *SBalancerController) UsageBalance(ctx context.Context, userCred mcclient.TokenCredential, session *mcclient.ClientSession, short SClusterUsage) {
	log.Debugf("balance for cluster '%s', targetList length is : %d, src: %s", short.ID, len(short.TargetHostList), short.SrcHost.GetName())
	var (
		err     error
		success = true
	)
	setFail := func(sa *models.SClusterActivity, reason string) {
		success = false
		err = sa.SetFailed("", reason)
	}
	defer func() {
		if err != nil {
			log.Errorf("Usage balance for cluster '%s': %s", short.ID, err.Error())
		}
		bc.Finish(short.ID, success)
		<-bc.usageQueue
		log.Infof("Usage balance for cluster '%s' finished", short.ID)
	}()
	log.Debugf("fetch the latest data")
	// fetch the latest data
	model, err := models.ClusterManager.FetchById(short.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			err = nil
		}
		return
	}
	cluster := model.(*models.SCluster)
	if !bc.PreBalance(cluster, userCred) {
		success = true
		return
	}

	log.Debugf("insert ba")
	candidates, targetHost, err := bc.findUsageFitCandidates(session, short)
	if err != nil {
		log.Errorln(err)
		return
	}

	clusterActivity, err := models.ClusterActivityManager.CreateClusterActivity(
		ctx,
		cluster.Id,
		"",
		STRATEGY_USAGE,
		compute.SA_STATUS_EXEC,
	)
	if err != nil {
		return
	}

	// userCred是管理员，ownerId是拥有者
	//ownerId := cluster.GetOwnerId()
	num := len(candidates)
	succeedInstances, err := bc.MigrateInstances(ctx, userCred, candidates, short.SrcHost.memHost, targetHost.SHost, STRATEGY_USAGE, clusterActivity)
	switch len(succeedInstances) {
	case 0:
		setFail(clusterActivity, fmt.Sprintf("All instances migrate failed: %s", err.Error()))
	case num:
		var action bytes.Buffer
		action.WriteString("Instances ")
		for _, instance := range succeedInstances {
			action.WriteString(fmt.Sprintf("'%s', ", instance.Name))
		}
		action.Truncate(action.Len() - 2)
		action.WriteString(" are migrated")
		err = clusterActivity.SetResult(action.String(), compute.SA_STATUS_SUCCEED, "", num)
	default:
		var action bytes.Buffer
		action.WriteString("Instances ")
		for _, instance := range succeedInstances {
			action.WriteString(fmt.Sprintf("'%s', ", instance.Name))
		}
		action.Truncate(action.Len() - 2)
		action.WriteString(" are migrated")
		err = clusterActivity.SetResult(action.String(), compute.SA_STATUS_PART_SUCCEED, fmt.Sprintf("Some instances migrate failed: %s", err.Error()), len(succeedInstances))
	}
	return
}

type sortableUsedPercentCandidate []usageCandidate

func (s sortableUsedPercentCandidate) Len() int      { return len(s) }
func (s sortableUsedPercentCandidate) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortableUsedPercentCandidate) Less(i, j int) bool {
	return s[i].GetUsedPercentScore() > s[j].GetUsedPercentScore()
}

type sortableUsageActiveCandidate []usageCandidate

func (s sortableUsageActiveCandidate) Len() int      { return len(s) }
func (s sortableUsageActiveCandidate) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortableUsageActiveCandidate) Less(i, j int) bool {
	return s[i].GetUsageActiveScore() > s[j].GetUsageActiveScore()
}

func (bc *SBalancerController) findUsageFitCandidates(session *mcclient.ClientSession, cluster SClusterUsage) ([]*memCandidate, *memHost, error) {
	if len(cluster.TargetHostList) == 0 {
		return nil, nil, errors.Errorf("Not target host found")
	}
	retCandidates := make([]*memCandidate, 0)
	candidates := make([]usageCandidate, 0)
	srcHost := cluster.SrcHost
	guests, err := srcHost.GetGuests()
	if err != nil {
		log.Errorf("get guests for host %s err %v", srcHost.GetName(), err)
		return nil, nil, errors.Errorf("Not target host found")
	}
	for i := range guests {
		if guests[i].GetStatus() != computeapi.VM_RUNNING {
			log.Debugf("ignore guest %s cause status is %s", guests[i].GetName(), guests[i].GetStatus())
			continue
		}
		can, err := newUsageCandidate(session, &guests[i], srcHost)
		if err != nil {
			log.Debugf("new guest %s candidates err %v", guests[i].GetName(), err)
			continue
		}
		candidates = append(candidates, *can)
	}
	if cluster.Eval == "mem" {
		sort.Sort(sortableUsedPercentCandidate(candidates))
	} else {
		sort.Sort(sortableUsageActiveCandidate(candidates))
	}

	targetHosts := cluster.TargetHostList
	for i := range candidates {
		for _, targetHost := range targetHosts {
			targetGuests, err := targetHost.GetGuests()
			if err != nil {
				log.Errorf("get guests for host %s err: %v", targetHost.GetName(), err)
				continue
			}
			targetGuestMap := make(map[string]string)
			for _, guest := range targetGuests {
				targetGuestMap[guest.Id] = guest.Status
			}
			targetUsageHost := newTargetUsageHost(targetHost)
			err = targetUsageHost.IsFitCandidate(targetGuestMap, &candidates[i])
			if err == nil {
				retCandidates = append(retCandidates, candidates[i].memCandidate)
				return retCandidates, targetHost.memHost, nil
			}
		}
	}

	return nil, nil, errors.Errorf("Not fit candidates found")
}

type SClusterUsage struct {
	ID             string
	Eval           string
	TargetHostList []*usageHost
	SrcHost        *usageHost
}

func (bc *SBalancerController) UsageClustersNeedBalance(session *mcclient.ClientSession) ([]SClusterUsage, error) {
	rows, err := bc.usageSql.Rows()
	if err != nil {
		return nil, errors.Wrap(err, "execute usage sql error")
	}
	defer rows.Close()
	clusters := make([]SClusterUsage, 0, 10)
	for rows.Next() {
		cluster := compute.SCluster{}
		err := bc.usageSql.Row2Struct(rows, &cluster)
		if err != nil {
			return nil, errors.Wrap(err, "sqlchemy.SQuery.Row2Struct error")
		}
		//hostclusters := models.HostclusterManager.Query().SubQuery()
		//scopeQuery := hostclusters.Query(hostclusters.Field("host_id")).Equals("cluster_id", cluster.Id).SubQuery()
		hostClusterSql := models.HostclusterManager.Query("host_id").Equals("cluster_id", cluster.Id)
		count := hostClusterSql.Count()
		if count <= 1 {
			//asc.readyRecord[cluster.Id] = 0
			continue
		}
		hostIdRows, err := hostClusterSql.Rows()
		if err != nil {
			log.Errorf("query host sql error %v", err)
			continue
		}
		//var srcHost *models.SHost
		//var targetHost *models.SHost
		hosts := make([]models.SHost, 0)
		for hostIdRows.Next() {
			var hostId string
			err := hostIdRows.Scan(&hostId)
			if err != nil {
				log.Errorf("sqlchemy.SQuery.Scan error %v", err)
				continue
			}
			hostRows, err := models.HostManager.Query().Equals("id", hostId).Rows()
			if err != nil {
				log.Errorf("query host sql error %v", err)
				continue
			}
			for hostRows.Next() {
				host := models.SHost{}
				err = models.HostManager.Query().Equals("id", hostId).Row2Struct(hostRows, &host)
				if err != nil {
					log.Errorf("sqlchemy.SQuery.Scan error %v", err)
					continue
				}
				// 判定host是否是启用以及维护状态
				if host.IsMaintenance || !host.Enabled.Bool() {
					continue
				}
				hosts = append(hosts, host)
			}
		}
		targetHosts, srcHost, eval, err := bc.GetClusterUsageBalanceHost(hosts, session)
		if err != nil {
			log.Errorf("GetClusterUsageBalanceHost for cluster %s error: %v", cluster.Id, err)
			//asc.readyRecord[cluster.Id] = 0
			continue
		}
		clusterUsage := SClusterUsage{
			ID:             cluster.Id,
			Eval:           eval,
			SrcHost:        srcHost,
			TargetHostList: targetHosts,
		}

		clusters = append(clusters, clusterUsage)
	}
	return clusters, nil
}

func (bc *SBalancerController) GetClusterUsageBalanceHost(hosts []models.SHost, session *mcclient.ClientSession) ([]*usageHost, *usageHost, string, error) {
	//mem 排序确认
	memParams := &TsdbQuery{
		Database:    monitor.METRIC_DATABASE_TELE,
		Measurement: "mem",
		Fields:      []string{"used_percent"},
	}
	cpuParams := &TsdbQuery{
		Database:    monitor.METRIC_DATABASE_TELE,
		Measurement: "cpu",
		Fields:      []string{"usage_active"},
	}
	usageHosts := make([]usageHost, 0)
	for i := range hosts {
		memMetrics, err := InfluxdbQuery(session, "host_id", hosts[i].GetId(), memParams)
		if err != nil {
			log.Errorf("InfluxdbQuery host %q(%q) mem err, %v", hosts[i].GetName(), hosts[i].GetId(), err)
			continue
		}
		memUsage := 0.00
		if memMetrics != nil {
			metric := memMetrics.Get(hosts[i].GetId())
			if metric != nil {
				memUsage = metric.Values["used_percent"]
			}
		} else {
			log.Warningf("InfluxdbQuery host %q(%q) mem metric is nil", hosts[i].GetName(), hosts[i].GetId())
			continue
		}

		cpuMetrics, err := InfluxdbQuery(session, "host_id", hosts[i].GetId(), cpuParams)
		if err != nil {
			log.Errorf("InfluxdbQuery host %q(%q) cpu err, %v", hosts[i].GetName(), hosts[i].GetId(), err)
			continue
		}

		cpuUsage := 0.00
		if cpuMetrics != nil {
			metric := cpuMetrics.Get(hosts[i].GetId())
			if metric != nil {
				cpuUsage = metric.Values["usage_active"]
			}
		} else {
			log.Warningf("InfluxdbQuery host %q(%q) cpu metric is nil", hosts[i].GetName(), hosts[i].GetId())
			continue
		}
		usageHost := newUsageHost(&hosts[i], memUsage, cpuUsage)
		usageHosts = append(usageHosts, *usageHost)
	}
	if len(usageHosts) <= 1 {
		return nil, nil, "", fmt.Errorf("no need balance")
	}

	targetHosts, srcHost, err := bc.getMemBalanceHost(usageHosts)
	if err != nil {
		targetHosts, srcHost, err1 := bc.getCpuBalanceHost(usageHosts)
		if err1 != nil {
			return nil, nil, "", fmt.Errorf("no need balance")
		}
		return targetHosts, srcHost, "cpu", nil
	}
	//return nil, nil, fmt.Errorf("never happened")
	return targetHosts, srcHost, "mem", nil
}

func (bc *SBalancerController) getMemBalanceHost(usageHosts []usageHost) (targetHosts []*usageHost, srcHost *usageHost, err error) {
	sort.Sort(sortableUsedPercentHost(usageHosts))
	if usageHosts[0].GetUsedPercent() >= USAGE_THRESHOLD {
		srcHost = &usageHosts[0]
	} else {
		return nil, nil, fmt.Errorf("no mem usage host need balance")
	}
	//倒序排列 -->> 正序排列
	for i := len(usageHosts) - 1; i > 0; i-- {
		if usageHosts[i].GetUsedPercent() < USAGE_THRESHOLD {
			targetHosts = append(targetHosts, &usageHosts[i])
		}
	}
	if len(targetHosts) == 0 {
		return nil, nil, fmt.Errorf("no target mem usage host for balance")
	}
	return
}

func (bc *SBalancerController) getCpuBalanceHost(usageHosts []usageHost) (targetHosts []*usageHost, srcHost *usageHost, err error) {
	sort.Sort(sortableUsageActiveHost(usageHosts))
	if usageHosts[0].GetUsageActive() >= USAGE_THRESHOLD {
		srcHost = &usageHosts[0]
	} else {
		return nil, nil, fmt.Errorf("no cpu usage host need balance")
	}
	//倒序排列 -->> 正序排列
	for i := len(usageHosts) - 1; i > 0; i-- {
		//for i := 1; i < len(usageHosts); i++ {
		if usageHosts[i].GetUsageActive() < USAGE_THRESHOLD {
			targetHosts = append(targetHosts, &usageHosts[i])
		}
	}
	if len(targetHosts) == 0 {
		return nil, nil, fmt.Errorf("no target cpu usage host for balance")
	}
	return
}
