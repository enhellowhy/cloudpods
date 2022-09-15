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
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/mcclient"
)

var (
	PriceInfoManager *SPriceInfoManager
)

func init() {
	PriceInfoManager = &SPriceInfoManager{
		SVirtualResourceBaseManager: db.NewVirtualResourceBaseManager(
			&SPriceInfoManager{},
			"",
			"price_info",
			"price_infos",
		),
	}
	PriceInfoManager.SetVirtualObject(PriceInfoManager)
}

type SPriceInfoManager struct {
	db.SVirtualResourceBaseManager
}

type SPriceInfoModel struct{}

func (self *SPriceInfoManager) AllowGetProperty(ctx context.Context, userCred mcclient.TokenCredential,
	query jsonutils.JSONObject) bool {
	return true
}

func (self *SPriceInfoManager) GetProperty(ctx context.Context, userCred mcclient.TokenCredential,
	query jsonutils.JSONObject) (jsonutils.JSONObject, error) {

	log.Infoln("jjjjjj")
	return nil, nil
}

func getTagFilterByRequestQuery(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject) (filter string, err error) {

	scope, _ := query.GetString("scope")
	filter, err = filterByScope(ctx, userCred, scope, query)
	return
}

func filterByScope(ctx context.Context, userCred mcclient.TokenCredential, scope string, data jsonutils.JSONObject) (string, error) {
	domainId := jsonutils.GetAnyString(data, []string{"domain_id", "domain", "project_domain_id", "project_domain"})
	projectId := jsonutils.GetAnyString(data, []string{"project_id", "project"})
	if projectId != "" {
		project, err := db.DefaultProjectFetcher(ctx, projectId)
		if err != nil {
			return "", err
		}
		projectId = project.GetProjectId()
		domainId = project.GetProjectDomainId()
	}
	if domainId != "" {
		domain, err := db.DefaultDomainFetcher(ctx, domainId)
		if err != nil {
			return "", err
		}
		domainId = domain.GetProjectDomainId()
		domain.GetProjectId()
	}
	switch scope {
	case "system":
		return "", nil
	case "domain":
		if domainId == "" {
			domainId = userCred.GetProjectDomainId()
		}
		return "", nil
	default:
		if projectId == "" {
			projectId = userCred.GetProjectId()
		}
		return "", nil
	}
}

