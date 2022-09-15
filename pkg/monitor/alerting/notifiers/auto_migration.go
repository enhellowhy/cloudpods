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

package notifiers

import (
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud/pkg/apis/monitor"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/monitor/alerting"
	"yunion.io/x/onecloud/pkg/monitor/controller/balancer"
	"yunion.io/x/onecloud/pkg/monitor/models"
	"yunion.io/x/onecloud/pkg/monitor/options"
)

func init() {
	alerting.RegisterNotifier(&alerting.NotifierPlugin{
		Type:    monitor.AlertNotificationTypeAutoMigration,
		Factory: newAutoMigratingNotifier,
		ValidateCreateData: func(cred mcclient.IIdentityProvider, input monitor.NotificationCreateInput) (monitor.NotificationCreateInput, error) {
			settings := new(monitor.NotificationSettingAutoMigration)
			if err := input.Settings.Unmarshal(settings); err != nil {
				return input, errors.Wrap(err, "Unmarshal setting")
			}
			input.Settings = jsonutils.Marshal(settings)
			return input, nil
		},
	})
}

type autoMigrationNotifier struct {
	NotifierBase

	Settings *monitor.NotificationSettingAutoMigration
}

func newAutoMigratingNotifier(conf alerting.NotificationConfig) (alerting.Notifier, error) {
	settings := new(monitor.NotificationSettingAutoMigration)
	if err := conf.Settings.Unmarshal(settings); err != nil {
		return nil, errors.Wrap(err, "unmarshal setting")
	}
	return &autoMigrationNotifier{
		NotifierBase: NewNotifierBase(conf),
		Settings:     settings,
	}, nil
}

func (am *autoMigrationNotifier) getBalancerRules(ctx *alerting.EvalContext, alert *models.SMigrationAlert) (*balancer.Rules, error) {
	if len(ctx.EvalMatches) >= 1 {
		//log.Warningf("EvalMatches great than 1, use first one")
		log.Warningf("EvalMatches length is %d", len(ctx.EvalMatches))
	} else {
		return nil, errors.Errorf("Matches not >= 1 %d", len(ctx.EvalMatches))
	}
	var match *monitor.EvalMatch //match is nil
	//match := new(monitor.EvalMatch) in this case, match is not nil
	//schedParams := self.GetSchedMigrateParams(userCred, input)
	s := auth.GetAdminSession(ctx.Ctx, options.Options.Region, "")
	//_, res, err := modules.SchedManager.DoScheduleForecast(s, schedParams, 1)
	//if err != nil {
	//	return nil, errors.Wrap(err, "Do schedule migrate forecast")
	//}
	res, err := modules.Clusterhosts.List(s, nil)
	if err != nil {
		return nil, errors.Wrap(err, "get Clusterhosts")
	}
	hosts := []api.HostclusterDetails{}
	jsonutils.Update(&hosts, res.Data)
	if len(hosts) == 0 {
		return nil, errors.Wrap(err, "no host cluster")
	}
EVAL:
	for i, m := range ctx.EvalMatches {
		if id, ok := m.Tags["host_id"]; ok {
			for _, host := range hosts {
				if host.HostId == id {
					cluster, err := modules.Clusters.GetById(s, host.ClusterId, nil)
					if err != nil {
						log.Errorf("get cluster %s details %v", host.ClusterId, err)
						//return nil, errors.Wrapf(err, "get cluster %s details", host.ClusterId)
						continue EVAL
					}
					clusterDetail := new(api.ClusterDetails)
					err = cluster.Unmarshal(clusterDetail)
					if err != nil {
						log.Errorf("fail to unmarshal clusterDetail %v", err)
						continue EVAL
						//return nil, errors.Wrapf(err, "fail to unmarshal clusterDetail %s", cluster.String())
					}
					if !*clusterDetail.DrsEnabled {
						log.Warningf("DrsEnabled is false for cluster %s", clusterDetail.Name)
						continue EVAL
						//return nil, errors.Errorf("DrsEnabled is false for cluster %s", clusterDetail.Name)
					}
					match = ctx.EvalMatches[i]
					match.Tags["cluster"] = host.ClusterId
					break EVAL
				}
			}
		}
	}
	if match == nil {
		log.Warningf("EvalMatches length is %d, but no match", len(ctx.EvalMatches))
		return nil, errors.Errorf("no eval match")
	}
	drv, err := balancer.GetMetricDrivers().Get(alert.GetMetricType())
	if err != nil {
		return nil, errors.Wrap(err, "Get metric driver")
	}
	//match
	//{"metric":"mem.used_percent","tags":{"access_ip":"10.133.0.2","brand":"OneCloud","cloudregion":"北京","cloudregion_id":"default","domain_id":"dault","host":"cloudpod-0-2-10-133-0-2","host_id":"0c75c565-a7e1-4ff9-8afb-a89d4fd41144","ip":"10.133.0.2","name":"cloudpod-0-2-10-133-0-2","project_domain":"Default","status":"running","zone":"可用区A","zone_id":"7f1e3985-a36f-44c3-8130-3fcf4f47f18d"},"unit":"%","value":77.54603103945,"value_str":"77.0835%"}
	log.Infof("autoMigrationNotifier for evalMatch: %s", jsonutils.Marshal(match))
	return balancer.NewRules(ctx, match, alert, drv)
}

func (am *autoMigrationNotifier) Notify(ctx *alerting.EvalContext, data jsonutils.JSONObject) error {
	if !ctx.Firing {
		// do nothing
		return nil
	}

	alertId := ctx.Rule.Id
	man := models.GetMigrationAlertManager()
	obj, err := man.FetchById(alertId)
	if err != nil {
		return errors.Wrapf(err, "Fetch alert by Id: %q", alertId)
	}
	alert := obj.(*models.SMigrationAlert)

	rules, err := am.getBalancerRules(ctx, alert)
	if err != nil {
		return errors.Wrapf(err, "get balancer rules")
	}

	if err := balancer.DoBalance(ctx.Ctx, auth.GetAdminSession(ctx.Ctx, options.Options.Region, ""), rules, balancer.NewRecorder()); err != nil {
		return errors.Wrap(err, "DoBalance")
	}

	return nil
}
