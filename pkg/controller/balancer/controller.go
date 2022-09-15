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
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/tristate"
	"yunion.io/x/pkg/util/sets"
	"yunion.io/x/sqlchemy"

	"yunion.io/x/onecloud/pkg/apis/compute"
	computeapi "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/cronman"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/compute/models"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
	mcoptions "yunion.io/x/onecloud/pkg/mcclient/options"
	"yunion.io/x/onecloud/pkg/util/logclient"
	"yunion.io/x/onecloud/pkg/util/nopanic"
)

const (
	STRATEGY_USAGE    = "usage"
	STRATEGY_ASSIGNED = "assigned"
)

type SBalancerController struct {
	//options      options.SASControllerOptions
	//clusterQueue chan struct{}
	assignedQueue chan struct{}
	usageQueue    chan struct{}

	clusterSet *SLockedSet

	assignedSql *sqlchemy.SQuery
	usageSql    *sqlchemy.SQuery
	// record the consecutive failures of scaling group's scale
	failRecord  map[string]int
	readyRecord map[string]int
}

type SScalingInfo struct {
	ScalingGroup *models.SScalingGroup
	Total        int
}

type SLockedSet struct {
	set  sets.String
	lock sync.Mutex
}

func (set *SLockedSet) Has(s string) bool {
	return set.set.Has(s)
}

func (set *SLockedSet) CheckAndInsert(s string) bool {
	set.lock.Lock()
	defer set.lock.Unlock()
	if set.set.Has(s) {
		return false
	}
	set.set.Insert(s)
	return true
}

func (set *SLockedSet) Delete(s string) {
	set.lock.Lock()
	defer set.lock.Unlock()
	set.set.Delete(s)
}

var BalancerController = new(SBalancerController)

func (bc *SBalancerController) Init(checkBalanceInterval int, cronm *cronman.SCronJobManager) {
	bc.assignedQueue = make(chan struct{}, 10)
	bc.usageQueue = make(chan struct{}, 10)
	bc.clusterSet = &SLockedSet{set: sets.NewString()}
	bc.failRecord = make(map[string]int)
	bc.readyRecord = make(map[string]int)

	// init scalingSql
	assignedSql := models.ClusterManager.Query().Equals("drs_enabled", true).Equals("strategy", "assigned")
	usageSql := models.ClusterManager.Query().Equals("drs_enabled", true).Equals("strategy", "usage")
	bc.assignedSql = assignedSql
	bc.usageSql = usageSql

	log.Infof("checkBalanceInterval is %ds", checkBalanceInterval)
	cronm.AddJobAtIntervalsWithStartRun("CheckBalance", time.Duration(checkBalanceInterval)*time.Second, bc.CheckAssignedBalance, true)
	cronm.AddJobAtIntervalsWithStartRun("CheckUsageBalance", time.Duration(checkBalanceInterval)*time.Second, bc.CheckUsageBalance, false)

	// check all cluster activity
	nopanic.Run(func() {
		log.Infof("check and update cluster activities...")
		cas := make([]models.SClusterActivity, 0, 10)
		q := models.ClusterActivityManager.Query().Equals("status", compute.SA_STATUS_EXEC)
		err := db.FetchModelObjects(models.ClusterActivityManager, q, &cas)
		if err != nil {
			log.Errorf("unable to check and update cluster activities")
			return
		}
		for i := range cas {
			cas[i].SetFailed("", "As the service restarts, the status becomes unknown")
		}
		log.Infof("check and update clusteractivities complete")
	})
}

func (bc *SBalancerController) IsReady(clusterId string, userCred mcclient.TokenCredential) bool {
	consecution := 3
	//disableReason := fmt.Sprintf("The number of consecutive failures of balancing exceeds %d times", maxFailures)
	//bc.readyRecord[cluster.GetId()]++
	times := bc.readyRecord[clusterId]
	if times >= consecution {
		return true
	}
	return false
}

func (asc *SBalancerController) PreBalance(cluster *models.SCluster, userCred mcclient.TokenCredential) bool {
	maxFailures := 3
	disableReason := fmt.Sprintf("The number of consecutive failures of balancing exceeds %d times", maxFailures)
	times := asc.failRecord[cluster.GetId()]
	if times >= maxFailures {
		_, err := db.Update(cluster, func() error {
			cluster.DrsEnabled = tristate.False
			return nil
		})
		if err != nil {
			return false
		}
		logclient.AddSimpleActionLog(cluster, logclient.ACT_DISABLE, disableReason, userCred, true)
		return false
	}
	return true
}

