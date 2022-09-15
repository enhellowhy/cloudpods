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
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/compute/models"
	"yunion.io/x/pkg/errors"
)

//func (m *memCondition) GetSourceThresholdDelta(threshold float64, host IHost) float64 {
//	// mem.used
//	return host.GetCurrent() - threshold
//}
//
// memCandidate implements ICandidate
type memCandidate struct {
	*models.SGuest

	memScore float64
	cpuScore float64
}

func newMemCandidate(guest *models.SGuest, host *memHost) *memCandidate {
	memRate := float64(guest.VmemSize) * 100.0 / (float64(host.MemSize-host.MemReserved) * float64(host.MemCmtbound))
	cpuRate := float64(guest.VcpuCount) * 100.0 / (float64(host.CpuCount) * float64(host.CpuCmtbound))
	return &memCandidate{
		SGuest:   guest,
		memScore: memRate,
		cpuScore: cpuRate,
	}
}

func (m *memCandidate) GetScore() float64 {
	return m.memScore
}

func (m *memCandidate) GetCpuScore() float64 {
	return m.cpuScore
}

type memHost struct {
	*models.SHost
	memPercent float64
	cpuPercent float64
}

func newMemHost(host *models.SHost) *memHost {
	return &memHost{
		host,
		float64(host.GetRunningGuestMemorySize()) * 100.0 / (float64(host.MemSize-host.MemReserved) * float64(host.MemCmtbound)),
		float64(host.GetRunningGuestCpuCount()) * 100.0 / (float64(host.CpuCount) * float64(host.CpuCmtbound)),
	}
}

func (ts *memHost) SetCurrent(val float64) *memHost {
	ts.memPercent = val
	return ts
}

func (ts *memHost) GetCurrent() float64 {
	return ts.memPercent
}

func (ts *memHost) SetCpuCurrent(val float64) *memHost {
	ts.cpuPercent = val
	return ts
}

func (ts *memHost) GetCpuCurrent() float64 {
	return ts.cpuPercent
}

func (ts *memHost) Compare(oh *memHost) bool {
	return ts.GetCurrent() < oh.GetCurrent()
}

func (mh *memHost) Migrated(c *memCandidate) *memHost {
	mh.SetCurrent(mh.GetCurrent() - c.GetScore())       //同一台宿主机，所以不需要变更分数
	mh.SetCpuCurrent(mh.GetCpuCurrent() - c.GetScore()) //同一台宿主机，所以不需要变更分数
	return mh
}

func (m *memHost) IsFitTarget(t *targetMemHost, targetGuestMap map[string]string, c *memCandidate) error {
	guestTargetMemRate := float64(c.VmemSize) * 100.0 / (float64(t.MemSize-t.MemReserved) * float64(t.MemCmtbound))
	guestTargetCpuRate := float64(c.VcpuCount) * 100.0 / (float64(t.CpuCount) * float64(t.CpuCmtbound))
	//guestSourceMemRate := float64(c.VmemSize) * 100.0 / (float64(m.MemSize) * float64(m.MemCmtbound))
	if !isFitAntiGroup(targetGuestMap, c) {
		return errors.Errorf("guest:%s is not fit anti group for target host %s", c.GetName(), t.GetName())
	}
	if t.GetCurrent()+guestTargetMemRate < m.GetCurrent()-c.GetScore() && (t.GetCpuCurrent()+guestTargetCpuRate < 100.0) ||
		(t.GetCurrent()+guestTargetMemRate-m.GetCurrent()+c.GetScore() <= 20.0) && (t.GetCpuCurrent()+guestTargetCpuRate < 100.0) {
		return nil
	}
	return errors.Errorf("targetHost:%s:current(%f) + guest:%s:mem(%f) > srcHost:%s:current(%f) - guest:%s:mem(%f)", t.GetName(), t.GetCurrent(), c.GetName(), guestTargetMemRate, m.GetName(), m.GetCurrent(), c.GetName(), c.GetScore())
}

// ScalingGroupNeedScale will fetch all ScalingGroup need to scale
func isFitAntiGroup(targetGuestMap map[string]string, c *memCandidate) bool {
	groupGuestSql := models.GroupguestManager.Query("group_id").Equals("guest_id", c.Id)
	count := groupGuestSql.Count()
	if count == 0 {
		return true
	}
	if count > 1 {
		//asc.readyRecord[cluster.Id] = 0
		log.Warningf("the guest %s is in more than one anti group", c.GetName())
		return false
	}
	groupIdRows, err := groupGuestSql.Rows()
	if err != nil {
		log.Errorf("query group guest sql error %v", err)
		return false
	}
	defer groupIdRows.Close()
	antiCount := 0
	for groupIdRows.Next() {
		var groupId string
		err := groupIdRows.Scan(&groupId)
		if err != nil {
			log.Errorf("sqlchemy.SQuery.Scan error %v", err)
			return false
			//continue
		}
		groupRows, err := models.GroupManager.Query().Equals("id", groupId).Rows()
		if err != nil {
			log.Errorf("query group sql error %v", err)
			return false
			//continue
		}
		for groupRows.Next() {
			group := models.SGroup{}
			err = models.GroupManager.Query().Equals("id", groupId).Row2Struct(groupRows, &group)
			if err != nil {
				log.Errorf("sqlchemy.SQuery.Scan error %v", err)
				return false
				//continue
			}
			// 判定group是否是启用状态
			//if !group.Enabled.Bool() {
			//	return true
			//}
			guestIdRows, err := models.GroupguestManager.Query("guest_id").Equals("group_id", groupId).Rows()
			if err != nil {
				log.Errorf("query guest sql error %v", err)
				return false
			}
			for guestIdRows.Next() {
				var guestId string
				err := guestIdRows.Scan(&guestId)
				if err != nil {
					log.Errorf("sqlchemy.SQuery.Scan error %v", err)
					return false
					//continue
				}
				if _, ok := targetGuestMap[guestId]; ok {
					antiCount++
				}
			}
			if antiCount >= group.Granularity {
				return false
			} else {
				return true
			}
		}
	}
	return false
}

type targetMemHost struct {
	*memHost
}

func newTargetMemHost(host *models.SHost) *targetMemHost {
	ts := &targetMemHost{
		newMemHost(host),
	}
	return ts
}

func (th *targetMemHost) Selected(c *memCandidate) *targetMemHost {
	guestTargetMemRate := float64(c.VmemSize) * 100.0 / (float64(th.MemSize-th.MemReserved) * float64(th.MemCmtbound))
	guestTargetCpuRate := float64(c.VcpuCount) * 100.0 / (float64(th.CpuCount) * float64(th.CpuCmtbound))
	th.SetCurrent(th.GetCurrent() + guestTargetMemRate)
	th.SetCpuCurrent(th.GetCpuCurrent() + guestTargetCpuRate)
	return th
}
