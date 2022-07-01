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
	"yunion.io/x/pkg/utils"

	"yunion.io/x/jsonutils"
	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/util/influxdb"
	"yunion.io/x/pkg/errors"
)

func (lblis *SLoadbalancerListener) GetDetailsBackendStatus(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	provider := lblis.GetCloudprovider()
	if provider != nil {
		return jsonutils.NewArray(), nil
	}
	if lblis.BackendGroupId == "" {
		return jsonutils.NewArray(), nil
	}
	//var pxname string
	//switch lblis.ListenerType {
	//case api.LB_LISTENER_TYPE_TCP:
	//	pxname = fmt.Sprintf("backends_listener-%s", lblis.Id)
	//case api.LB_LISTENER_TYPE_HTTP, api.LB_LISTENER_TYPE_HTTPS:
	//	pxname = fmt.Sprintf("backends_listener_default-%s", lblis.Id)
	//}
	//return lbGetBackendGroupCheckStatus(ctx, userCred, lblis.LoadbalancerId, pxname, lblis.BackendGroupId)
	return lbGetBackendGroupCheckStatus(ctx, userCred, lblis.LoadbalancerId, lblis.Id, lblis.BackendGroupId)
}

func (lbr *SLoadbalancerListenerRule) GetDetailsBackendStatus(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject) (jsonutils.JSONObject, error) {
	provider := lbr.GetCloudprovider()
	if provider != nil {
		return jsonutils.NewArray(), nil
	}
	lblis, err := lbr.GetLoadbalancerListener()
	if err != nil {
		return nil, err
	}
	pxname := fmt.Sprintf("backends_rule-%s", lbr.Id)
	return lbGetBackendGroupCheckStatus(ctx, userCred, lblis.LoadbalancerId, pxname, lbr.BackendGroupId)
}

func lbGetInfluxdbByLbId(lbId string) (*influxdb.SInfluxdb, string, error) {
	lb, err := LoadbalancerManager.getLoadbalancer(lbId)
	if err != nil {
		return nil, "", err
	}
	lbagents, err := LoadbalancerAgentManager.getByClusterId(lb.ClusterId)
	if err != nil {
		return nil, "", err
	}
	if len(lbagents) == 0 {
		return nil, "", errors.Wrapf(errors.ErrNotFound, "lbcluster %s has no agent", lb.ClusterId)
	}
	var (
		dbUrl  string
		dbName string
	)
	for i := range lbagents {
		lbagent := &lbagents[i]
		params := lbagent.Params
		if params == nil {
			continue
		}
		paramsTelegraf := params.Telegraf
		if paramsTelegraf.InfluxDbOutputUrl != "" && paramsTelegraf.InfluxDbOutputName != "" {
			dbUrl = paramsTelegraf.InfluxDbOutputUrl
			dbName = paramsTelegraf.InfluxDbOutputName
			if lbagent.HaState == api.LB_HA_STATE_MASTER {
				// prefer the one on master
				break
			}
		}
	}
	if dbUrl == "" || dbName == "" {
		return nil, "", errors.Wrap(errors.ErrNotFound, "no lbagent has influxdb url or db name")
	}
	dbinst := influxdb.NewInfluxdb(dbUrl)
	return dbinst, dbName, nil
}

func lbGetBackendGroupCheckStatus(ctx context.Context, userCred mcclient.TokenCredential, lbId string, lbLis string, groupId string) (*jsonutils.JSONArray, error) {
	var (
		backendJsons []jsonutils.JSONObject
		backendIds   []string
	)
	{
		var err error
		q := LoadbalancerBackendManager.Query().Equals("backend_group_id", groupId).IsFalse("pending_deleted")
		backendJsons, err = db.Query2List(LoadbalancerBackendManager, ctx, userCred, q, jsonutils.NewDict(), false)
		if err != nil {
			return nil, errors.Wrapf(err, "query backends of backend group %s", groupId)
		}
		if len(backendJsons) == 0 {
			return jsonutils.NewArray(), nil
		}
		for _, backendJson := range backendJsons {
			id, err := backendJson.GetString("id")
			if err != nil {
				return nil, errors.Wrap(err, "get backend id from json")
			}
			if id == "" {
				return nil, errors.Wrap(err, "get backend id from json: id empty")
			}
			backendIds = append(backendIds, id)
		}
	}

	dbinst, dbName, err := lbGetInfluxdbByLbId(lbId)
	if err != nil {
		return nil, errors.Wrapf(err, "find influxdb for loadbalancer %s", lbId)
	}

	//queryFmt := "select check_status, check_code from %s..haproxy where pxname = '%s' and svname =~ /........-....-....-....-............/ group by pxname, svname order by time desc limit 1"
	queryFmt := "select mean(value) AS \"check_code\" from %s..loadbalancer_backends_state where listener_uuid = '%s' and time > now()-5s and realserver_uuid =~ /........-....-....-....-............/ group by listener_uuid, realserver_uuid"
	querySql := fmt.Sprintf(queryFmt, dbName, lbLis)
	//querySql := fmt.Sprintf(queryFmt, dbName, "dc0c55c1-5dd6-4c48-855c-c075abe6d34f")
	queryRes, err := dbinst.Query(querySql)
	if err != nil {
		return nil, errors.Wrap(err, "query influxdb")
	}
	if len(queryRes) != 1 {
		return nil, fmt.Errorf("query influxdb: expecting 1 set of results, got %d", len(queryRes))
	}
	type Tags struct {
		//PxName string `json:"pxname"`
		//SvName string `json:"svname"`
		ListenerUuid   string `json:"listener_uuid"`
		RealserverUuid string `json:"realserver_uuid"`
	}
	for _, resSeries := range queryRes[0] {
		if len(resSeries.Values) == 0 {
			continue
		}
		resColumns := resSeries.Values[0]
		if len(resColumns) != 2 {
			continue
		}
		tags := Tags{}
		if err := resSeries.Tags.Unmarshal(&tags); err != nil {
			return nil, errors.Wrap(err, "unmarshal tags in influxdb query result")
		}
		ok, i := utils.InStringArray(tags.RealserverUuid, backendIds)
		if !ok {
			continue
		}
		backendJson := backendJsons[i].(*jsonutils.JSONDict)
		for j, colName := range resSeries.Columns {
			colVal := resColumns[j]
			if colVal == nil {
				colVal = jsonutils.JSONNull
			}
			if colName == "time" {
				colName = "check_time"
			} else {
				val, _ := colVal.Float()
				if val == 0 {
					backendJson.Set("check_status", jsonutils.NewString("正常"))
				} else if val == 1 {
					backendJson.Set("check_status", jsonutils.NewString("异常"))
				} else {
					backendJson.Set("check_status", jsonutils.NewString("部分异常"))
				}
			}
			backendJson.Set(colName, colVal)
		}
	}
	return jsonutils.NewArray(backendJsons...), nil
}
