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
	"database/sql"
	"fmt"
	"strconv"
	"time"
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	api "yunion.io/x/onecloud/pkg/apis/billing"
	computeapi "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modules/compute"
	"yunion.io/x/onecloud/pkg/util/rbacutils"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/sqlchemy"
)

const (
	PRICE_UNIT_SERVER        = "/unit/hour"
	PRICE_UNIT_DISK          = "/GB/hour"
	QUERY_GROUP_INSTANCES    = "instances"
	QUERY_GROUP_RESOURCES    = "resources"
	QUERY_TYPE_EXPENSE_TREND = "expense_trend"
	QUERY_TYPE_PROVIDER      = "provider"
	QUERY_TYPE_RESOURCE      = "resource_type"
	QUERY_TYPE_PROJECT       = "project"
	QUERY_TYPE_CHARGE        = "charge_type"
	DATA_TYPE_DAY            = "day"
	DATA_TYPE_MONTH          = "month"
)

// +onecloud:swagger-gen-ignore
type SBillManager struct {
	db.SResourceBaseManager
	db.SProjectizedResourceBaseManager
}

var BillManager *SBillManager

func init() {
	BillManager = &SBillManager{
		SResourceBaseManager: db.NewResourceBaseManager(
			SBill{},
			"bills_tbl",
			"bill_detail",
			"bill_details",
		),
	}
	BillManager.SetVirtualObject(BillManager)
}

/*
+-----------------------+------------+----+---+-------+-----+
|Field                  |Type        |Null|Key|Default|Extra|
+-----------------------+------------+----+---+-------+-----+
|domain_id              |varchar(64) |NO  |MUL|default|     |
|tenant_id              |varchar(128)|NO  |MUL|NULL   |     |
|id                     |varchar(128)|NO  |PRI|NULL   |     |
|provider               |varchar(32) |YES |   |NULL   |     |
|account_id             |varchar(256)|YES |MUL|NULL   |     |
|account                |varchar(256)|YES |   |NULL   |     |
|resource_type          |varchar(32) |YES |MUL|NULL   |     |
|product_detail         |varchar(128)|YES |   |NULL   |  n  |
|external_id            |varchar(256)|YES |   |NULL   |  n  |
|external_name          |varchar(256)|YES |   |NULL   |  n  |
|charge_type            |varchar(32) |YES |MUL|NULL   |     |
|payment_time           |datetime    |YES |MUL|NULL   |     |
|usage_start_time       |datetime    |YES |   |NULL   |     |
|usage_end_time         |datetime    |YES |   |NULL   |     |
|gross_amount           |double      |YES |   |0      |     |
|amount                 |double      |YES |   |0      |     |
|cash_pay_amount        |double      |YES |   |0      |     |
|voucher_pay_amount     |double      |YES |   |0      |     |
|prepaid_card_pay_amount|double      |YES |   |0      |     |
|incentive_pay_amount   |double      |YES |   |0      |     |
|coupon_pay_amount      |double      |YES |   |0      |     |
|discount               |double      |YES |   |0      |     |
|discount_type          |varchar(32) |YES |   |none   |     |
|outstanding_amount     |double      |YES |   |0      |     |
|usage_type             |varchar(64) |YES |   |NULL   |     |
|rate                   |double      |YES |   |0      |     |
|price_unit             |varchar(48) |YES |   |NULL   |     |
|usage                  |double      |YES |   |0      |     |
|deducted_usage         |double      |YES |   |0      |     |
|reserved               |tinyint(1)  |YES |   |0      |     |
|resource_id            |varchar(256)|YES |MUL|NULL   |     |
|resource_name          |varchar(256)|YES |   |NULL   |     |
|brand                  |varchar(32) |YES |MUL|NULL   |     |
|domain                 |varchar(128)|YES |   |NULL   |     |
|project                |varchar(128)|YES |   |NULL   |     |
|region_id              |varchar(128)|YES |MUL|NULL   |     |
|region                 |varchar(128)|YES |   |NULL   |     |
|zone_id                |varchar(128)|YES |   |NULL   |     |
|zone                   |varchar(128)|YES |   |NULL   |     |
|currency               |varchar(8)  |YES |MUL|NULL   |     |
|spec                   |varchar(128)|YES |   |NULL   |     |
|mark                   |varchar(32) |YES |   |NULL   |     |
|progress_id            |varchar(128)|YES |MUL|NULL   |     |
|associate_id           |varchar(256)|YES |MUL|NULL   |     |
|cloudprovider_id       |varchar(128)|YES |MUL|NULL   |  n  |
|cloudprovider_name     |varchar(256)|YES |   |NULL   |  n  |
|external_project_id    |varchar(256)|YES |MUL|NULL   |  n  |
|external_project_name  |varchar(256)|YES |   |NULL   |  n  |
|linked                 |tinyint(1)  |NO  |MUL|0      |     |
|created_at             |datetime    |NO  |MUL|NULL   |     |
|updated_at             |datetime    |NO  |   |NULL   |     |
|update_version         |int(11)     |NO  |   |0      |     |
|deleted_at             |datetime    |YES |   |NULL   |     |
|deleted                |tinyint(1)  |NO  |   |0      |     |
+-----------------------+------------+----+---+-------+-----+
*/

