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
	"math"
	"strconv"
	"strings"
	"time"
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	api "yunion.io/x/onecloud/pkg/apis/billing"
	"yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/util/stringutils2"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/sortedmap"
	"yunion.io/x/sqlchemy"
)

const (
	DISK_ROTATE_LOCAL = "rotate::local"
	DISK_SSD_LOCAL    = "ssd::local"
	DISK_SSD_RBD      = "ssd::rbd"
	DISK_ROTATE_RBD   = "rotate::rbd"
	DISK_HYBRID_RBD   = "hybrid::rbd"
	Date_FORMAT       = "2006-01-02TZ"
	Date_Day_FORMAT   = "20060102"
	Date_Month_FORMAT = "200601"
	TIME_FORMAT       = "2006-01-02T15:04:05.000000Z"
	TIME2_FORMAT      = "2006-01-02T15:04:05Z"

	// Action
	ACTION_QUERY_HSITORY = "queryhistory"
	ACTION_QUERY_GROUP   = "querygroup"

	// Action
	RES_TYPE_ALL        = "all"
	RES_TYPE_SERVER     = "server"
	RES_TYPE_INSTANCE   = "instance"
	RES_TYPE_VM         = "vm"
	RES_TYPE_BAREMETAL  = "baremetal"
	RES_TYPE_CPU        = "cpu"
	RES_TYPE_MEM        = "mem"
	RES_TYPE_DISK       = "disk"
	RES_TYPE_FILESYSTEM = "filesystem"
	RES_TYPE_BUCKET     = "bucket"
	RES_TYPE_EIP        = "eip"
)

// no order
var resMap = map[string]string{
	"cpu":        "g1,c1,r1",
	"mem":        "",
	"disk":       "rotate::local,ssd::local,ssd::rbd,rotate::rbd,hybrid::rbd",
	"filesystem": "standard,performance,capacity",
	"bucket":     "data,cold,archived",
}

// +onecloud:swagger-gen-ignore
type SRateManager struct {
	db.SResourceBaseManager
}

var RateManager *SRateManager

func init() {
	RateManager = &SRateManager{
		SResourceBaseManager: db.NewResourceBaseManager(
			SRate{},
			"rates_tbl",
			"rate",
			"rates",
		),
	}
	RateManager.SetVirtualObject(RateManager)
}

/*
+-----------------+--------------+------+-----+---------+-------+
| Field           | Type         | Null | Key | Default | Extra |
+-----------------+--------------+------+-----+---------+-------+
| created_at      | datetime     | NO   | MUL | NULL    |       |
| updated_at      | datetime     | NO   |     | NULL    |       |
| update_version  | int(11)      | NO   |     | 0       |       |
| deleted_at      | datetime     | YES  |     | NULL    |       |
| deleted         | tinyint(1)   | NO   |     | 0       |       |
| id              | varchar(128) | NO   | PRI | NULL    |       |
| resource_type   | varchar(64)  | YES  | MUL | NULL    |       |
| model           | varchar(256) | YES  | MUL | NULL    |       |
| brand           | varchar(32)  | YES  | MUL | NULL    |       |
| price_type      | varchar(32)  | YES  |     | amount  |       |
| price           | double       | YES  |     | 0       |       |
| domain_id       | varchar(128) | YES  | MUL | default |       |
| cloudaccount_id | varchar(256) | YES  | MUL | NULL    |       |
| remark          | varchar(256) | YES  |     | NULL    |       |
| enable_time     | datetime     | YES  | MUL | NULL    |       |
| misc            | text         | YES  |     | NULL    |       |
+-----------------+--------------+------+-----+---------+-------+
*/

