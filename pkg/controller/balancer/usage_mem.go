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
	"yunion.io/x/onecloud/pkg/compute/models"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud/pkg/apis/monitor"
)

// usageCandidate implements ICandidate
type usageCandidate struct {
	*memCandidate
	srtHost *usageHost

	usedPercentScore float64 //mem
	usageActiveScore float64 //cpu
}

func newUsageCandidate(session *mcclient.ClientSession, guest *models.SGuest, host *usageHost) (*usageCandidate, error) {
	memCandidate := newMemCandidate(guest, host.memHost)
	// fetch metric from influxdb
	memMetrics, err := InfluxdbQuery(session, "vm_id", guest.Id, &TsdbQuery{
		Database:    monitor.METRIC_DATABASE_TELE,
		Measurement: "agent_mem",
		Fields:      []string{"used_percent"},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "InfluxdbQuery guest mem %q(%q)", guest.GetName(), guest.GetId())
	}

	memUsage := 0.00
	if memMetrics != nil {
		metric := memMetrics.Get(guest.GetId())
		if metric != nil {
			memUsage = metric.Values["used_percent"]
		}
	}

	usedPercentScore := memUsage * (float64(guest.VmemSize) / float64(host.GetMemSize()))

	cpuMetrics, err := InfluxdbQuery(session, "vm_id", guest.Id, &TsdbQuery{
		Database:    monitor.METRIC_DATABASE_TELE,
		Measurement: "agent_cpu",
		Fields:      []string{"usage_active"},
	})
	if err != nil {
		return nil, errors.Wrapf(err, "InfluxdbQuery guest cpu %q(%q)", guest.GetName(), guest.GetId())
	}

	cpuUsage := 0.00
	if cpuMetrics != nil {
		metric := cpuMetrics.Get(guest.GetId())
		if metric != nil {
			cpuUsage = metric.Values["usage_active"]
		}
	}

	usageActiveScore := cpuUsage * (float64(guest.VcpuCount) / float64(host.GetCpuCount()))
	return &usageCandidate{
		memCandidate:     memCandidate,
		srtHost:          host,
		usedPercentScore: usedPercentScore,
		usageActiveScore: usageActiveScore,
	}, nil
}

func (c *usageCandidate) GetUsedPercentScore() float64 {
	return c.usedPercentScore
}

func (c *usageCandidate) GetUsageActiveScore() float64 {
	return c.usageActiveScore
}

func (c *usageCandidate) GetTargetUsedPercentScore(host *targetUsageHost) float64 {
	return c.GetUsedPercentScore() * float64(c.srtHost.GetMemSize()) / float64(host.GetMemSize())
}

func (c *usageCandidate) GetTargetUsageActiveScore(host *targetUsageHost) float64 {
	return c.GetUsageActiveScore() * float64(c.srtHost.GetCpuCount()) / float64(host.GetCpuCount())
}

//func (ma *memAvailable) GetTsdbQuery() *TsdbQuery {
//	return &TsdbQuery{
//		Database:    monitor.METRIC_DATABASE_TELE,
//		Measurement: "mem",
//		Fields:      []string{"total", "free", "available", "used_percent"},
//	}
//}

type usageHost struct {
	*memHost
	usedPercent float64 //mem
	usageActive float64 //cpu
}

func newUsageHost(host *models.SHost, usedPercent, usageActive float64) *usageHost {
	h := newMemHost(host)
	return &usageHost{
		memHost:     h,
		usedPercent: usedPercent,
		usageActive: usageActive,
	}
}

func (uh *usageHost) GetUsedPercent() float64 {
	return uh.usedPercent
}

func (uh *usageHost) SetUsedPercent(val float64) *usageHost {
	uh.usedPercent = val
	return uh
}

func (uh *usageHost) GetUsageActive() float64 {
	return uh.usageActive
}

func (uh *usageHost) SetUsageActive(val float64) *usageHost {
	uh.usageActive = val
	return uh
}

func (uh *usageHost) CompareUsedPercent(oh usageHost) bool {
	return uh.GetUsedPercent() > oh.GetUsedPercent()
}

func (uh *usageHost) CompareUsageActive(oh usageHost) bool {
	return uh.GetUsageActive() > oh.GetUsageActive()
}

type targetUsageHost struct {
	*usageHost
}

func newTargetUsageHost(host *usageHost) *targetUsageHost {
	th := &targetUsageHost{
		usageHost: host,
	}
	return th
}

func (th *targetUsageHost) Selected(c *usageCandidate) *targetUsageHost {
	guestTargetMemRate := float64(c.VmemSize) * 100.0 / (float64(th.GetMemSize()) * float64(th.MemCmtbound))
	guestTargetCpuRate := float64(c.VcpuCount) * 100.0 / (float64(th.GetCpuCount()) * float64(th.CpuCmtbound))
	th.SetCurrent(th.GetCurrent() + guestTargetMemRate)
	th.SetCpuCurrent(th.GetCpuCurrent() + guestTargetCpuRate)

	th.SetUsedPercent(th.GetUsedPercent() + c.GetTargetUsedPercentScore(th))
	th.SetUsageActive(th.GetUsageActive() + c.GetTargetUsageActiveScore(th))
	return th
}

func (th *targetUsageHost) IsFitCandidate(targetGuestMap map[string]string, c *usageCandidate) error {
	if !isFitAntiGroup(targetGuestMap, c.memCandidate) {
		return errors.Errorf("guest:%s is not fit anti group for target host %s", c.GetName(), th.GetName())
	}
	th.Selected(c)
	if th.GetUsedPercent() < USAGE_THRESHOLD && th.GetUsageActive() < USAGE_THRESHOLD &&
		th.GetCurrent() < 100.0 && th.GetCpuCurrent() < 100.0 {
		return nil
	}
	return errors.Errorf("after select, host:%s:UsedPercent(%f),UsageActive(%f),MemPercent(%f),CpuPercent(%f)", th.GetName(), th.GetUsedPercent(), th.GetUsageActive(), th.GetCurrent(), th.GetCpuCurrent())
}
