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
	"crypto/md5"
	"fmt"
	"strconv"
	"strings"
	"time"
	"yunion.io/x/jsonutils"
	api "yunion.io/x/onecloud/pkg/apis/billing"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/util/rbacutils"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/sqlchemy"
)

const ()

// +onecloud:swagger-gen-ignore
type SBillResourceManager struct {
	db.SResourceBaseManager
	db.SProjectizedResourceBaseManager
}

var BillResourceManager *SBillResourceManager

func init() {
	BillResourceManager = &SBillResourceManager{
		SResourceBaseManager: db.NewResourceBaseManager(
			SBillResource{},
			"bill_resources_tbl",
			"bill_resource",
			"bill_resources",
		),
	}
	BillResourceManager.SetVirtualObject(BillResourceManager)
}

/*
+--------------+------------+----+---+-------+-----+
|Field         |Type        |Null|Key|Default|Extra|
+--------------+------------+----+---+-------+-----+
|created_at    |datetime    |NO  |MUL|NULL   |     |
|updated_at    |datetime    |NO  |   |NULL   |     |
|update_version|int(11)     |NO  |   |0      |     |
|deleted_at    |datetime    |YES |   |NULL   |     |
|deleted       |tinyint(1)  |NO  |   |0      |     |
|id            |varchar(128)|NO  |PRI|NULL   |     |
|account_id    |varchar(128)|YES |   |NULL   |     |
|account       |varchar(128)|YES |   |NULL   |     |
|provider      |varchar(32) |YES |MUL|NULL   |     |
|brand         |varchar(32) |YES |   |NULL   |     |
|domain_id     |varchar(128)|YES |   |NULL   |     |
|domain        |varchar(128)|YES |   |NULL   |     |
|project_id    |varchar(128)|YES |   |NULL   |     |
|project       |varchar(128)|YES |   |NULL   |     |
|external_id   |varchar(256)|YES |MUL|NULL   |     |
|region_id     |varchar(128)|YES |   |NULL   |     |
|region        |varchar(128)|YES |   |NULL   |     |
|zone_id       |varchar(128)|YES |   |NULL   |     |
|zone          |varchar(128)|YES |   |NULL   |     |
|associate_id  |varchar(128)|YES |MUL|NULL   |     |
|resource_id            |varchar(256)|YES |MUL|NULL   |     |
|resource_name          |varchar(256)|YES |   |NULL   |     |
|resource_type |varchar(64) |YES |   |NULL   |     |
|cpu           |int(11)     |YES |   |0      |     |
|mem           |double      |YES |   |0      |     |
|size          |double      |YES |   |0      |     |
|model         |varchar(128)|YES |MUL|NULL   |     |
|usage_start_time       |datetime    |YES |   |NULL   |     |
|usage_end_time         |datetime    |YES |   |NULL   |     |
|expired       |tinyint(1)  |YES |   |0      |     |
|exist         |tinyint(1)  |YES |   |0      |     |
+--------------+------------+----+---+-------+-----+
*/