func (asc *SBalancerController) Finish(clusterId string, success bool) {
	asc.clusterSet.Delete(clusterId)
	if success {
		asc.failRecord[clusterId] = 0
		//asc.readyRecord[clusterId]--
		return
	}
	asc.failRecord[clusterId]++
	//asc.readyRecord[clusterId] = 0
}

// SScalingGroupShort wrap the ScalingGroup's ID and DesireInstanceNumber with field 'total' which means the total
// guests number in this ScalingGroup
type SClusterShort struct {
	ID          string
	SrcHostList []*models.SHost
	TargetHost  *models.SHost
}

func (bc *SBalancerController) CheckAssignedBalance(ctx context.Context, userCred mcclient.TokenCredential, isStart bool) {
	clusters, err := bc.AssignedClustersNeedBalance()
	if err != nil {
		log.Errorf("asc.ScalingGroupNeedScale: %s", err.Error())
		return
	}
	if len(clusters) == 0 {
		log.Debugln("No assigned cluster to balance")
		return
	}
	keys := make([]string, 0)
	for k := range bc.readyRecord {
		find := false
		for _, cluster := range clusters {
			if k == cluster.ID {
				find = true
				break
			}
		}
		if !find {
			keys = append(keys, k)
		}
	}
	for _, key := range keys {
		delete(bc.readyRecord, key)
	}
	for _, cluster := range clusters {
		if !bc.IsReady(cluster.ID, userCred) {
			continue
		}
		insert := bc.clusterSet.CheckAndInsert(cluster.ID)
		if !insert {
			log.Warningf("A balance activity of cluster %s is in progress, so current balance activity was rejected.", cluster.ID)
			continue
		}
		bc.assignedQueue <- struct{}{}
		go bc.Balance(ctx, userCred, cluster)
		log.Infof("%v", cluster)
	}
	// log.Debugf("This cronJob about CheckScale finished")
}

func (bc *SBalancerController) Balance(ctx context.Context, userCred mcclient.TokenCredential, short SClusterShort) {
	log.Debugf("balance for cluster '%s', srcList length is : %d, target: %s", short.ID, len(short.SrcHostList), short.TargetHost.GetName())
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
			log.Errorf("Balance for cluster '%s': %s", short.ID, err.Error())
		}
		bc.Finish(short.ID, success)
		<-bc.assignedQueue
		log.Infof("Balance for cluster '%s' finished", short.ID)
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
	candidates, sourceHost, err := bc.findCandidates(short)
	if err != nil {
		log.Errorln(err)
		return
	}

	clusterActivity, err := models.ClusterActivityManager.CreateClusterActivity(
		ctx,
		cluster.Id,
		"",
		STRATEGY_ASSIGNED,
		compute.SA_STATUS_EXEC,
	)
	if err != nil {
		return
	}

	// userCred是管理员，ownerId是拥有者
	//ownerId := cluster.GetOwnerId()
	num := len(candidates)
	succeedInstances, err := bc.MigrateInstances(ctx, userCred, candidates, sourceHost, short.TargetHost, STRATEGY_ASSIGNED, clusterActivity)
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