type SRate struct {
	db.SResourceBase

	Id             string    `width:"128" charset:"ascii" primary:"true" list:"user" json:"id"`
	ResourceType   string    `width:"64" charset:"ascii" index:"true" list:"user" create:"domain_optional" update:"admin" json:"resource_type"`
	Model          string    `width:"256" charset:"utf8" index:"true" list:"user" create:"domain_optional" update:"admin" json:"model"`
	Brand          string    `width:"32" charset:"ascii" index:"true" list:"user" update:"user" json:"brand"`
	PriceType      string    `width:"32" charset:"ascii" default:"amount" list:"user" create:"domain_optional" update:"admin" json:"price_type"`
	Price          float64   `default:"0" list:"user" create:"domain_optional" update:"admin" json:"price"`
	CloudaccountId string    `width:"256" charset:"ascii" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"cloudaccount_id"`
	Remark         string    `width:"256" charset:"utf8" list:"user" update:"user" json:"remark"`
	EnableTime     time.Time `nullable:"false" list:"user" create:"domain_optional" update:"admin" json:"enable_time"`
	Misc           string    `length:"text" nullable:"true"`
}

func (manager *SRateManager) ListItemFilter(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	input api.RateListInput,
) (*sqlchemy.SQuery, error) {
	var err error
	q, err = manager.SResourceBaseManager.ListItemFilter(ctx, q, userCred, input.ResourceBaseListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SResourceBaseManager.ListItemFilter")
	}
	// res_type
	if input.ResType == RES_TYPE_VM {
		q.NotEquals("resource_type", RES_TYPE_BAREMETAL)
	} else if input.ResType == RES_TYPE_BAREMETAL {
		q.Equals("resource_type", RES_TYPE_BAREMETAL)
	} else {
		q.Equals("resource_type", input.ResType)
	}
	// action
	if input.Action == ACTION_QUERY_GROUP || input.Action == ACTION_QUERY_HSITORY {
		//q.LE("enable_time", time.Now())
		q.Desc("enable_time")
	} else {
		return nil, fmt.Errorf("no support action %s", input.Action)
	}
	// model
	if len(input.Model) > 0 {
		q.Equals("model", input.Model)
	}
	if input.Action == ACTION_QUERY_GROUP && input.ResType == RES_TYPE_BAREMETAL {
		q.GroupBy("model")
	}

	return q, nil
}

func (manager *SRateManager) FilterById(q *sqlchemy.SQuery, idStr string) *sqlchemy.SQuery {
	return q.Equals("id", idStr)
}