type SBill struct {
	db.SResourceBase
	db.SProjectizedResourceBase

	Id                   string    `width:"128" charset:"ascii" primary:"true" list:"user" json:"id"`
	ResourceType         string    `width:"32" charset:"ascii" index:"true" list:"user" create:"domain_optional" update:"admin" json:"resource_type"`
	AccountId            string    `width:"128" charset:"ascii" index:"true" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"account_id"`
	Account              string    `width:"128" charset:"ascii" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"account"`
	Provider             string    `width:"32" charset:"ascii" list:"user" create:"domain_optional" update:"admin" json:"provider"`
	ChargeType           string    `width:"32" charset:"ascii" index:"true" list:"user" create:"domain_optional" update:"admin" json:"charge_type"`
	PaymentTime          time.Time `nullable:"yes" index:"true" list:"user" create:"domain_optional" json:"payment_time"`
	UsageStartTime       time.Time `nullable:"yes" list:"user" create:"domain_optional" json:"usage_start_time"`
	UsageEndTime         time.Time `nullable:"yes" list:"user" create:"domain_optional" json:"usage_end_time"`
	GrossAmount          float64   `default:"0" list:"user" create:"domain_optional" update:"admin" json:"gross_amount"`
	Amount               float64   `default:"0" list:"user" create:"domain_optional" update:"admin" json:"amount"`
	CashPayAmount        float64   `default:"0" list:"user" create:"domain_optional" update:"admin" json:"cash_pay_amount"`
	VoucherPayAmount     float64   `default:"0" list:"user" create:"domain_optional" update:"admin" json:"voucher_pay_amount"`
	PrepaidCardPayAmount float64   `default:"0" list:"user" create:"domain_optional" update:"admin" json:"prepaid_card_pay_amount"`
	IncentivePayAmount   float64   `default:"0" list:"user" create:"domain_optional" update:"admin" json:"incentive_pay_amount"`
	CouponPayAmount      float64   `default:"0" list:"user" create:"domain_optional" update:"admin" json:"coupon_pay_amount"`
	Discount             float64   `default:"0" list:"user" create:"domain_optional" update:"admin" json:"discount"`
	DiscountType         string    `default:"none" width:"32" charset:"ascii" list:"user" create:"domain_optional" update:"admin" json:"discount_type"`
	OutstandingAmount    float64   `default:"0" list:"user" create:"domain_optional" update:"admin" json:"outstanding_amount"`
	UsageType            string    `width:"32" charset:"ascii" list:"user" create:"domain_optional" update:"admin" json:"usage_type"`
	Rate                 float64   `default:"0" list:"user" create:"domain_optional" update:"admin" json:"rate"`
	PriceUnit            string    `width:"48" charset:"ascii" list:"user" create:"domain_optional" update:"admin" json:"price_unit"`
	Usage                float64   `default:"0" list:"user" create:"domain_optional" update:"admin" json:"usage"`
	DeductedUsage        float64   `default:"0" list:"user" create:"domain_optional" update:"admin" json:"deducted_usage"`
	Reserved             bool      `list:"user" create:"domain_optional" update:"admin" json:"reserved"`
	ResourceId           string    `width:"256" charset:"ascii" index:"true" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"resource_id"`
	ResourceName         string    `width:"256" charset:"ascii" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"resource_name"`
	Brand                string    `width:"32" charset:"ascii" index:"true" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"brand"`
	Domain               string    `width:"128" charset:"utf8" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"domain"`
	Project              string    `width:"128" charset:"utf8" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"project"`
	RegionId             string    `width:"128" charset:"ascii" index:"true" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"region_id"`
	Region               string    `width:"128" charset:"utf8" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"region"`
	ZoneId               string    `width:"128" charset:"ascii" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"zone_id"`
	Zone                 string    `width:"128" charset:"utf8" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"zone"`
	Currency             string    `width:"48" charset:"ascii" index:"true" list:"user" create:"domain_optional" update:"admin" json:"currency"`
	Spec                 string    `width:"128" charset:"ascii" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"spec"`
	Mark                 string    `width:"128" charset:"utf8" list:"user" update:"user" json:"mark"`
	Day                  int       `list:"user" update:"user" index:"true" json:"day"`
	Month                int       `list:"user" update:"user" index:"true" json:"month"`
	ProgressId           string    `width:"128" charset:"ascii" index:"true" list:"user" create:"domain_optional" update:"admin" json:"progress_id"`
	AssociateId          string    `width:"256" charset:"ascii" index:"true" list:"user" create:"domain_optional" update:"admin" json:"associate_id"`
	Linked               bool      `nullable:"false" list:"user" create:"domain_optional" update:"admin" json:"linked"`
}

func (manager *SBillManager) FilterById(q *sqlchemy.SQuery, idStr string) *sqlchemy.SQuery {
	return q.Equals("id", idStr)
}

func (manager *SBillManager) QueryDistinctExtraField(q *sqlchemy.SQuery, field string) (*sqlchemy.SQuery, error) {
	var err error

	q, err = manager.SResourceBaseManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}
	return q, httperrors.ErrNotFound
}