func (bc *SBalancerController) doMigrate(ctx context.Context, session *mcclient.ClientSession, userCred mcclient.TokenCredential, activity *models.SClusterActivity, candidates []*memCandidate, sourceHost *memHost, targetHost *models.SHost, startTime time.Time, strategy string) ([]string, []SInstance) {
	failedList := make([]string, 0)
	succeedList := make([]SInstance, 0)

	for i := range candidates {
		guest := candidates[i]
		trueObj := true
		//input := &computeapi.GuestLiveMigrateInput{
		//	PreferHost:   targetHost.GetId(),
		//	SkipCpuCheck: &trueObj,
		//}
		//obj, err := guest.PerformLiveMigrate(ctx, userCred, nil, input)
		//if err != nil {
		//	//迁移记录
		//	noteStr := fmt.Sprintf("guest %s live migrate err: %s", guest.GetName(), err.Error())
		//	bc.drsRecords(session, activity, guest, sourceHost, targetHost, startTime, noteStr, false)
		//	failedList = append(failedList, noteStr)
		//	continue
		//}
		input := &mcoptions.ServerLiveMigrateOptions{
			ID:           guest.GetId(),
			PreferHost:   targetHost.GetId(),
			SkipCpuCheck: &trueObj,
			//SkipKernelCheck: &trueObj,
		}
		params, _ := input.Params()
		//if err != nil {
		//	return nil, errors.Wrapf(err, "live migrate input %#v", input)
		//}
		obj, err := modules.Servers.PerformAction(session, input.GetId(), "live-migrate", params)
		if err != nil {
			//迁移记录
			noteStr := fmt.Sprintf("guest %s live migrate err: %s", guest.GetName(), err.Error())
			bc.drsRecords(session, activity, guest, sourceHost, targetHost, startTime, noteStr, strategy, false)
			failedList = append(failedList, noteStr)
			continue
		}
		id, _ := obj.GetString("id")
		name, _ := obj.GetString("name")
		succeedList = append(succeedList, SInstance{id, name})
	}
	return failedList, succeedList
}

type sortableGuest []models.SGuest

func (s sortableGuest) Len() int      { return len(s) }
func (s sortableGuest) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortableGuest) Less(i, j int) bool {
	return s[i].VmemSize > s[j].VmemSize
}

func (bc *SBalancerController) drsRecords(session *mcclient.ClientSession, activity *models.SClusterActivity, guest *memCandidate, sourceHost *memHost, targetHost *models.SHost, startTime time.Time, noteStr, strategy string, success bool) {
	params := &compute.DrsRecordCreateInput{}
	params.StartTime = startTime
	params.ClusterId = activity.ClusterId
	//
	params.ActivityId = activity.Id
	params.GuestId = guest.Id
	params.GuestName = guest.Name
	params.FromHostId = sourceHost.Id
	params.FromHostName = sourceHost.Name
	params.ToHostId = targetHost.Id
	params.ToHostName = targetHost.Name
	params.Action = db.ACT_MIGRATE
	params.Notes = noteStr
	params.Strategy = strategy
	params.Success = success

	_, err := modules.DrsRecords.Create(session, jsonutils.Marshal(params))
	if err != nil {
		log.Errorf("DrsRecordManager.Create err: %v", err)
	}
}

func (bc *SBalancerController) findCandidates(short SClusterShort) ([]*memCandidate, *memHost, error) {
	//threshold := cond.GetThreshold()
	//delta := cond.GetSourceThresholdDelta(threshold, src.Host)
	//cds := sortCandidatesByThreshold(src.Candidates, threshold)
	targetHost := newTargetMemHost(short.TargetHost)
	return bc.findFitCandidates(short.SrcHostList, targetHost)
}

func (bc *SBalancerController) findFitCandidates(srcHostList []*models.SHost, targetHost *targetMemHost) ([]*memCandidate, *memHost, error) {
	if len(srcHostList) == 0 {
		return nil, nil, errors.Errorf("Not host found")
	}
	targetGuests, err := targetHost.GetGuests()
	if err != nil {
		log.Errorf("get guests for host %s err: %v", targetHost.GetName(), err)
		return nil, nil, errors.Errorf("Get target host guests err", err)
	}
	targetGuestMap := make(map[string]string)
	for _, guest := range targetGuests {
		targetGuestMap[guest.Id] = guest.Status
	}
	for i := range srcHostList {
		candidates := make([]*memCandidate, 0)
		s := srcHostList[i]
		mh := newMemHost(s)
		guests, err := mh.GetGuests()
		if err != nil {
			log.Errorf("get guests for host %s err %v", mh.GetName(), err)
			continue
		}
		sort.Sort(sortableGuest(guests))
		for j := range guests {
			mc := newMemCandidate(&guests[j], mh)
			if mc.GetStatus() != computeapi.VM_RUNNING {
				log.Debugf("ignore guest %s cause status is %s", mc.GetName(), mc.GetStatus())
				continue
			}
			err := mh.IsFitTarget(targetHost, targetGuestMap, mc)
			if err == nil {
				candidates = append(candidates, mc)
				mh = mh.Migrated(mc)
				targetHost = targetHost.Selected(mc)
				if mh.GetCurrent()-targetHost.GetCurrent() < 20.0 {
					return candidates, mh, nil
				}
			}
			if len(candidates) == 3 {
				return candidates, mh, nil
			}
		}
		if len(candidates) > 0 {
			return candidates, mh, nil
		}
	}

	return nil, nil, errors.Errorf("Not fit candidates found")
}