func (manager *SRateManager) InitializeData() error {
	q := manager.Query()
	rates := []SRate{}
	err := db.FetchModelObjects(manager, q, &rates)
	if err != nil {
		return errors.Wrapf(err, "db.FetchModelObjects")
	}
	if len(rates) == 0 {
		sortedmap.NewSortedMap()
		for res, models := range resMap {
			switch res {
			case RES_TYPE_CPU:
				for _, model := range strings.Split(models, ",") {
					rate := &SRate{}
					rate.SetModelManager(manager, rate)
					rate.Id = db.DefaultUUIDGenerator()
					rate.Model = model
					rate.ResourceType = res
					rate.Brand = "Default"
					rate.PriceType = "amount"
					if model == "c1" {
						rate.Price = 0.001
					} else if model == "r1" {
						rate.Price = 0.002
					} else {
						rate.Price = 0.004
					}
					//rate.EnableTime = time.Now().UTC()
					rate.EnableTime, _ = time.Parse(Date_FORMAT, time.Now().Format(Date_FORMAT))
					_ = manager.TableSpec().Insert(context.TODO(), rate)
				}
			case RES_TYPE_DISK:
				for _, model := range strings.Split(models, ",") {
					rate := &SRate{}
					rate.SetModelManager(manager, rate)
					rate.Id = db.DefaultUUIDGenerator()
					rate.Model = model
					rate.ResourceType = res
					rate.Brand = "Default"
					rate.PriceType = "amount"
					switch model {
					case DISK_ROTATE_LOCAL:
						rate.Price = 0.00
					case DISK_SSD_LOCAL:
						rate.Price = 0.00
					case DISK_ROTATE_RBD:
						rate.Price = 0.00003787
					case DISK_SSD_RBD:
						rate.Price = 0.000194979
					case DISK_HYBRID_RBD:
						rate.Price = 0.00005787
					}
					rate.EnableTime, _ = time.Parse(Date_FORMAT, time.Now().Format(Date_FORMAT))
					_ = manager.TableSpec().Insert(context.TODO(), rate)
				}
			case RES_TYPE_MEM:
				rate := &SRate{}
				rate.SetModelManager(manager, rate)
				rate.Id = db.DefaultUUIDGenerator()
				rate.Model = ""
				rate.ResourceType = res
				rate.Brand = "Default"
				rate.PriceType = "amount"
				rate.Price = 0.004
				//rate.EnableTime = time.Now().UTC()
				rate.EnableTime, _ = time.Parse(Date_FORMAT, time.Now().Format(Date_FORMAT))
				_ = manager.TableSpec().Insert(context.TODO(), rate)
			}
		}
	} else {
		for res, models := range resMap {
			switch res {
			case RES_TYPE_CPU:
				for _, model := range strings.Split(models, ",") {
					count, err := manager.GetResourceModelCount(res, model)
					if err != nil {
						log.Errorf("get resource %s model %s count err %s", res, model, err)
						continue
					}
					if count > 0 {
						continue
					}

					rate := &SRate{}
					rate.SetModelManager(manager, rate)
					rate.Id = db.DefaultUUIDGenerator()
					rate.Model = model
					rate.ResourceType = res
					rate.Brand = "Default"
					rate.PriceType = "amount"
					if model == "c1" {
						rate.Price = 0.001
					} else if model == "r1" {
						rate.Price = 0.002
					} else {
						rate.Price = 0.004
					}
					//rate.EnableTime = time.Now().UTC()
					rate.EnableTime, _ = time.Parse(Date_FORMAT, time.Now().Format(Date_FORMAT))
					_ = manager.TableSpec().Insert(context.TODO(), rate)
				}
			case RES_TYPE_DISK:
				for _, model := range strings.Split(models, ",") {
					count, err := manager.GetResourceModelCount(res, model)
					if err != nil {
						log.Errorf("get resource %s model %s count err %s", res, model, err)
						continue
					}
					if count > 0 {
						continue
					}

					rate := &SRate{}
					rate.SetModelManager(manager, rate)
					rate.Id = db.DefaultUUIDGenerator()
					rate.Model = model
					rate.ResourceType = res
					rate.Brand = "Default"
					rate.PriceType = "amount"
					switch model {
					case DISK_ROTATE_LOCAL:
						rate.Price = 0.00
					case DISK_SSD_LOCAL:
						rate.Price = 0.00
					case DISK_ROTATE_RBD:
						rate.Price = 0.00003787
					case DISK_SSD_RBD:
						rate.Price = 0.000194979
					case DISK_HYBRID_RBD:
						rate.Price = 0.00005787
					}
					rate.EnableTime, _ = time.Parse(Date_FORMAT, time.Now().Format(Date_FORMAT))
					_ = manager.TableSpec().Insert(context.TODO(), rate)
				}
			case RES_TYPE_FILESYSTEM:
				for _, model := range strings.Split(models, ",") {
					count, err := manager.GetResourceModelCount(RES_TYPE_FILESYSTEM, model)
					if err != nil {
						log.Errorf("get resource %s model %s count err %s", RES_TYPE_FILESYSTEM, model, err)
						continue
					}
					if count > 0 {
						continue
					}

					rate := &SRate{}
					rate.SetModelManager(manager, rate)
					rate.Id = db.DefaultUUIDGenerator()
					rate.Model = model
					rate.ResourceType = res
					rate.Brand = "Default"
					rate.PriceType = "amount"
					switch model {
					case compute.NAS_STORAGE_TYPE_STANDARD:
						rate.Price = 0.000059651
					case compute.NAS_STORAGE_TYPE_PERFORMANCE:
						rate.Price = 0.00019854
					case compute.NAS_STORAGE_TYPE_CAPACITY:
						rate.Price = 0.000043081
					}
					rate.EnableTime, _ = time.Parse(Date_FORMAT, time.Now().Format(Date_FORMAT))
					_ = manager.TableSpec().Insert(context.TODO(), rate)
				}
			case RES_TYPE_BUCKET:
				for _, model := range strings.Split(models, ",") {
					count, err := manager.GetResourceModelCount(RES_TYPE_BUCKET, model)
					if err != nil {
						log.Errorf("get resource %s model %s count err %s", RES_TYPE_BUCKET, model, err)
						continue
					}
					if count > 0 {
						continue
					}

					rate := &SRate{}
					rate.SetModelManager(manager, rate)
					rate.Id = db.DefaultUUIDGenerator()
					rate.Model = model
					rate.ResourceType = res
					rate.Brand = "Default"
					rate.PriceType = "amount"
					switch model {
					case "data":
						rate.Price = 0.000059651
					case "cold":
						rate.Price = 0.000043081
					case "archived":
						rate.Price = 0.000031283
					}
					rate.EnableTime, _ = time.Parse(Date_FORMAT, time.Now().Format(Date_FORMAT))
					_ = manager.TableSpec().Insert(context.TODO(), rate)
				}
			case RES_TYPE_MEM:
				count, err := manager.GetResourceModelCount(res, "")
				if err != nil {
					log.Errorf("get resource %s model %s count err %s", res, "", err)
					continue
				}
				if count > 0 {
					continue
				}

				rate := &SRate{}
				rate.SetModelManager(manager, rate)
				rate.Id = db.DefaultUUIDGenerator()
				rate.Model = ""
				rate.ResourceType = res
				rate.Brand = "Default"
				rate.PriceType = "amount"
				rate.Price = 0.004
				//rate.EnableTime = time.Now().UTC()
				rate.EnableTime, _ = time.Parse(Date_FORMAT, time.Now().Format(Date_FORMAT))
				_ = manager.TableSpec().Insert(context.TODO(), rate)
			}
		}
	}
	return nil
}