func (manager *SBillManager) NamespaceScope() rbacutils.TRbacScope {
	return rbacutils.ScopeProject
}

func (manager *SBillManager) ResourceScope() rbacutils.TRbacScope {
	return rbacutils.ScopeProject
}

func (manager *SBillManager) ListItemFilter(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	input api.BillListInput,
) (*sqlchemy.SQuery, error) {
	//q.DebugQuery()
	var err error
	q, err = manager.SResourceBaseManager.ListItemFilter(ctx, q, userCred, input.ResourceBaseListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SResourceBaseManager.ListItemFilter")
	}
	q, err = manager.SProjectizedResourceBaseManager.ListItemFilter(ctx, q, userCred, input.ProjectizedResourceListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SProjectizedResourceBaseManager.ListItemFilter")
	}
	// brand
	if len(input.Brand) > 0 {
		q.Equals("brand", input.Brand)
	}
	// res_type
	if len(input.ResType) > 0 {
		q.Equals("resource_type", input.ResType)
	}
	//project
	//if len(input.Project) > 0 {
	//	q.Equals("project", input.Project)
	//}
	// usage_start_time: 2022-08-31 00:00:00
	// usage_end_time: 2022-09-01 00:00:00
	if len(input.StartDate) > 0 {
		q.GE("usage_start_time", input.StartDate) //start_date: 2022-08-01TZ
	}
	if len(input.EndDate) > 0 {
		q.LE("usage_start_time", input.EndDate) //end_date: 2022-08-31TZ
	}
	if input.StartDay > 0 {
		q = q.GE("day", input.StartDay)
	}
	if input.EndDay > 0 {
		q = q.LE("day", input.EndDay)
	}
	if len(input.QueryGroup) > 0 && input.QueryGroup == QUERY_GROUP_RESOURCES && input.NeedGroup {
		q.GroupBy("resource_id")
		//q = q.AppendField(q.QueryFields()...)
		//q = q.AppendField(
		//	sqlchemy.SUM("total_gross_amount", q.Field("gross_amount")),
		//	sqlchemy.SUM("total_amount", q.Field("amount")),
		//	sqlchemy.SUM("total_usage", q.Field("usage"))).GroupBy("resource_id")
	}
	if len(input.QueryGroup) > 0 && input.QueryGroup == QUERY_GROUP_INSTANCES && input.NeedGroup {
		cond := sqlchemy.OR(sqlchemy.Equals(q.Field("resource_type"), RES_TYPE_BAREMETAL),
			sqlchemy.Equals(q.Field("resource_type"), RES_TYPE_SERVER))
		q = q.Filter(cond).GroupBy("resource_id")
		//q.Equals("resource_type", RES_TYPE_SERVER).GroupBy("resource_id")
		//q.NotEquals("associate_id", "")
		//q = q.AppendField(q.QueryFields()...)
		//q = q.AppendField(
		//	sqlchemy.SUM("gross_amount", q.Field("gross_amount")),
		//	sqlchemy.SUM("amount", q.Field("amount"))).GroupBy("associate_id")
	}
	if len(input.QueryType) > 0 && input.QueryType == QUERY_TYPE_EXPENSE_TREND && len(input.DataType) > 0 {
		q.GroupBy(input.DataType).Asc(input.DataType)
	}
	if len(input.QueryType) > 0 && (input.QueryType == QUERY_TYPE_CHARGE || input.QueryType == QUERY_TYPE_PROVIDER || input.QueryType == QUERY_TYPE_RESOURCE || input.QueryType == QUERY_TYPE_PROJECT) {
		q.GroupBy(input.QueryType).Desc("res_fee")
	}
	//log.Info("aaaaaaaaaaaaaaaaa")
	q.DebugQuery()

	return q, nil
}