func (asc *SBalancerController) findSuitableInstance(sg *models.SScalingGroup, num int) ([]models.SGuest, error) {
	ggSubQ := models.ScalingGroupGuestManager.Query("guest_id").Equals("scaling_group_id", sg.Id).SubQuery()
	guestQ := models.GuestManager.Query().In("id", ggSubQ)
	switch sg.ShrinkPrinciple {
	case compute.SHRINK_EARLIEST_CREATION_FIRST:
		guestQ = guestQ.Asc("created_at").Limit(num)
	case compute.SHRINK_LATEST_CREATION_FIRST:
		guestQ = guestQ.Desc("created_at").Limit(num)
	}
	guests := make([]models.SGuest, 0, num)
	err := db.FetchModelObjects(models.GuestManager, guestQ, &guests)
	if err != nil {
		return nil, err
	}
	return guests, nil
}

type SInstance struct {
	ID   string
	Name string
}

func (asc *SBalancerController) MigrateInstances(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	candidates []*memCandidate,
	sourceHost *memHost,
	targetHost *models.SHost,
	strategy string,
	activity *models.SClusterActivity,
) ([]SInstance, error) {
	rand.Seed(time.Now().UnixNano())
	session := auth.GetSession(ctx, userCred, "", "")

	// fisrt stage: migrate request
	targetHostId := targetHost.GetId()
	startTime := time.Now().UTC()
	failedList, succeedList := asc.doMigrate(ctx, session, userCred, activity, candidates, sourceHost, targetHost, startTime, strategy)

	// second stage: joining scaling group
	// third stage: wait for migrate complete
	retChan := make(chan SMigrateRet, len(succeedList))

	guestIds := make([]string, len(succeedList))
	instanceMap := make(map[string]SInstance, len(succeedList))
	for i, instane := range succeedList {
		guestIds[i] = instane.ID
		instanceMap[instane.ID] = instane
	}
	// check all server's status
	var waitLimit, waitinterval time.Duration
	waitLimit = 30 * time.Minute
	waitinterval = 10 * time.Second

	go asc.checkAllServer(session, guestIds, retChan, waitLimit, waitinterval)

	// fourth stage: bind lb and db
	failRecord := &SFailRecord{
		recordList: failedList,
	}
	succeedInstances := make([]string, 0, len(succeedList))
	//workerLimit := make(chan struct{}, len(succeedList))
	for {
		ret, ok := <-retChan
		if !ok {
			break
		}
		var guest *memCandidate
		for i := range candidates {
			if candidates[i].Id == ret.Id {
				guest = candidates[i]
			}
		}
		if guest == nil {
			log.Errorf("The ret id is %s not in candidates, not impossible!!", ret.Id)
			continue
		}
		if ret.Status == compute.VM_RUNNING || ret.Status == compute.VM_READY {
			if ret.CurrentHostId != sourceHost.GetId() {
				if ret.CurrentHostId == targetHostId {
					asc.drsRecords(session, activity, guest, sourceHost, targetHost, startTime, "success", strategy, true)
					succeedInstances = append(succeedInstances, ret.Id)
				} else {
					log.Warningf("Server expected in target host %s, current %s", targetHostId, ret.CurrentHostId)
					//succeedInstances = append(succeedInstances, ret.Id)
					noteStr := fmt.Sprintf("Server expected in target host %s, current %s", targetHostId, ret.CurrentHostId)
					asc.drsRecords(session, activity, guest, sourceHost, targetHost, startTime, noteStr, strategy, false)
					failRecord.Append(noteStr)
				}
			} else {
				log.Errorf("Server %s still in source host %s", ret.Id, ret.CurrentHostId)
				noteStr := fmt.Sprintf("Server %s still in source host %s", ret.Id, ret.CurrentHostId)
				asc.drsRecords(session, activity, guest, sourceHost, targetHost, startTime, noteStr, strategy, false)
				failRecord.Append(noteStr)
			}
		} else {
			log.Errorf("Server %s fail status %s", ret.Id, ret.Status)
			noteStr := fmt.Sprintf("server fail status %s", ret.Status)
			asc.drsRecords(session, activity, guest, sourceHost, targetHost, startTime, noteStr, strategy, false)
			failRecord.Append(noteStr)
		}
		if strings.HasSuffix(ret.Status, "_fail") || strings.HasSuffix(ret.Status, "_failed") {
			if ret.Status == compute.VM_MIGRATE_FAILED {
				// try sync status to orignal
				if _, err := modules.Servers.PerformAction(session, ret.Id, "syncstatus", nil); err != nil {
					log.Errorf("Sync server %s migrate_failed status error: %v", ret.Id, err)
				}
			}
		}
		//workerLimit <- struct{}{}
		// bind ld and db
		//go func() {
		//	succeed := asc.actionAfterCreate(ctx, userCred, session, sg, ret, failRecord)
		//	log.Debugf("action after create instance '%s', succeed: %t", ret.Id, succeed)
		//	if succeed {
		//		succeedInstances = append(succeedInstances, ret.Id)
		//	}
		//	<-workerLimit
		//}()
	}
	log.Debugf("wait fo all migrate worker finish")
	// wait for all worker finish
	//log.Debugf("workerlimit cap: %d", cap(workerLimit))
	//for i := 0; i < cap(workerLimit); i++ {
	//	log.Debugf("no.%d insert worker limit", i)
	//	workerLimit <- struct{}{}
	//}

	instances := make([]SInstance, 0, len(succeedInstances))
	for _, id := range succeedInstances {
		instances = append(instances, instanceMap[id])
	}
	return instances, fmt.Errorf(failRecord.String())
}

