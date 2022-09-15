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
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/sqlchemy"
)

const (
	DAILY_BILL_PULL       = "daily_bill_pull"
	MONTHLY_BILL_GENERATE = "monthly_bill_generate"
)

// +onecloud:swagger-gen-ignore
type SProgressManager struct {
	db.SResourceBaseManager
}

var ProgressManager *SProgressManager

func init() {
	ProgressManager = &SProgressManager{
		SResourceBaseManager: db.NewResourceBaseManager(
			SProgress{},
			"progresses_tbl",
			"progress",
			"progresses",
		),
	}
	ProgressManager.SetVirtualObject(ProgressManager)
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
|type          |varchar(128)|NO  |MUL|NULL   |     |
|finished       |tinyint(1)  |NO  |   |0      |     |
|progress      |int(11)     |YES |   |0      |     |
|account_id    |varchar(128)|YES |MUL|NULL   |     |
|date          |int(11)     |YES |MUL|0      |     |
|mark          |varchar(128)|YES |   |NULL   |     |
|task_key      |varchar(128)|YES |MUL|NULL   |     |
|user_id       |varchar(128)|YES |MUL|NULL   |     |
|options       |text        |YES |   |NULL   |     |
+--------------+------------+----+---+-------+-----+
*/

type SProgress struct {
	db.SResourceBase

	Id        string `width:"128" charset:"ascii" primary:"true" list:"user" json:"id"`
	Type      string `width:"128" charset:"ascii" index:"true" list:"user" create:"domain_optional" update:"admin" json:"type"`
	Finished  bool   `list:"user" create:"domain_optional" update:"admin" json:"finished"`
	Progress  int    `list:"user" update:"user" json:"progress"`
	AccountId string `width:"128" charset:"ascii" index:"true" nullable:"true" list:"user" create:"domain_optional" update:"admin" json:"account_id"`
	Date      int    `list:"user" update:"user" index:"true" json:"date"`
	Mark      string `width:"128" charset:"utf8" list:"user" update:"user" json:"mark"`
	TaskKey   string `width:"128" charset:"ascii" list:"user" create:"domain_optional" update:"admin" json:"task_key"`
	Misc      string `length:"text" nullable:"true"`
}

func (manager *SProgressManager) FilterById(q *sqlchemy.SQuery, idStr string) *sqlchemy.SQuery {
	return q.Equals("id", idStr)
}

func (manager *SProgressManager) QueryDistinctExtraField(q *sqlchemy.SQuery, field string) (*sqlchemy.SQuery, error) {
	var err error

	q, err = manager.SResourceBaseManager.QueryDistinctExtraField(q, field)
	if err == nil {
		return q, nil
	}
	return q, httperrors.ErrNotFound
}

func (manager *SProgressManager) InDailyPullProgress(date int) bool {
	q := manager.Query().Equals("date", date).Equals("type", DAILY_BILL_PULL)
	count, err := q.CountWithError()
	if err != nil {
		log.Errorf("get %s task progress err %s.", DAILY_BILL_PULL, err)
		return true
	}
	if count > 0 {
		log.Debugf("the %s task is in progress.", DAILY_BILL_PULL)
		return true
	}
	return false
}

func (manager *SProgressManager) CreateDailyPullProgress(ctx context.Context, date int) (string, error) {
	p := new(SProgress)

	p.Id = db.DefaultUUIDGenerator()
	p.TaskKey = DAILY_BILL_PULL
	p.Mark = "OneCloud"
	p.Date = date
	p.Type = DAILY_BILL_PULL
	p.Finished = false
	p.Progress = 0

	p.SetModelManager(manager, p)
	err := manager.TableSpec().Insert(ctx, p)
	if err != nil {
		return "", err
	}
	return p.Id, nil
}

func (manager *SProgressManager) FinishDailyPullProgress(date int, misc string, errCount int) error {
	q := manager.Query().Equals("date", date).Equals("type", DAILY_BILL_PULL)
	count, err := q.CountWithError()
	if err != nil {
		log.Errorf("get progress task of %d err %v", date, err)
		return err
	}
	if count == 0 {
		log.Debugf("the %s task empty?", DAILY_BILL_PULL)
		return errors.ErrNotFound
	}

	p := SProgress{}
	//row := q.Row()
	//err := q.Row2Struct(row, &p)
	err = q.First(&p)
	if err != nil {
		log.Errorf("GetProgress fail: %s", err)
		return err
	}
	p.SetModelManager(ProgressManager, &p) // SetModelManager if else is nil pointer
	_, err = db.Update(&p, func() error {
		p.Finished = true
		p.Progress = 100 - errCount
		p.Misc = misc
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "FinishDailyPullProgress:%d err", date)
	}
	return nil
}
