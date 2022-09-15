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

package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	api "yunion.io/x/onecloud/pkg/apis/billing"
	"yunion.io/x/onecloud/pkg/apis/compute"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud/pkg/apis/monitor"
	"yunion.io/x/onecloud/pkg/hostman/hostinfo/hostconsts"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/mcclient/modulebase"
	mc_mds "yunion.io/x/onecloud/pkg/mcclient/modules"
)

var (
	resourceSyncMap map[string]IResourceSync
	//guestResourceSync   IResourceSync
	//hostResourceSync    IResourceSync
)

func RegistryResourceSync(sync IResourceSync) error {
	if resourceSyncMap == nil {
		resourceSyncMap = make(map[string]IResourceSync)
	}
	if _, ok := resourceSyncMap[sync.SyncType()]; ok {
		return errors.Errorf(fmt.Sprintf("syncType:%s has registered", sync.SyncType()))
	}
	resourceSyncMap[sync.SyncType()] = sync
	return nil
}

func GetResourceSyncByType(syncType string) IResourceSync {
	if resourceSyncMap == nil {
		resourceSyncMap = make(map[string]IResourceSync)
	}
	return resourceSyncMap[syncType]
}

func GetResourceSyncMap() map[string]IResourceSync {
	if resourceSyncMap == nil {
		resourceSyncMap = make(map[string]IResourceSync)
	}
	return resourceSyncMap
}

type SyncObject struct {
	sync IResourceSync
}

type IResourceSync interface {
	SyncResources(ctx context.Context, userCred mcclient.TokenCredential, param jsonutils.JSONObject) error
	SyncType() string
}

func newSyncObj(sync IResourceSync) SyncObject {
	return SyncObject{sync: sync}
}

func (manager *SBillResourceManager) SyncResources(ctx context.Context, userCred mcclient.TokenCredential, isStart bool) {
	log.Infoln("start sync resources")
	session := auth.GetAdminSession(ctx, "", "")
	// sync servers
	manager.SyncServers(ctx, session)
	// sync baremetals
	manager.SyncBaremetals(ctx, session)
	// sync disks
	manager.SyncDisks(ctx, session)
}

func (manager *SBillResourceManager) SyncServers(ctx context.Context, session *mcclient.ClientSession) error {
	log.Infoln("start sync servers")
	// sync servers
	servers, err := ListGuests(ctx, session)
	if err != nil {
		return errors.Wrapf(err, fmt.Sprintf("ListGuests err"))
	}
	billResources, err := manager.GetBillResources(RES_TYPE_SERVER)
	if err != nil {
		return errors.Wrap(err, "GetBillResources server err")
	}
	errs := make([]error, 0)
Loop:
	for i := range billResources {
		for index, server := range servers {
			if server.Id == billResources[i].ResourceId {
				if billResources[i].IsChanged(server.ProjectId, server.Project, server.InstanceType, "0") {
					//_, err = db.Update(&billResources[i], func() error {
					//	billResources[i].Cpu = server.VcpuCount
					//	billResources[i].Mem = server.VmemSize
					//	billResources[i].Model = server.InstanceType
					//	billResources[i].Project = server.Project
					//	billResources[i].ProjectId = server.ProjectId
					//	return nil
					//})
					err = billResources[i].DoExpired()
					if err != nil {
						errs = append(errs, errors.Wrapf(err, "billResources:%s Update err", billResources[i].ResourceName))
						continue Loop
					}
				} else {
					if index == len(servers)-1 {
						servers = servers[0:index]
					} else {
						servers = append(servers[0:index], servers[index+1:]...)
					}

					index--
				}
				continue Loop
			}
		}
		err := billResources[i].NoExistState()
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "NoExistState update billResources:%s err", billResources[i].ResourceName))
		}
	}
	for i := range servers {
		input := manager.newBillResourceCreateInputByServer(servers[i])
		err = manager.CreateResource(ctx, input)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "billResources:%s resType:%s DoCreate err", input.ResourceName, RES_TYPE_SERVER))
		}
	}
	return errors.NewAggregate(errs)
}