type SMigrateRet struct {
	Id            string
	CurrentHostId string
	Status        string
}

func (asc *SBalancerController) checkAllServer(session *mcclient.ClientSession, guestIds []string, retChan chan SMigrateRet,
	waitLimit, waitInterval time.Duration) {
	guestIDSet := sets.NewString(guestIds...)
	timer := time.NewTimer(waitLimit)
	ticker := time.NewTicker(waitInterval)
	defer func() {
		close(retChan)
		ticker.Stop()
		timer.Stop()
		log.Debugf("finish all check jobs when migrating servers")
	}()
	log.Debugf("guestIds: %s", guestIds)
	for {
		select {
		default:
			for _, id := range guestIDSet.UnsortedList() {
				//ret, e := modules.Servers.GetSpecific(session, id, "status", nil)
				ret, e := modules.Servers.GetById(session, id, nil)
				if e != nil {
					log.Errorf("Servers GetDetails failed: %s", e)
					<-ticker.C
					continue
				}
				log.Debugf("ret from Get: %s", ret.String())
				status, _ := ret.GetString("status")
				curHostId, _ := ret.GetString("host_id")
				if status == compute.VM_RUNNING || status == compute.VM_READY || strings.HasSuffix(status, "fail") || strings.HasSuffix(status, "failed") {
					guestIDSet.Delete(id)
					retChan <- SMigrateRet{
						Id:            id,
						CurrentHostId: curHostId,
						Status:        status,
					}
				}
				log.Errorf("server status %q, continue watching", status)
			}
			if guestIDSet.Len() == 0 {
				return
			}
			<-ticker.C
		case <-timer.C:
			log.Errorf("some migrate check jobs for server timeout")
			for _, id := range guestIDSet.UnsortedList() {
				retChan <- SMigrateRet{
					Id:            id,
					CurrentHostId: "",
					Status:        "timeout",
				}
			}
			return
		}
	}
}

type SFailRecord struct {
	lock       sync.Mutex
	recordList []string
}

func (fr *SFailRecord) Append(record string) {
	fr.lock.Lock()
	defer fr.lock.Unlock()
	fr.recordList = append(fr.recordList, record)
}

func (fr *SFailRecord) String() string {
	return strings.Join(fr.recordList, "; ")
}