func (manager *SBillManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []api.BillDetails {
	rows := make([]api.BillDetails, len(objs))
	queryGroup, _ := query.GetString("query_group")
	switch queryGroup {
	case QUERY_GROUP_INSTANCES:
		for i := range rows {
			bill := objs[i].(*SBill)
			rows[i] = api.BillDetails{
				ResId:   bill.ResourceId,
				ResName: bill.ResourceName,
				ResType: bill.ResourceType,
			}
			content, err := manager.getGroup(ctx, userCred, query, bill.ResourceId)
			if err != nil {
				log.Errorf("getGroup err %s", err)
				continue
			}
			rows[i].Content = content

			total, err := manager.getTotal(ctx, userCred, query, bill.ResourceId, "associate_id")
			if err != nil {
				log.Errorf("getTotal err %s", err)
				continue
			}
			rows[i].TotalAmount = total.Amount
		}
	case QUERY_GROUP_RESOURCES:
		for i := range rows {
			bill := objs[i].(*SBill)
			rows[i] = api.BillDetails{
				ResId:   bill.ResourceId,
				ResName: bill.ResourceName,
				ResType: bill.ResourceType,
			}
			total, err := manager.getTotal(ctx, userCred, query, bill.ResourceId, "resource_id")
			if err != nil {
				log.Errorf("getTotal err %s", err)
				continue
			}
			rows[i].TotalUsage = total.Usage
			rows[i].TotalAmount = total.Amount
		}
	default:
		for i := range rows {
			bill := objs[i].(*SBill)

			rows[i] = api.BillDetails{
				BillId:    bill.Id,
				ItemFee:   fmt.Sprintf("%.6f", bill.GrossAmount),
				ItemRate:  round(bill.Rate, 6),
				ResId:     bill.ResourceId,
				ResName:   bill.ResourceName,
				ResType:   bill.ResourceType,
				StartTime: bill.UsageStartTime.Format(TIME2_FORMAT),
				EndTime:   bill.UsageEndTime.Format(TIME2_FORMAT),
			}
		}
	}

	return rows
}

func (manager *SBillManager) getTotal(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, resId, groupBy string) (*STotal, error) {
	startDay, _ := query.Int("start_day")
	endDay, _ := query.Int("end_day")
	sq := manager.Query().SubQuery()
	q := sq.Query(sqlchemy.COUNT("total_count"),
		sqlchemy.SUM("amount", sq.Field("amount")),
		sqlchemy.SUM("usage", sq.Field("usage"))).Equals(groupBy, resId).GE("day", startDay).LE("day", endDay)

	total := STotal{}
	row := q.Row()
	err := q.Row2Struct(row, &total)
	total.Amount = round(total.Amount, 6)
	total.Usage = round(total.Usage, 6)
	return &total, err
}

func (manager *SBillManager) getGroup(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, resId string) ([]jsonutils.JSONObject, error) {
	startDay, _ := query.Int("start_day")
	endDay, _ := query.Int("end_day")
	//sq := manager.Query().SubQuery()
	//q := sq.Query(sqlchemy.COUNT("total_count"),
	//	sqlchemy.SUM("amount", sq.Field("gross_amount")),
	//	sqlchemy.SUM("usage", sq.Field("usage")),
	//	sqlchemy.SUM("res_fee", sq.Field("gross_amount"))).Equals("associate_id", resId).GE("day", startDay).LE("day", endDay)
	q := manager.Query()
	q = q.AppendField(q.QueryFields()...)
	q = q.AppendField(
		sqlchemy.SUM("total_amount", q.Field("amount")),
		sqlchemy.SUM("total_usage", q.Field("usage"))).Equals("associate_id", resId).GE("day", startDay).LE("day", endDay).GroupBy("resource_id").Desc("usage_type")

	content := make([]jsonutils.JSONObject, 0)
	rows, err := q.Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		item := SBillTotal{}
		err = q.Row2Struct(rows, &item)
		if err != nil {
			return nil, err
		}
		content = append(content, jsonutils.Marshal(item))
	}
	return content, err
}

type SBillTotal struct {
	SBill
	TotalAmount float64
	TotalUsage  float64
}