func (manager *SBillResourceManager) newBillResourceCreateInputByServer(server *compute.ServerDetails) *api.BillResourceCreateInput {
	input := new(api.BillResourceCreateInput)
	input.ResourceType = RES_TYPE_SERVER
	input.ResourceId = server.Id
	input.ResourceName = server.Name
	input.Project = server.Project
	input.ProjectId = server.ProjectId
	input.ZoneId = server.ZoneId
	input.Zone = server.Zone
	input.RegionId = server.RegionId
	input.Region = server.Region
	input.AssociateId = server.Id
	input.Cpu = server.VcpuCount
	input.Mem = server.VmemSize / 1024
	input.Model = server.InstanceType
	input.UsageModel = strings.Split(server.InstanceType, ".")[1]
	input.Size = 0

	return input
}

func (manager *SBillResourceManager) SyncBaremetals(ctx context.Context, session *mcclient.ClientSession) error {
	log.Infoln("start sync baremetals")
	// sync baremetals
	baremetals, err := ListBaremetals(ctx, session)
	if err != nil {
		return errors.Wrapf(err, fmt.Sprintf("ListBaremetals err"))
	}
	billResources, err := manager.GetBillResources(RES_TYPE_BAREMETAL)
	if err != nil {
		return errors.Wrap(err, "GetBillResources baremetal err")
	}
	errs := make([]error, 0)
Loop:
	for i := range billResources {
		for index, baremetal := range baremetals {
			if baremetal.Id == billResources[i].ResourceId {
				spec := fmt.Sprintf("%dC%dM", baremetal.VcpuCount, baremetal.VmemSize/1024)
				if billResources[i].IsBaremetalChanged(baremetal.ProjectId, baremetal.Project, spec, "0") {
					err = billResources[i].DoExpired()
					if err != nil {
						errs = append(errs, errors.Wrapf(err, "billResources:%s Update err", billResources[i].ResourceName))
						continue Loop
					}
				} else {
					if index == len(baremetals)-1 {
						baremetals = baremetals[0:index]
					} else {
						baremetals = append(baremetals[0:index], baremetals[index+1:]...)
					}

					index--
				}
				continue Loop
			}
		}
		err := billResources[i].NoExistState()
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "NoExistState update billResources:%s err", billResources[i].ResourceName))
		}
	}
	for i := range baremetals {
		input := manager.newBillResourceCreateInputByBaremetal(session, baremetals[i])
		if input == nil {
			continue
		}
		err = manager.CreateResource(ctx, input)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "billResources:%s resType:%s DoCreate err", input.ResourceName, RES_TYPE_BAREMETAL))
		}

		//billResourcess, _ := manager.GetBillResources(RES_TYPE_BAREMETAL)
		//aa, err := BillManager.Compute("1111111111", &billResourcess[0])
		//if err != nil {
		//	log.Errorf("create compute input err", err)
		//	errs = append(errs, errors.Wrapf(err, "billResources:%s compute err", billResourcess[0].ResourceName))
		//	continue
		//}
		//err = BillManager.CreateBill(ctx, aa, time.Now())
		//if err != nil {
		//	log.Errorf("create bill err", err)
		//	errs = append(errs, errors.Wrapf(err, "billResources:%s create bill err", billResourcess[0].ResourceName))
		//	continue
		//}
	}
	return errors.NewAggregate(errs)
}

