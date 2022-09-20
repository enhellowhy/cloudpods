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
	"time"
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/apis"
	api "yunion.io/x/onecloud/pkg/apis/workflow"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/sqlchemy"
)

// +onecloud:swagger-gen-ignore
type SWorkflowProcessDefineManager struct {
	db.SModelBaseManager
	db.SEnabledResourceBaseManager
}

var WorkflowProcessDefineManager *SWorkflowProcessDefineManager

func init() {
	WorkflowProcessDefineManager = &SWorkflowProcessDefineManager{
		SModelBaseManager: db.NewModelBaseManager(
			SWorkflowProcessDefine{},
			"workflow_process_define",
			"workflow_process_definition",
			"workflow_process_definitions",
		),
	}
	WorkflowProcessDefineManager.SetVirtualObject(WorkflowProcessDefineManager)
}

/*
+------------+--------------+------+-----+---------+-------+
| Field      | Type         | Null | Key | Default | Extra |
+------------+--------------+------+-----+---------+-------+
| id         | varchar(128) | NO   | PRI | NULL    |       |
| name       | varchar(128) | NO   |     | NULL    |       |
| extra_id   | varchar(128) | NO   |     | NULL    |       |
| key        | varchar(256) | NO   |     | NULL    |       |
| type       | varchar(128) | NO   |     | NULL    |       |
| setting    | text         | YES  |     | NULL    |       |
| enabled    | tinyint(1)   | NO   |     | 0       |       |
| created_at | datetime     | NO   |     | NULL    |       |
| updated_at | datetime     | NO   |     | NULL    |       |
+------------+--------------+------+-----+---------+-------+
*/

type SWorkflowProcessDefine struct {
	db.SModelBase
	db.SEnabledResourceBase

	// 资源UUID
	Id      string `width:"128" charset:"ascii" primary:"true" list:"user" json:"id"`
	Name    string `width:"128" nullable:"false" list:"user" json:"name"`
	ExtraId string `width:"128" nullable:"true"`
	Key     string `width:"256" nullable:"false" list:"user" json:"key"`
	Type    string `width:"128" nullable:"false" list:"user" json:"type"`
	Setting string `length:"text" nullable:"true"`
	//Data    string            `length:"text"`
	//Enabled tristate.TriState `nullable:"false" default:"false" list:"user"`
	// 资源创建时间
	CreatedAt time.Time `nullable:"false" created_at:"true" list:"user" json:"created_at"`
	// 资源更新时间
	UpdatedAt time.Time `nullable:"false" updated_at:"true" list:"user" json:"updated_at"`
}

func (p *SWorkflowProcessDefineManager) ListItemFilter(
	ctx context.Context,
	q *sqlchemy.SQuery,
	userCred mcclient.TokenCredential,
	input api.WorkflowProcessDefineListInput,
) (*sqlchemy.SQuery, error) {
	var err error
	q, err = p.SEnabledResourceBaseManager.ListItemFilter(ctx, q, userCred, input.EnabledResourceBaseListInput)
	if err != nil {
		return nil, errors.Wrap(err, "SEnabledResourceBaseManager.ListItemFilter")
	}

	return q, nil
}

func (p *SWorkflowProcessDefineManager) FilterById(q *sqlchemy.SQuery, idStr string) *sqlchemy.SQuery {
	return q.Equals("id", idStr)
}

func (p *SWorkflowProcessDefine) CustomizeCreate(ctx context.Context, userCred mcclient.TokenCredential, ownerId mcclient.IIdentityProvider, query jsonutils.JSONObject, data jsonutils.JSONObject) error {
	var input api.WorkflowProcessDefineCreateInput
	err := data.Unmarshal(&input)
	if err != nil {
		return err
	}
	p.Id = db.DefaultUUIDGenerator()
	p.Name = input.Name
	p.Key = input.Key
	p.Type = input.Type
	return nil
}

func (p *SWorkflowProcessDefine) PerformEnable(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, input apis.PerformEnableInput) (jsonutils.JSONObject, error) {
	err := db.EnabledPerformEnable(p, ctx, userCred, true)
	if err != nil {
		return nil, errors.Wrap(err, "EnabledPerformEnable")
	}
	return nil, nil
}

func (p *SWorkflowProcessDefine) PerformDisable(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, input apis.PerformDisableInput) (jsonutils.JSONObject, error) {
	err := db.EnabledPerformEnable(p, ctx, userCred, false)
	if err != nil {
		return nil, errors.Wrap(err, "EnabledPerformEnable")
	}
	return nil, nil
}

func (p *SWorkflowProcessDefine) PostUpdate(ctx context.Context, userCred mcclient.TokenCredential, query jsonutils.JSONObject, data jsonutils.JSONObject) {
	enabled, _ := data.Bool("enabled")
	err := db.EnabledPerformEnable(p, ctx, userCred, enabled)
	if err != nil {
		//return nil, errors.Wrap(err, "EnabledPerformEnable")
	}
}