func (manager *SBillManager) OrderByExtraFields(
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

func (manager *SBillManager) GetExportExtraKeys(ctx context.Context, keys stringutils2.SSortedStrings, rowMap map[string]string) *jsonutils.JSONDict {
	res := manager.SResourceBaseManager.GetExportExtraKeys(ctx, keys, rowMap)
	// exportKeys, _ := query.GetString("export_keys")
	// keys := strings.Split(exportKeys, ",")
	if totalGrossAmount, ok := rowMap["total_gross_amount"]; ok {
		total, _ := strconv.ParseFloat(totalGrossAmount, 64)
		res.Set("total_gross_amount", jsonutils.NewFloat64(total))
	}
	if totalAmount, ok := rowMap["total_amount"]; ok {
		total, _ := strconv.ParseFloat(totalAmount, 64)
		res.Set("total_amount", jsonutils.NewFloat64(total))
	}
	if totalUsage, ok := rowMap["total_usage"]; ok {
		total, _ := strconv.ParseFloat(totalUsage, 64)
		res.Set("total_usage", jsonutils.NewFloat64(total))
	}
	//if keys.Contains("tenant") {
	//	if projectId, ok := rowMap["tenant_id"]; ok {
	//		tenant, err := db.TenantCacheManager.FetchTenantById(ctx, projectId)
	//		if err == nil {
	//			res.Set("tenant", jsonutils.NewString(tenant.GetName()))
	//		}
	//	}
	//}
	//if keys.Contains("os_distribution") {
	//	if osType, ok := rowMap["os_type"]; ok {
	//		res.Set("os_distribution", jsonutils.NewString(osType))
	//	}
	//}
	return res
}

func (m *SBillManager) VerifyExistingGuests(ctx context.Context, session *mcclient.ClientSession) *computeapi.ServerDetails {
	params := jsonutils.NewDict()
	params.Set("limit", jsonutils.NewInt(0))
	params.Set("scope", jsonutils.NewString("system"))
	params.Set("system", jsonutils.JSONTrue)
	params.Set("pending_delete", jsonutils.NewBool(false))
	params.Set("hypervisor", jsonutils.NewString("kvm"))
	//params.Set("get_all_guests_on_host", jsonutils.NewString(m.host.GetHostId()))
	//params.Set("filter.0", jsonutils.NewString(fmt.Sprintf("id.in(%s)", strings.Join(keys, ","))))
	res, err := compute.Servers.List(session, params)
	if err != nil {
		log.Warningf("test")
	} else {
		for _, v := range res.Data {
			log.Infof(v.String())
			serverDetail := new(computeapi.ServerDetails)
			err = res.Data[3].Unmarshal(serverDetail)
			if err != nil {
				log.Errorf("fail to unmarshal ServerDetails %v", err)
				return nil
			}
			return serverDetail
		}
	}
	return nil
}

func (manager *SBillManager) Compute(progressId string, billResource *SBillResource) (*api.BillCreateInput, error) {
	input := new(api.BillCreateInput)
	switch billResource.ResourceType {
	case RES_TYPE_SERVER:
		//cpuRate, err := manager.GetEffectiveRate(RES_TYPE_CPU, strings.Split(billResource.Model, ".")[1], time.Now())
		cpuRate, err := manager.GetEffectiveRate(RES_TYPE_CPU, billResource.UsageModel, time.Now())
		if err != nil {
			return nil, err
		}
		memRate, err := manager.GetEffectiveRate(RES_TYPE_MEM, "", time.Now())
		if err != nil {
			return nil, err
		}
		input.Rate = float64(billResource.Cpu)*cpuRate.Price + float64(billResource.Mem)*memRate.Price
		input.Spec = billResource.Model
		input.Usage = 24
		input.ResourceType = RES_TYPE_SERVER
		input.UsageType = RES_TYPE_INSTANCE
		input.PriceUnit = PRICE_UNIT_SERVER
	case RES_TYPE_BAREMETAL:
		baremetalRate, err := manager.GetEffectiveRate(RES_TYPE_BAREMETAL, billResource.Model, time.Now())
		if err != nil {
			return nil, err
		}
		input.Rate = baremetalRate.Price
		input.Spec = billResource.Model
		input.Usage = 24
		input.ResourceType = RES_TYPE_BAREMETAL
		input.UsageType = RES_TYPE_INSTANCE
		input.PriceUnit = PRICE_UNIT_SERVER
	case RES_TYPE_DISK:
		diskRate, err := manager.GetEffectiveRate(RES_TYPE_DISK, billResource.Model, time.Now())
		if err != nil {
			return nil, err
		}
		input.Rate = diskRate.Price
		input.Spec = billResource.Model + " " + strconv.Itoa(billResource.Size) + "GB"
		input.Usage = 24 * float64(billResource.Size)
		input.ResourceType = RES_TYPE_DISK
		input.UsageType = RES_TYPE_DISK
		input.PriceUnit = PRICE_UNIT_DISK
	}
	input.GrossAmount = input.Rate * input.Usage
	input.Amount = input.Rate * input.Usage
	input.ResourceName = billResource.ResourceName
	input.ProgressId = progressId
	input.ResourceId = billResource.ResourceId
	input.ProjectId = billResource.ProjectId
	input.Project = billResource.Project
	input.ZoneId = billResource.ZoneId
	input.Zone = billResource.Zone
	input.RegionId = billResource.RegionId
	input.Region = billResource.Region
	input.AssociateId = billResource.AssociateId

	return input, nil
}

func (manager *SBillManager) GetEffectiveRate(resType, model string, t time.Time) (*SRate, error) {
	q := RateManager.Query().Equals("resource_type", resType).Equals("model", model).Desc("enable_time").LE("enable_time", t)
	count, err := q.CountWithError()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, fmt.Errorf("no effective res price")
	}
	rate := new(SRate)
	err = q.First(rate)
	if err != nil {
		return rate, err
	}
	//log.Infof("%v", rate)
	return rate, nil
}

func (manager *SBillManager) CheckBill(ctx context.Context, userCred mcclient.TokenCredential, isStart bool) {
	//session := auth.GetAdminSession(ctx, "", "")
	t := time.Now()
	date, _ := strconv.Atoi(t.Add(-24 * time.Hour).Format(Date_Day_FORMAT))
	inProgress := ProgressManager.InDailyPullProgress(date)
	var progressId string
	if !inProgress {
		// 增加二次检查 check daily bill
		time.Sleep(5 * time.Second)
		inBill := BillManager.InDailyPullBill(date)
		if inBill {
			return
		}
		pId, err := ProgressManager.CreateDailyPullProgress(ctx, date)
		if err != nil {
			log.Errorf("CreateDailyPullProgress err %s", err)
			return
		}
		progressId = pId
	} else {
		return
	}

	//server := manager.VerifyExistingGuests(ctx, session)
	billResources, err := BillResourceManager.GetAllBillResources()
	if err != nil {
		log.Errorf("GetBillResources err %s", err)
		return
	}
	errs := make([]error, 0)
	for i := range billResources {
		input, err := manager.Compute(progressId, &billResources[i])
		if err != nil {
			log.Errorf("create compute input err", err)
			errs = append(errs, errors.Wrapf(err, "billResources:%s compute err", billResources[i].ResourceName))
			continue
		}
		err = manager.CreateBill(ctx, input, t)
		if err != nil {
			log.Errorf("create bill err", err)
			errs = append(errs, errors.Wrapf(err, "billResources:%s create bill err", billResources[i].ResourceName))
			continue
		}
	}
	//finish progress
	if len(errs) != 0 {
		log.Errorf("This cronJob about CheckBill finished with err count is %d", len(errs))
		misc := errors.NewAggregate(errs)
		err = ProgressManager.FinishDailyPullProgress(date, misc.Error(), len(errs))
		if err != nil {
			log.Errorf("FinishDailyPullProgress err %s", err)
			return
		}
	} else {
		log.Infof("This checkBill finished with %d billResources", len(billResources))
		err = ProgressManager.FinishDailyPullProgress(date, "", 0)
		if err != nil {
			log.Errorf("FinishDailyPullProgress err %s", err)
			return
		}
	}
}