func (manager *SBillResourceManager) newBillResourceCreateInputByBaremetal(session *mcclient.ClientSession, server *compute.ServerDetails) *api.BillResourceCreateInput {
	input := new(api.BillResourceCreateInput)
	input.ResourceType = RES_TYPE_BAREMETAL
	input.ResourceId = server.Id
	input.ResourceName = server.Name
	input.Project = server.Project
	input.ProjectId = server.ProjectId
	input.ZoneId = server.ZoneId
	input.Zone = server.Zone
	input.RegionId = server.RegionId
	input.Region = server.Region
	input.AssociateId = server.Id
	input.Cpu = server.VcpuCount
	input.Mem = server.VmemSize / 1024

	// get model
	hostJson, err := mc_mds.Hosts.GetById(session, server.HostId, nil)
	if err != nil {
		log.Errorf("get baremetal host %s err %v", server.Host, err)
		return nil
	}
	hostDetail := new(compute.HostDetails)
	err = hostJson.Unmarshal(hostDetail)
	if err != nil {
		log.Errorf("fail to unmarshal %s HostDetails %v", server.Host, err)
		return nil
	}
	var manufacture string
	var model string
	if hostDetail.SysInfo == nil {
		log.Errorf("hostDetail.SysInfo %s is nil", server.Host)
		return nil
	}
	manufacture, _ = hostDetail.SysInfo.GetString("manufacture")
	model, _ = hostDetail.SysInfo.GetString("model")
	if manufacture == "" || model == "" {
		log.Errorf("manufacture or model %s is nil", server.Host)
		return nil
	}
	manufactureR := strings.ReplaceAll(manufacture, " ", "_")
	modelR := strings.ReplaceAll(model, " ", "_")
	spec := fmt.Sprintf("cpu:%d/mem:%dM/manufacture:%s/model:%s", server.VcpuCount, server.VmemSize, manufactureR, modelR)
	input.Model = spec
	//input.UsageModel = strings.Split(server.InstanceType, ".")[1]
	input.Size = 0

	return input
}

func (manager *SBillResourceManager) SyncDisks(ctx context.Context, session *mcclient.ClientSession) error {
	log.Infoln("start sync disks")
	// sync disks
	disks, err := ListDisks(ctx, session)
	if err != nil {
		return errors.Wrapf(err, fmt.Sprintf("ListDisks err"))
	}
	billResources, err := manager.GetBillResources(RES_TYPE_DISK)
	if err != nil {
		return errors.Wrap(err, "GetBillResources disk err")
	}
	errs := make([]error, 0)
Loop:
	for i := range billResources {
		for index, disk := range disks {
			if disk.Id == billResources[i].ResourceId {
				model := disk.MediumType + "::" + disk.StorageType
				if billResources[i].IsChanged(disk.ProjectId, disk.Project, model, strconv.Itoa(disk.DiskSize/1024)) {
					err = billResources[i].DoExpired()
					if err != nil {
						errs = append(errs, errors.Wrapf(err, "billResources:%s Update err", billResources[i].ResourceName))
						continue Loop
					}
				} else {
					if index == len(disks)-1 {
						disks = disks[0:index]
					} else {
						disks = append(disks[0:index], disks[index+1:]...)
					}

					index--
				}
				continue Loop
			}
		}
		err := billResources[i].NoExistState()
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "NoExistState update disk billResources:%s err", billResources[i].ResourceName))
		}
	}
	for i := range disks {
		input := manager.newBillResourceCreateInputByDisk(disks[i])
		err = manager.CreateResource(ctx, input)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "billResources:%s resType:%s DoCreate err", input.ResourceName, RES_TYPE_DISK))
		}
	}
	return errors.NewAggregate(errs)
}

func (manager *SBillResourceManager) newBillResourceCreateInputByDisk(disk *compute.DiskDetails) *api.BillResourceCreateInput {
	input := new(api.BillResourceCreateInput)
	input.ResourceType = RES_TYPE_DISK
	input.ResourceId = disk.Id
	input.ResourceName = disk.Name
	input.Project = disk.Project
	input.ProjectId = disk.ProjectId
	input.ZoneId = disk.ZoneId
	input.Zone = disk.Zone
	input.RegionId = disk.RegionId
	input.Region = disk.Region
	if disk.GuestCount > 0 {
		input.AssociateId = disk.Guests[0].Id
	}
	input.Cpu = 0
	input.Mem = 0
	input.Model = disk.MediumType + "::" + disk.StorageType
	input.UsageModel = disk.MediumType + "::" + disk.StorageType
	input.Size = disk.DiskSize / 1024

	return input
}