func (manager *SRateManager) GetResourceModelCount(resource, model string) (int, error) {
	q := manager.Query().Equals("resource_type", resource).Equals("model", model)
	//q = q.Filter(
	//	sqlchemy.OR(
	//		sqlchemy.Equals(q.Field("name"), name),
	//		sqlchemy.Equals(q.Field("id"), name),
	//	),
	//)
	return q.CountWithError()
}

func (manager *SRateManager) ValidateCreateData(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	ownerId mcclient.IIdentityProvider,
	query jsonutils.JSONObject,
	input api.RateCreateInput,
) (api.RateCreateInput, error) {
	var err error
	var output api.RateCreateInput
	err = manager.ValidateUniqValues(ctx, input)
	if err != nil {
		return output, errors.Wrap(err, "ResourceBaseCreateInput.ValidateUniqValues")
	}
	output.ResourceBaseCreateInput, err = manager.SResourceBaseManager.ValidateCreateData(ctx, userCred, ownerId, query, output.ResourceBaseCreateInput)
	if err != nil {
		return output, errors.Wrap(err, "ResourceBaseCreateInput.ValidateCreateData")
	}

	//output.ProjectizedResourceCreateInput.ProjectDomainId, err = manager.SProjectizedResourceBaseManager.(ctx, userCred, ownerId, query, output.EnabledStatusStandaloneResourceCreateInput)
	//if err != nil {
	//	return output, errors.Wrap(err, "EnabledStatusStandaloneResourceCreateInput.ValidateCreateData")
	//}
	return output, nil
}

func (manager *SRateManager) ValidateUniqValues(ctx context.Context, input api.RateCreateInput) error {
	if len(input.RequestType) > 0 && input.RequestType == "create_baremetal" {
		return manager.ValidateUniqBaremetals(ctx, input)
	}
	enableTime, err := time.Parse(Date_FORMAT, input.EffectiveDate)
	if err != nil {
		return errors.Wrap(err, "time.Parse err")
	}

	q := manager.Query().Equals("resource_type", input.ResType).Equals("enable_time", enableTime)
	if len(input.Model) > 0 {
		q.Equals("model", input.Model)
	}
	if len(input.MediumType) > 0 && len(input.StorageType) > 0 {
		q.Equals("model", input.MediumType+"::"+input.StorageType)
	}
	count, err := q.CountWithError()
	if err != nil {
		return errors.Wrap(err, "get count err")
	}
	if count > 0 {
		return sqlchemy.ErrDuplicateEntry
	}
	return nil
}