func (manager *SBillManager) InDailyPullBill(date int) bool {
	q := manager.Query().Equals("day", date)
	count, err := q.CountWithError()
	if err != nil {
		log.Errorf("get daily bill err %s", err)
		return true
	}
	if count > 0 {
		log.Warningf("the bills of %d is in???", date)
		return true
	}
	return false
}

func (manager *SBillManager) CreateBill(ctx context.Context, input *api.BillCreateInput, t time.Time) error {
	currentTime := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	bill := &SBill{
		UsageStartTime: currentTime.AddDate(0, 0, -1),
		UsageEndTime:   currentTime,
		PaymentTime:    time.Now(),
	}
	bill.Id = db.DefaultUUIDGenerator()
	bill.Domain = "Default"
	bill.Currency = "CNY"
	bill.Brand = "OneCloud"
	bill.Provider = "OneCloud"
	bill.ChargeType = api.BILLING_TYPE_POSTPAID
	bill.Usage = round(input.Usage, 6)
	bill.UsageType = input.UsageType
	bill.ResourceType = input.ResourceType
	bill.ResourceId = input.ResourceId
	bill.ResourceName = input.ResourceName
	bill.Project = input.Project
	bill.ProjectId = input.ProjectId
	bill.ZoneId = input.ZoneId
	bill.Zone = input.Zone
	bill.RegionId = input.RegionId
	bill.Region = input.Region
	bill.Spec = input.Spec
	bill.ProgressId = input.ProgressId
	bill.AssociateId = input.AssociateId
	bill.Linked = true
	bill.Rate = round(input.Rate, 6)
	bill.GrossAmount = round(input.GrossAmount, 6)
	bill.Amount = round(input.Amount, 6)
	bill.PriceUnit = input.PriceUnit
	bill.Day, _ = strconv.Atoi(currentTime.Add(-24 * time.Hour).Format(Date_Day_FORMAT))
	bill.Month, _ = strconv.Atoi(currentTime.Add(-24 * time.Hour).Format(Date_Month_FORMAT))

	bill.SetModelManager(manager, bill)
	return manager.TableSpec().Insert(ctx, bill)
}

type STotal struct {
	Amount float64
	Usage  float64
	ResFee float64
}

// class action is manager?
//func (manager *SBillManager) PerformTotal(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) (jsonutils.JSONObject, error) {
//	//startDay, _ := data.Int("start_day")
//	//endDay, _ := data.Int("end_day")
//	sq := manager.Query().SubQuery()
//	q := sq.Query(sqlchemy.COUNT("total_count"),
//		sqlchemy.SUM("amount", sq.Field("amount")),
//		sqlchemy.SUM("res_fee", sq.Field("gross_amount")))
//
//	var err error
//	filterAny := false
//	filters := jsonutils.GetQueryStringArray(data, "filter")
//	if len(filters) > 0 {
//		q, err = db.ApplyListItemsGeneralFilters(manager, q, userCred, filters, filterAny)
//		if err != nil {
//			return nil, err
//		}
//	}
//
//	q, err = manager.ListItemFilter(ctx, q, userCred, query)
//	if err != nil {
//		return nil, err
//	}
//
//	queryGroup, _ := data.GetString("query_group")
//	if queryGroup == QUERY_GROUP_INSTANCES {
//		q.NotEquals("associate_id", "")
//	}
//
//	log.Info("jjjjjjjjjjjjjjjj")
//	q.DebugQuery()
//	total := STotal{}
//	row := q.Row()
//	err := q.Row2Struct(row, &total)
//	total.Amount = round(total.Amount, 6)
//	total.ResFee = round(total.ResFee, 6)
//	return jsonutils.Marshal(total), err
//}
//
func (manager *SBillManager) GetPropertyTotal(ctx context.Context, userCred mcclient.TokenCredential, query api.BillListInput) (jsonutils.JSONObject, error) {
	sq := manager.Query().SubQuery()
	q := sq.Query(sqlchemy.COUNT("total_count"),
		sqlchemy.SUM("amount", sq.Field("amount")),
		sqlchemy.SUM("res_fee", sq.Field("gross_amount")))

	if len(query.Scope) > 0 {
		q = manager.FilterByOwner(q, userCred, rbacutils.String2Scope(query.Scope))
	}
	var err error
	filterAny := false
	q, err = db.ApplyListItemsGeneralFilters(manager, q, userCred, query.Filter, filterAny)
	if err != nil {
		return nil, err
	}

	query.NeedGroup = false
	q, err = manager.ListItemFilter(ctx, q, userCred, query)
	if err != nil {
		return nil, err
	}

	// via listItemQueryFiltersRaw handle, is different from ListItemFilter
	if len(query.ProjectId) > 0 {
		q = q.In("tenant_id", query.ProjectId)
	}

	if query.QueryGroup == QUERY_GROUP_INSTANCES {
		q.NotEquals("associate_id", "")
	}

	total := STotal{}
	row := q.Row()
	err = q.Row2Struct(row, &total)
	total.Amount = round(total.Amount, 6)
	total.ResFee = round(total.ResFee, 6)
	return jsonutils.Marshal(total), err
}