func GetOnecloudResources(resTyep string) ([]jsonutils.JSONObject, error) {
	var err error
	allResources := make([]jsonutils.JSONObject, 0)

	query := jsonutils.NewDict()
	query.Add(jsonutils.NewStringArray([]string{"running", "ready"}), "status")
	query.Add(jsonutils.NewString("true"), "admin")
	switch resTyep {
	case monitor.METRIC_RES_TYPE_HOST:
		//query.Set("host-type", jsonutils.NewString(hostconsts.TELEGRAF_TAG_KEY_HYPERVISOR))
		allResources, err = ListAllResources(&mc_mds.Hosts, query)
	case monitor.METRIC_RES_TYPE_GUEST:
		allResources, err = ListAllResources(&mc_mds.Servers, query)
	case monitor.METRIC_RES_TYPE_AGENT:
		allResources, err = ListAllResources(&mc_mds.Servers, query)
	case monitor.METRIC_RES_TYPE_RDS:
		allResources, err = ListAllResources(&mc_mds.DBInstance, query)
	case monitor.METRIC_RES_TYPE_REDIS:
		allResources, err = ListAllResources(&mc_mds.ElasticCache, query)
	case monitor.METRIC_RES_TYPE_OSS:
		allResources, err = ListAllResources(&mc_mds.Buckets, query)
	case monitor.METRIC_RES_TYPE_CLOUDACCOUNT:
		query.Remove("status")
		query.Add(jsonutils.NewBool(true), "enabled")
		allResources, err = ListAllResources(&mc_mds.Cloudaccounts, query)
	case monitor.METRIC_RES_TYPE_TENANT:
		allResources, err = ListAllResources(&mc_mds.Projects, query)
	case monitor.METRIC_RES_TYPE_DOMAIN:
		allResources, err = ListAllResources(&mc_mds.Domains, query)
	case monitor.METRIC_RES_TYPE_STORAGE:
		query.Remove("status")
		allResources, err = ListAllResources(&mc_mds.Storages, query)
	default:
		query := jsonutils.NewDict()
		query.Set("brand", jsonutils.NewString(hostconsts.TELEGRAF_TAG_ONECLOUD_BRAND))
		query.Set("host-type", jsonutils.NewString(hostconsts.TELEGRAF_TAG_KEY_HYPERVISOR))
		allResources, err = ListAllResources(&mc_mds.Hosts, query)
	}

	if err != nil {
		return nil, errors.Wrap(err, "NoDataQueryCondition Host list error")
	}
	return allResources, nil
}

func ListAllResources(manager modulebase.Manager, params *jsonutils.JSONDict) ([]jsonutils.JSONObject, error) {
	if params == nil {
		params = jsonutils.NewDict()
	}
	params.Add(jsonutils.NewString("system"), "scope")
	params.Add(jsonutils.NewInt(0), "limit")
	params.Add(jsonutils.NewBool(true), "details")
	var count int
	session := auth.GetAdminSession(context.Background(), "", "")
	objs := make([]jsonutils.JSONObject, 0)
	for {
		params.Set("offset", jsonutils.NewInt(int64(count)))
		result, err := manager.List(session, params)
		if err != nil {
			return nil, errors.Wrapf(err, "list %s resources with params %s", manager.KeyString(), params.String())
		}
		for _, data := range result.Data {
			objs = append(objs, data)
		}
		total := result.Total
		count = count + len(result.Data)
		if count >= total {
			break
		}
	}
	return objs, nil
}

func ListGuests(ctx context.Context, session *mcclient.ClientSession) ([]*compute.ServerDetails, error) {
	guestList := make([]*compute.ServerDetails, 0)
	params := jsonutils.NewDict()
	params.Set("limit", jsonutils.NewInt(0))
	params.Set("scope", jsonutils.NewString("system"))
	params.Set("system", jsonutils.JSONTrue)
	params.Set("pending_delete", jsonutils.NewBool(false))
	params.Set("hypervisor", jsonutils.NewString("kvm"))
	//params.Set("get_all_guests_on_host", jsonutils.NewString(m.host.GetHostId()))
	//params.Set("filter.0", jsonutils.NewString(fmt.Sprintf("id.in(%s)", strings.Join(keys, ","))))
	res, err := mc_mds.Servers.List(session, params)
	if err != nil {
		log.Errorf("get server list err %v", err)
		return nil, err
	} else {
		for i := range res.Data {
			//log.Infof(v.String())
			serverDetail := new(compute.ServerDetails)
			err = res.Data[i].Unmarshal(serverDetail)
			if err != nil {
				log.Errorf("fail to unmarshal ServerDetails %v", err)
				continue
			}
			guestList = append(guestList, serverDetail)
		}
	}
	if len(guestList) != len(res.Data) {
		log.Errorf("the unmarshal list len is %d not equal res.Data len %d", len(guestList), len(res.Data))
	}
	return guestList, nil
}