type SBillResource struct {
	db.SResourceBase
	db.SProjectizedResourceBase

	Id             string    `width:"128" charset:"ascii" primary:"true" list:"user" json:"id"`
	ResourceId     string    `width:"256" charset:"ascii" index:"true" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"resource_id"`
	ResourceName   string    `width:"256" charset:"ascii" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"resource_name"`
	ResourceType   string    `width:"32" charset:"ascii" index:"true" list:"user" create:"domain_optional" update:"admin" json:"resource_type"`
	AccountId      string    `width:"128" charset:"ascii" index:"true" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"account_id"`
	Account        string    `width:"128" charset:"ascii" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"account"`
	Provider       string    `width:"32" charset:"ascii" list:"user" create:"domain_optional" update:"admin" json:"provider"`
	Brand          string    `width:"32" charset:"ascii" index:"true" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"brand"`
	Domain         string    `width:"128" charset:"utf8" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"domain"`
	Project        string    `width:"128" charset:"utf8" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"project"`
	RegionId       string    `width:"128" charset:"ascii" index:"true" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"region_id"`
	Region         string    `width:"128" charset:"utf8" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"region"`
	ZoneId         string    `width:"128" charset:"ascii" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"zone_id"`
	Zone           string    `width:"128" charset:"utf8" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"zone"`
	AssociateId    string    `width:"256" charset:"ascii" index:"true" list:"user" create:"domain_optional" update:"admin" json:"associate_id"`
	Cpu            int       `list:"user" update:"user" json:"cpu"`
	Mem            int       `list:"user" update:"user" json:"mem"`
	Size           int       `list:"user" update:"user" json:"size"`
	Model          string    `width:"256" charset:"utf8" index:"true" list:"user" create:"domain_optional" update:"admin" json:"model"`
	UsageModel     string    `width:"256" charset:"utf8" index:"true" list:"user" create:"domain_optional" update:"admin" json:"usage_model"`
	UsageStartTime time.Time `nullable:"false" list:"user" index:"true" create:"domain_optional" json:"usage_start_time"`
	UsageEndTime   time.Time `nullable:"true" list:"user" index:"true" create:"domain_optional" json:"usage_end_time"`
	Expired        bool      `list:"user" create:"domain_optional" update:"admin" json:"expired"`
	Exist          bool      `list:"user" create:"domain_optional" update:"admin" json:"exist"`
}

func (manager *SBillResourceManager) FilterById(q *sqlchemy.SQuery, idStr string) *sqlchemy.SQuery {
	return q.Equals("id", idStr)
}

func (manager *SBillResourceManager) QueryDistinctExtraField(q *sqlchemy.SQuery, field string) (*sqlchemy.SQuery, error) {
	var err error

	q, err = manager.SResourceBaseManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}
	return q, httperrors.ErrNotFound
}

func (manager *SBillResourceManager) NamespaceScope() rbacutils.TRbacScope {
	return rbacutils.ScopeProject
}

func (manager *SBillResourceManager) ResourceScope() rbacutils.TRbacScope {
	return rbacutils.ScopeProject
}

func (manager *SBillResourceManager) ListItemFilter(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	input api.BillResourceListInput,
) (*sqlchemy.SQuery, error) {
	var err error
	q, err = BillManager.SResourceBaseManager.ListItemFilter(ctx, q, userCred, input.ResourceBaseListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SResourceBaseManager.ListItemFilter")
	}
	// res_type
	if len(input.ResType) > 0 {
		q.Equals("resource_type", input.ResType)
	}
	//project
	if len(input.Project) > 0 {
		q.Equals("project", input.Project)
	}
	// start_day
	if input.StartDay > 0 {
		q = q.In("resource_name", input.ResNames)
	}

	return q, nil
}

func (manager *SBillResourceManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []api.BillDetails {
	rows := make([]api.BillDetails, len(objs))

	for i := range rows {
		billResource := objs[i].(*SBillResource)

		rows[i] = api.BillDetails{
			BillId:    billResource.Id,
			ResId:     billResource.ResourceId,
			ResName:   billResource.ResourceName,
			ResType:   billResource.ResourceType,
			StartTime: billResource.UsageStartTime.Format(TIME2_FORMAT),
			EndTime:   billResource.UsageEndTime.Format(TIME2_FORMAT),
		}
	}
	return rows
}

func (manager *SBillResourceManager) OrderByExtraFields(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.BillListInput,
) (*sqlchemy.SQuery, error) {
	var err error

	q, err = manager.SResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.ResourceBaseListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SResourceBaseManager.OrderByExtraFields")
	}

	return q, nil
}

func (manager *SBillResourceManager) GetAllBillResources(resType []string) ([]SBillResource, error) {
	billResources := make([]SBillResource, 0)
	q := manager.Query().Equals("exist", true).Equals("expired", false).In("resource_type", resType)
	err := db.FetchModelObjects(manager, q, &billResources)
	if err != nil {
		return nil, errors.Wrap(err, "SBillResourceManager FetchModelObjects err")
	}
	return billResources, nil
}