type SBillResources struct {
	SBill
	TotalAmount float64
	TotalUsage  float64
}

type SBillOverview struct {
	Balance      int
	LastMonthFee string
	MonthFee     string
	YearFee      string
	RingRadio    string
}

// request id is 'billOverview', if request id is 'bill-overview', then property should be 'billOverview', sep is '-'.
func (manager *SBillManager) GetPropertyBilloverview(ctx context.Context, userCred mcclient.TokenCredential, query api.BillListInput) (jsonutils.JSONObject, error) {
	now := time.Now()
	firstDateTime := now.AddDate(0, 0, -now.Day()+1)
	//firstDateZeroTime := time.Date(firstDateTime.Year(), firstDateTime.Month(), firstDateTime.Day(), 0, 0, 0, 0, firstDateTime.Location())
	//lastDateTime := now.AddDate(0, 1, -now.Day())
	//lastDateZeroTime := time.Date(lastDateTime.Year(), lastDateTime.Month(), lastDateTime.Day(), 0, 0, 0, 0, firstDateTime.Location())

	monthFee, err := manager.getFee(ctx, userCred, query)
	if err != nil {
		log.Errorf("get month fee err %s", err)
		return nil, err
	}

	lastMonthFee := 0.00
	ringRadio := 0.00
	if firstDateTime.Format(Date_FORMAT) == query.StartDate && now.Format(Date_FORMAT) == query.EndDate {
		query.StartDate = firstDateTime.AddDate(0, -1, 0).Format(Date_FORMAT)
		query.EndDate = now.AddDate(0, -1, -1).Format(Date_FORMAT)
		lastMonthFee, err = manager.getFee(ctx, userCred, query)
		if err != nil {
			log.Errorf("get last month fee err %s", err)
			return nil, err
		}
		if lastMonthFee == 0.00 {
			ringRadio = monthFee
		} else {
			ringRadio = (monthFee - lastMonthFee) / lastMonthFee
		}
	}

	query.StartDate = firstDateTime.AddDate(0, -int(firstDateTime.Month())+1, 0).Format(Date_FORMAT)
	query.EndDate = now.Format(Date_FORMAT)
	yearFee, err := manager.getFee(ctx, userCred, query)
	if err != nil {
		log.Errorf("get year fee err %s", err)
		return nil, err
	}

	view := SBillOverview{}

	view.LastMonthFee = fmt.Sprintf("%.6f", lastMonthFee)
	view.MonthFee = fmt.Sprintf("%.6f", monthFee)
	view.YearFee = fmt.Sprintf("%.6f", yearFee)
	view.RingRadio = fmt.Sprintf("%.6f", ringRadio)
	view.Balance = 0
	return jsonutils.Marshal(view), err
}

func (manager *SBillManager) getFee(ctx context.Context, userCred mcclient.TokenCredential, query api.BillListInput) (float64, error) {
	sq := manager.Query().SubQuery()
	q := sq.Query(sqlchemy.SUM("amount", sq.Field("amount")))

	if len(query.Scope) > 0 {
		q = manager.FilterByOwner(q, userCred, rbacutils.String2Scope(query.Scope))
	}
	var err error
	filterAny := false
	q, err = db.ApplyListItemsGeneralFilters(manager, q, userCred, query.Filter, filterAny)
	if err != nil {
		return -1, err
	}

	query.NeedGroup = false
	q, err = manager.ListItemFilter(ctx, q, userCred, query)
	if err != nil {
		return -1, err
	}

	// via listItemQueryFiltersRaw handle, is different from ListItemFilter
	if len(query.ProjectId) > 0 {
		q = q.In("tenant_id", query.ProjectId)
	}

	if query.QueryGroup == QUERY_GROUP_INSTANCES {
		q.NotEquals("associate_id", "")
	}

	row := q.Row()
	var fee sql.NullFloat64
	err = row.Scan(&fee)
	if err != nil {
		return -1, err
	}

	return fee.Float64, nil
}

func (manager *SBillManager) GetPropertyExpenseTrend(ctx context.Context, userCred mcclient.TokenCredential, query api.BillListInput) (jsonutils.JSONObject, error) {
	statList, err := manager.getFeeList(ctx, userCred, query)
	if err != nil {
		log.Errorf("get fee list err %s", err)
		return nil, err
	}
	return jsonutils.Marshal(statList), err
}

type SExpenseStat struct {
	Date      int
	StatDate  string
	StatValue float64
}