func ListBaremetals(ctx context.Context, session *mcclient.ClientSession) ([]*compute.ServerDetails, error) {
	guestList := make([]*compute.ServerDetails, 0)
	params := jsonutils.NewDict()
	params.Set("limit", jsonutils.NewInt(0))
	params.Set("scope", jsonutils.NewString("system"))
	params.Set("system", jsonutils.JSONTrue)
	params.Set("pending_delete", jsonutils.NewBool(false))
	params.Set("hypervisor", jsonutils.NewString("baremetal"))
	res, err := mc_mds.Servers.List(session, params)
	if err != nil {
		log.Errorf("get baremetal list err %v", err)
		return nil, err
	} else {
		for i := range res.Data {
			//log.Infof(v.String())
			serverDetail := new(compute.ServerDetails)
			err = res.Data[i].Unmarshal(serverDetail)
			if err != nil {
				log.Errorf("fail to unmarshal baremetal ServerDetails %v", err)
				continue
			}
			guestList = append(guestList, serverDetail)
		}
	}
	if len(guestList) != len(res.Data) {
		log.Errorf("the unmarshal list len is %d not equal res.Data len %d", len(guestList), len(res.Data))
	}
	return guestList, nil
}

func ListDisks(ctx context.Context, session *mcclient.ClientSession) ([]*compute.DiskDetails, error) {
	diskList := make([]*compute.DiskDetails, 0)
	params := jsonutils.NewDict()
	params.Set("limit", jsonutils.NewInt(0))
	params.Set("scope", jsonutils.NewString("system"))
	params.Set("system", jsonutils.JSONTrue)
	params.Set("pending_delete", jsonutils.NewBool(false))
	res, err := mc_mds.Disks.List(session, params)
	if err != nil {
		log.Errorf("get disk list err %v", err)
		return nil, err
	} else {
		for i := range res.Data {
			//log.Infof(v.String())
			diskDetail := new(compute.DiskDetails)
			err = res.Data[i].Unmarshal(diskDetail)
			if err != nil {
				log.Errorf("fail to unmarshal diskDetails %v", err)
				continue
			}
			if diskDetail.StorageType == compute.STORAGE_BAREMETAL {
				continue
			}
			diskList = append(diskList, diskDetail)
		}
	}
	return diskList, nil
}

//func ListHosts(ctx context.Context, session *mcclient.ClientSession) ([]*compute.HostDetails, error) {
//	hostList := make([]*compute.HostDetails, 0)
//	params := jsonutils.NewDict()
//	params.Set("limit", jsonutils.NewInt(0))
//	params.Set("scope", jsonutils.NewString("system"))
//	params.Set("baremetal", jsonutils.JSONTrue)
//	params.Set("host_type", jsonutils.NewString("baremetal"))
//	params.Set("details", jsonutils.JSONTrue)
//	res, err := mc_mds.Hosts.List(session, params)
//	if err != nil {
//		log.Errorf("get host list err %v", err)
//		return nil, err
//	} else {
//		for i := range res.Data {
//			//log.Infof(v.String())
//			hostDetail := new(compute.HostDetails)
//			err = res.Data[i].Unmarshal(hostDetail)
//			if err != nil {
//				log.Errorf("fail to unmarshal hostDetails %v", err)
//				continue
//			}
//			if hostDetail.Guests == 0 || hostDetail.ServerPendingDeleted == true {
//				continue
//			}
//			hostList = append(hostList, hostDetail)
//		}
//	}
//	return hostList, nil
//}
