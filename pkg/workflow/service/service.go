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

package service

import (
	"fmt"
	"github.com/getsentry/sentry-go"
	"os"

	"yunion.io/x/log"
	_ "yunion.io/x/sqlchemy/backends"

	"yunion.io/x/onecloud/pkg/apis"
	"yunion.io/x/onecloud/pkg/cloudcommon"
	app_common "yunion.io/x/onecloud/pkg/cloudcommon/app"
	"yunion.io/x/onecloud/pkg/cloudcommon/consts"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	common_options "yunion.io/x/onecloud/pkg/cloudcommon/options"
	"yunion.io/x/onecloud/pkg/workflow/models"
	"yunion.io/x/onecloud/pkg/workflow/options"
	_ "yunion.io/x/onecloud/pkg/workflow/policy"
	_ "yunion.io/x/onecloud/pkg/workflow/tasks"
)

func StartService() {

	consts.DisableOpsLog()

	opts := &options.Options
	baseOpts := &opts.BaseOptions
	commonOpts := &opts.CommonOptions
	dbOpts := &opts.DBOptions
	common_options.ParseOptions(opts, os.Args, "workflow.conf", apis.SERVICE_TYPE_WORKFLOW)

	app_common.InitAuth(commonOpts, func() {
		log.Infof("Auth complete!!")
	})

	app := app_common.InitApp(baseOpts, true)

	cloudcommon.InitDB(dbOpts)
	initHandlers(app)

	db.EnsureAppSyncDB(app, dbOpts, models.InitDB)
	defer cloudcommon.CloseDB()

	//models.StartNotifyToWebsocketWorker()
	sentry.CaptureMessage(fmt.Sprintf("%s sentry works!", app.GetName()))

	app_common.ServeForever(app, baseOpts)
}