func (manager *SBillResourceManager) GetBillResources(resType string) ([]SBillResource, error) {
	billResources := make([]SBillResource, 0)
	q := manager.Query().Equals("exist", true).Equals("expired", false).Equals("resource_type", resType)
	err := db.FetchModelObjects(manager, q, &billResources)
	if err != nil {
		return nil, errors.Wrap(err, "SBillResourceManager FetchModelObjects err")
	}
	return billResources, nil
}

func (manager *SBillResourceManager) CreateResource(ctx context.Context, input *api.BillResourceCreateInput) error {
	bill := new(SBillResource)

	bill.Id = db.DefaultUUIDGenerator()
	bill.Domain = "Default"
	bill.Brand = "OneCloud"
	bill.Provider = "OneCloud"
	bill.Exist = true
	bill.ResourceType = input.ResourceType
	bill.ResourceId = input.ResourceId
	bill.ResourceName = input.ResourceName
	bill.Project = input.Project
	bill.ProjectId = input.ProjectId
	bill.ZoneId = input.ZoneId
	bill.Zone = input.Zone
	bill.RegionId = input.RegionId
	bill.Region = input.Region
	bill.AssociateId = input.AssociateId
	bill.UsageStartTime = time.Now()
	bill.Cpu = input.Cpu
	bill.Mem = input.Mem
	bill.Size = input.Size
	bill.Model = input.Model
	bill.UsageModel = input.UsageModel

	bill.SetModelManager(manager, bill)
	return manager.TableSpec().Insert(ctx, bill)
}

func (self *SBillResource) NoExistState() error {
	_, err := db.Update(self, func() error {
		self.Exist = false
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "SBillResource:%s NoExistState err", self.ResourceName)
	}
	return nil
}

func (self *SBillResource) DoExpired() error {
	_, err := db.Update(self, func() error {
		self.Expired = true
		self.UsageEndTime = time.Now()
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "SBillResource:%s DoExpired err", self.ResourceName)
	}
	return nil
}

func (self *SBillResource) IsChanged(elems ...string) bool {

	var signature = md5.Sum([]byte(strings.Join(elems, ",")))
	sign := fmt.Sprintf("%x", signature)

	base := make([]string, 0)
	base = append(base, self.ProjectId)
	base = append(base, self.Project)
	base = append(base, self.Model)
	base = append(base, strconv.Itoa(self.Size))

	var baseSignature = md5.Sum([]byte(strings.Join(base, ",")))
	baseSign := fmt.Sprintf("%x", baseSignature)

	if sign == baseSign {
		return false
	}
	return true
}

func (self *SBillResource) IsStorageChanged(elems ...string) bool {

	var signature = md5.Sum([]byte(strings.Join(elems, ",")))
	sign := fmt.Sprintf("%x", signature)

	base := make([]string, 0)
	base = append(base, self.ProjectId)
	base = append(base, self.Project)
	base = append(base, self.Model)
	//base = append(base, strconv.Itoa(self.Size))

	var baseSignature = md5.Sum([]byte(strings.Join(base, ",")))
	baseSign := fmt.Sprintf("%x", baseSignature)

	if sign == baseSign {
		return false
	}
	return true
}

func (self *SBillResource) IsBaremetalChanged(elems ...string) bool {

	var signature = md5.Sum([]byte(strings.Join(elems, ",")))
	sign := fmt.Sprintf("%x", signature)

	base := make([]string, 0)
	base = append(base, self.ProjectId)
	base = append(base, self.Project)
	spec := fmt.Sprintf("%dC%dM", self.Cpu, self.Mem)
	base = append(base, spec)
	base = append(base, strconv.Itoa(self.Size))

	var baseSignature = md5.Sum([]byte(strings.Join(base, ",")))
	baseSign := fmt.Sprintf("%x", baseSignature)

	if sign == baseSign {
		return false
	}
	return true
}