func (manager *SRateManager) ValidateUniqBaremetals(ctx context.Context, input api.RateCreateInput) error {
	q := manager.Query().Equals("resource_type", input.ResType)
	manufacture := strings.ReplaceAll(input.Manufacture, " ", "_")
	model := strings.ReplaceAll(input.Model, " ", "_")
	//cpu:64/mem:128473M/manufacture:Powerleader/model:PR1710P
	spec := fmt.Sprintf("cpu:%d/mem:%dM/manufacture:%s/model:%s", input.CpuCoreCount, input.MemorySizeMb, manufacture, model)
	q.Equals("model", spec)
	count, err := q.CountWithError()
	if err != nil {
		return errors.Wrap(err, "get count err")
	}
	if count > 0 {
		return sqlchemy.ErrDuplicateEntry
	}
	return nil
}

func (manager *SRateManager) QueryDistinctExtraField(q *sqlchemy.SQuery, field string) (*sqlchemy.SQuery, error) {
	var err error

	q, err = manager.SResourceBaseManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}
	return q, httperrors.ErrNotFound
}

func (manager *SRateManager) FetchCustomizeColumns(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	objs []interface{},
	fields stringutils2.SSortedStrings,
	isList bool,
) []api.RateDetails {
	rows := make([]api.RateDetails, len(objs))

	//baseRows := manager.SResourceBaseManager.FetchCustomizeColumns(ctx, userCred, query, objs, fields, isList)
	//s := auth.GetAdminSession(context.Background(), options.Options.Region, "")
	//recordIds := make([]string, len(objs))
	for i := range rows {
		rate := objs[i].(*SRate)

		var unit string
		switch rate.ResourceType {
		case RES_TYPE_CPU:
			unit = "1core"
		case RES_TYPE_MEM, RES_TYPE_DISK:
			unit = "1GB"
		case RES_TYPE_EIP:
			unit = "Mbps"
		default:
			unit = ""
		}

		rows[i] = api.RateDetails{
			Id:            rate.Id,
			EffectiveDate: rate.EnableTime.Format(Date_FORMAT),
			Duration:      "day",
			Unit:          unit,
			Model:         rate.Model,
			RateText:      strconv.FormatFloat(rate.Price*24, 'f', 6, 64),
			ResType:       rate.ResourceType,
		}
	}
	return rows
}

func (manager *SRateManager) OrderByExtraFields(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	query api.RateListInput,
) (*sqlchemy.SQuery, error) {
	var err error

	q, err = manager.SResourceBaseManager.OrderByExtraFields(ctx, q, userCred, query.ResourceBaseListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SResourceBaseManager.OrderByExtraFields")
	}

	return q, nil
}

func round(f float64, n int) float64 {
	pow10_n := math.Pow10(n)
	return math.Trunc((f+0.5/pow10_n)*pow10_n) / pow10_n
}

func (rate *SRate) CustomizeCreate(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data jsonutils.JSONObject) error {
	var input api.RateCreateInput
	err := data.Unmarshal(&input)
	if err != nil {
		return err
	}
	rate.Id = db.DefaultUUIDGenerator()
	if len(input.RequestType) > 0 && input.RequestType == "create_baremetal" {
		manufacture := strings.ReplaceAll(input.Manufacture, " ", "_")
		model := strings.ReplaceAll(input.Model, " ", "_")
		//cpu:64/mem:128473M/manufacture:Powerleader/model:PR1710P
		spec := fmt.Sprintf("cpu:%d/mem:%dM/manufacture:%s/model:%s", input.CpuCoreCount, input.MemorySizeMb, manufacture, model)
		rate.Model = spec
		//rate.EnableTime = time.Now().UTC()
	}
	if len(input.Model) > 0 && input.RequestType == "set" {
		rate.Model = input.Model
	}
	if len(input.MediumType) > 0 && len(input.StorageType) > 0 {
		rate.Model = input.MediumType + "::" + input.StorageType
	}
	rate.ResourceType = input.ResType
	rate.Brand = "Default"
	rate.PriceType = "amount"
	price, err := strconv.ParseFloat(input.RateText, 64)
	if err != nil {
		return err
	}
	rate.Price = round(price/24, 6)
	if len(input.EffectiveDate) > 0 {
		enableTime, err := time.Parse(Date_FORMAT, input.EffectiveDate)
		if err != nil {
			return errors.Wrap(err, "time.Parse err")
		}
		rate.EnableTime = enableTime.UTC()
	} else {
		enableTime, err := time.Parse(Date_FORMAT, time.Now().Format(Date_FORMAT))
		if err != nil {
			return errors.Wrap(err, "time.Parse err")
		}
		rate.EnableTime = enableTime.UTC()
	}
	return nil
}