func (asc *SBalancerController) actionAfterCreate(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	session *mcclient.ClientSession,
	sg *models.SScalingGroup,
	ret SMigrateRet,
	failRecord *SFailRecord,
) (succeed bool) {
	log.Debugf("start to action After create")
	deleteParams := jsonutils.NewDict()
	deleteParams.Set("override_pending_delete", jsonutils.JSONTrue)
	updateParams := jsonutils.NewDict()
	updateParams.Set("disable_delete", jsonutils.JSONFalse)
	rollback := func(failedReason string) {
		failRecord.Append(failedReason)
		// get scalingguest
		sggs, err := models.ScalingGroupGuestManager.Fetch(sg.GetId(), ret.Id)
		if err != nil || len(sggs) == 0 {
			log.Errorf("ScalingGroupGuestManager.Fetch failed: %s", err.Error())
			return
		}
		// cancel delete project
		_, e := modules.Servers.Update(session, ret.Id, updateParams)
		if err != nil {
			sggs[0].SetGuestStatus(compute.SG_GUEST_STATUS_READY)
			log.Errorf("cancel delete project of instance '%s' failed: %s", ret.Id, e.Error())
			return
		}
		// delete corresponding instance
		_, e = modules.Servers.Delete(session, ret.Id, deleteParams)
		if e != nil {
			// delete failed
			sggs[0].SetGuestStatus(compute.SG_GUEST_STATUS_READY)
			log.Errorf("delete instance '%s' failed: %s", ret.Id, e.Error())
			return
		}
		sggs[0].Detach(ctx, userCred)
		return
	}
	if ret.Status != compute.VM_RUNNING {
		if ret.Status == "timeout" {
			rollback(fmt.Sprintf("the creation process for instance '%s' has timed out", ret.Id))
		} else {
			// fetch the reason
			var reason string
			params := jsonutils.NewDict()
			params.Add(jsonutils.NewString(ret.Id), "obj_id")
			params.Add(jsonutils.NewStringArray([]string{db.ACT_ALLOCATE_FAIL}), "action")
			events, err := modules.Logs.List(session, params)
			if err != nil {
				log.Errorf("Logs.List failed: %s", err.Error())
				reason = fmt.Sprintf("instance '%s' which status is '%s' create failed", ret.Id, ret.Status)
			} else {
				switch events.Total {
				case 1:
					reason, _ = events.Data[0].GetString("notes")
				case 0:
					log.Errorf("These is no opslog about action '%s' for instance '%s", db.ACT_ALLOCATE_FAIL, ret.Id)
					reason = fmt.Sprintf("instance '%s' which status is '%s' create failed", ret.Id, ret.Status)
				default:
					log.Debugf("These are more than one optlogs about action '%s' for instance '%s'", db.ACT_ALLOCATE_FAIL, ret.Id)
					reason, _ = events.Data[0].GetString("notes")
				}
			}
			rollback(reason)
		}
		return
	}
	// bind lb
	if len(sg.BackendGroupId) != 0 {
		params := jsonutils.NewDict()
		params.Set("backend", jsonutils.NewString(ret.Id))
		params.Set("backend_type", jsonutils.NewString("guest"))
		params.Set("port", jsonutils.NewInt(int64(sg.LoadbalancerBackendPort)))
		params.Set("weight", jsonutils.NewInt(int64(sg.LoadbalancerBackendWeight)))
		params.Set("backend_group", jsonutils.NewString(sg.BackendGroupId))
		_, err := modules.LoadbalancerBackends.Create(session, params)
		if err != nil {
			rollback(fmt.Sprintf("bind instance '%s' to loadbalancer backend gropu '%s' failed: %s", ret.Id, sg.BackendGroupId, err.Error()))
		}
	}
	// todo bind bd

	// fifth stage: join scaling group finished
	sggs, err := models.ScalingGroupGuestManager.Fetch(sg.GetId(), ret.Id)
	if err != nil || sggs == nil || len(sggs) == 0 {
		log.Errorf("ScalingGroupGuestManager.Fetch failed; ScalingGroup '%s', Guest '%s'", sg.Id, ret.Id)
		return
	}
	sggs[0].SetGuestStatus(compute.SG_GUEST_STATUS_READY)
	return true
}

func (asc *SBalancerController) randStringRunes(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz1234567890")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func (asc *SBalancerController) countPRAndRequests(num int) (int, int) {
	key, tmp := 5, 1
	countPR, requests := 1, num
	if requests >= key {
		countPR := tmp * key
		requests = num / countPR
		tmp += 1
	}
	return countPR, requests
}

type sortableHost []models.SHost