func (manager *SBillManager) getFeeList(ctx context.Context, userCred mcclient.TokenCredential, query api.BillListInput) ([]jsonutils.JSONObject, error) {
	sq := manager.Query().SubQuery()
	q := sq.Query(sqlchemy.SUM("stat_value", sq.Field("amount")), sq.Field(query.DataType, "date"))

	if len(query.Scope) > 0 {
		q = manager.FilterByOwner(q, userCred, rbacutils.String2Scope(query.Scope))
	}
	var err error
	filterAny := false
	q, err = db.ApplyListItemsGeneralFilters(manager, q, userCred, query.Filter, filterAny)
	if err != nil {
		return nil, err
	}

	query.NeedGroup = false
	q, err = manager.ListItemFilter(ctx, q, userCred, query)
	if err != nil {
		return nil, err
	}

	// via listItemQueryFiltersRaw handle, is different from ListItemFilter
	if len(query.ProjectId) > 0 {
		q = q.In("tenant_id", query.ProjectId)
	}

	if query.QueryGroup == QUERY_GROUP_INSTANCES {
		q.NotEquals("associate_id", "")
	}

	rows, err := q.Rows()
	if err != nil {
		if errors.Cause(err) != sql.ErrNoRows {
			log.Errorf("query fee list fail %s", err)
		}
		return nil, err
	}
	defer rows.Close()

	statList := make([]jsonutils.JSONObject, 0)
	for rows.Next() {
		stat := SExpenseStat{}
		err = q.Row2Struct(rows, &stat)
		if err != nil {
			log.Errorf("Row2Struct fail %s", err)
			return nil, err
		}
		dateStr := strconv.Itoa(stat.Date)
		if query.DataType == DATA_TYPE_DAY {
			stat.StatDate = fmt.Sprintf("%s-%s-%sTZ", dateStr[:4], dateStr[4:6], dateStr[6:])
		} else {
			stat.StatDate = fmt.Sprintf("%s-%sTZ", dateStr[:4], dateStr[4:])
		}
		statList = append(statList, jsonutils.Marshal(stat))
	}

	return statList, nil
}

func (manager *SBillManager) GetPropertyChargeType(ctx context.Context, userCred mcclient.TokenCredential, query api.BillListInput) (jsonutils.JSONObject, error) {
	itemList, err := manager.getFeeGroup(ctx, userCred, query)
	if err != nil {
		log.Errorf("get fee group err %s", err)
		return nil, err
	}
	return jsonutils.Marshal(itemList), err
}

func (manager *SBillManager) GetPropertyResourceType(ctx context.Context, userCred mcclient.TokenCredential, query api.BillListInput) (jsonutils.JSONObject, error) {
	itemList, err := manager.getFeeGroup(ctx, userCred, query)
	if err != nil {
		log.Errorf("get fee group err %s", err)
		return nil, err
	}
	return jsonutils.Marshal(itemList), err
}

func (manager *SBillManager) GetPropertyProvider(ctx context.Context, userCred mcclient.TokenCredential, query api.BillListInput) (jsonutils.JSONObject, error) {
	itemList, err := manager.getFeeGroup(ctx, userCred, query)
	if err != nil {
		log.Errorf("get fee group err %s", err)
		return nil, err
	}
	return jsonutils.Marshal(itemList), err
}

func (manager *SBillManager) GetPropertyProject(ctx context.Context, userCred mcclient.TokenCredential, query api.BillListInput) (jsonutils.JSONObject, error) {
	itemList, err := manager.getFeeGroup(ctx, userCred, query)
	if err != nil {
		log.Errorf("get fee group err %s", err)
		return nil, err
	}
	return jsonutils.Marshal(itemList), err
}

func (manager *SBillManager) getFeeGroup(ctx context.Context, userCred mcclient.TokenCredential, query api.BillListInput) ([]jsonutils.JSONObject, error) {
	sq := manager.Query().SubQuery()
	q := sq.Query(sqlchemy.SUM("res_fee", sq.Field("amount")), sq.Field(query.QueryType, "item_name"))

	if len(query.Scope) > 0 {
		q = manager.FilterByOwner(q, userCred, rbacutils.String2Scope(query.Scope))
	}
	var err error
	filterAny := false
	q, err = db.ApplyListItemsGeneralFilters(manager, q, userCred, query.Filter, filterAny)
	if err != nil {
		return nil, err
	}

	query.NeedGroup = false
	q, err = manager.ListItemFilter(ctx, q, userCred, query)
	if err != nil {
		return nil, err
	}

	rows, err := q.Rows()
	if err != nil {
		if errors.Cause(err) != sql.ErrNoRows {
			log.Errorf("query fee group fail %s", err)
		}
		return nil, err
	}
	defer rows.Close()

	itemList := make([]jsonutils.JSONObject, 0)
	for rows.Next() {
		item := SItemFee{}
		err = q.Row2Struct(rows, &item)
		if err != nil {
			log.Errorf("Row2Struct fail %s", err)
			return nil, err
		}
		itemList = append(itemList, jsonutils.Marshal(item))
	}

	return itemList, nil
}

type SItemFee struct {
	ItemName string
	ResFee   float64
}