func (rate *SRate) ValidateUpdateData(
	ctx context.Context,
	userCred mcclient.TokenCredential,
	query jsonutils.JSONObject,
	input api.RateUpdateInput,
) (api.RateUpdateInput, error) {
	var err error
	if len(input.RateText) > 0 {
		price, err := strconv.ParseFloat(input.RateText, 64)
		if err != nil {
			return input, httperrors.NewInputParameterError("invalid rate parse float (%s): %s", input.RateText, err)
		}
		input.Price = round(price/24, 6)
	}
	if len(input.EffectiveDate) > 0 {
		enableTime, err := time.Parse(Date_FORMAT, input.EffectiveDate)
		if err != nil {
			return input, httperrors.NewInputParameterError("invalid time (%s): %s", input.EffectiveDate, err)
		}
		input.EnableTime = enableTime.UTC()
	}
	// brand is changed, so make it default
	input.Brand = "Default"
	input.ResourceBaseUpdateInput, err = rate.SResourceBase.ValidateUpdateData(ctx, userCred, query, input.ResourceBaseUpdateInput)
	if err != nil {
		return input, errors.Wrap(err, "SResourceBase.ValidateUpdateData")
	}
	return input, nil
}

//func (instance *SWorkflowProcessInstance) StartBpmProcessSubmitTask(ctx context.Context, userCred mcclient.TokenCredential) error {
//	task, err := taskman.TaskManager.NewTask(ctx, "BpmProcessSubmitTask", instance, userCred, nil, "", "")
//	if err != nil {
//		return err
//	}
//	instance.SetState(SUBMITTING)
//	task.ScheduleRun(nil)
//	return nil
//}
//
//func (instance *SWorkflowProcessInstance) StartMachineApplyTask(ctx context.Context, userCred mcclient.TokenCredential) error {
//	task, err := taskman.TaskManager.NewTask(ctx, "MachineApplyTask", instance, userCred, nil, "", "")
//	if err != nil {
//		return err
//	}
//	task.ScheduleRun(nil)
//	return nil
//}
//
//func (instance *SWorkflowProcessInstance) StartMachineChangeConfigTask(ctx context.Context, userCred mcclient.TokenCredential) error {
//	task, err := taskman.TaskManager.NewTask(ctx, "MachineChangeConfigTask", instance, userCred, nil, "", "")
//	if err != nil {
//		return err
//	}
//	task.ScheduleRun(nil)
//	return nil
//}
//
//func (instance *SWorkflowProcessInstance) StartMachineApplyRetryTask(ctx context.Context, userCred mcclient.TokenCredential, data jsonutils.JSONObject) error {
//	task, err := taskman.TaskManager.NewTask(ctx, "MachineApplyRetryTask", instance, userCred, nil, "", "")
//	if err != nil {
//		return err
//	}
//
//	task.ScheduleRun(data)
//	return nil
//}
//
//func (instance *SWorkflowProcessInstance) StartBucketApplyTask(ctx context.Context, userCred mcclient.TokenCredential) error {
//	task, err := taskman.TaskManager.NewTask(ctx, "BucketApplyTask", instance, userCred, nil, "", "")
//	if err != nil {
//		return err
//	}
//	task.ScheduleRun(nil)
//	return nil
//}
//
//func (instance *SWorkflowProcessInstance) StartLoadbalancerApplyTask(ctx context.Context, userCred mcclient.TokenCredential) error {
//	task, err := taskman.TaskManager.NewTask(ctx, "LoadbalancerApplyTask", instance, userCred, nil, "", "")
//	if err != nil {
//		return err
//	}
//	task.ScheduleRun(nil)
//	return nil
//}
//