func (s sortableHost) Len() int      { return len(s) }
func (s sortableHost) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortableHost) Less(i, j int) bool {
	imemCommitRate := float64(s[i].GetRunningGuestMemorySize()) * 100.0 / (float64(s[i].MemSize-s[i].MemReserved) * float64(s[i].MemCmtbound))
	jmemCommitRate := float64(s[j].GetRunningGuestMemorySize()) * 100.0 / (float64(s[j].MemSize-s[j].MemReserved) * float64(s[j].MemCmtbound))

	return imemCommitRate < jmemCommitRate
}

type sortableUsedPercentHost []usageHost

func (s sortableUsedPercentHost) Len() int      { return len(s) }
func (s sortableUsedPercentHost) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortableUsedPercentHost) Less(i, j int) bool {
	a, b := s[i], s[j]
	return a.CompareUsedPercent(b)
}

type sortableUsageActiveHost []usageHost

func (s sortableUsageActiveHost) Len() int      { return len(s) }
func (s sortableUsageActiveHost) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortableUsageActiveHost) Less(i, j int) bool {
	a, b := s[i], s[j]
	return a.CompareUsageActive(b)
}

// ScalingGroupNeedScale will fetch all ScalingGroup need to scale
func (asc *SBalancerController) AssignedClustersNeedBalance() ([]SClusterShort, error) {
	rows, err := asc.assignedSql.Rows()
	if err != nil {
		return nil, errors.Wrap(err, "execute scaling sql error")
	}
	defer rows.Close()
	clusters := make([]SClusterShort, 0, 10)
	for rows.Next() {
		cluster := compute.SCluster{}
		err := asc.assignedSql.Row2Struct(rows, &cluster)
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
				if host.MemSize > 0 && host.MemCmtbound > 0 && host.CpuCmtbound > 0 && host.CpuCount > 0 {
					hosts = append(hosts, host)
				}
			}
		}
		sort.Sort(sortableHost(hosts))
		srcList, target, err := asc.GetClusterBalanceHost(hosts)
		if err != nil {
			log.Errorf("GetClusterBalanceHost for cluster %s error: %v", cluster.Id, err)
			//asc.readyRecord[cluster.Id] = 0
			continue
		}
		clusterShort := SClusterShort{
			ID:          cluster.Id,
			SrcHostList: srcList,
			TargetHost:  target,
		}

		clusters = append(clusters, clusterShort)
		asc.readyRecord[cluster.Id]++
	}
	return clusters, nil
}

func (bc *SBalancerController) GetClusterBalanceHost(hosts []models.SHost) ([]*models.SHost, *models.SHost, error) {
	if len(hosts) <= 1 {
		return nil, nil, fmt.Errorf("no need balance")
	}
	targetHost := hosts[0]
	targetMemCommitRate := float64(targetHost.GetRunningGuestMemorySize()) * 100.0 / (float64(targetHost.MemSize-targetHost.MemReserved) * float64(targetHost.MemCmtbound))
	targetCpuCommitRate := float64(targetHost.GetRunningGuestCpuCount()) * 100.0 / (float64(targetHost.CpuCount) * float64(targetHost.CpuCmtbound))
	srcHostList := make([]*models.SHost, 0)
	for i := len(hosts) - 1; i > 0; i-- {
		memCommitRate := float64(hosts[i].GetRunningGuestMemorySize()) * 100.0 / (float64(hosts[i].MemSize-hosts[i].MemReserved) * float64(hosts[i].MemCmtbound))
		cpuCommitRate := float64(hosts[i].GetRunningGuestCpuCount()) * 100.0 / (float64(hosts[i].CpuCount) * float64(hosts[i].CpuCmtbound))
		memDiff := memCommitRate - targetMemCommitRate
		cpuDiff := cpuCommitRate - targetCpuCommitRate
		if memDiff >= 20.0 {
			if cpuDiff >= 20.0 {
				srcHostList = append(srcHostList, &hosts[i])
			}
			//} else {
			//	return bc.GetClusterBalanceHost(cluster, hosts[i+1:])
			//}
		} else {
			break
		}
	}
	if len(srcHostList) == 0 {
		return nil, nil, fmt.Errorf("no need balance")
	}
	return srcHostList, &targetHost, nil
	//return nil, nil, fmt.Errorf("never happened")
}
