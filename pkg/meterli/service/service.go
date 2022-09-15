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
	"time"
	"yunion.io/x/onecloud/pkg/cloudcommon/cronman"

	_ "github.com/go-sql-driver/mysql"

	"yunion.io/x/log"

	"yunion.io/x/onecloud/pkg/apis"
	"yunion.io/x/onecloud/pkg/cloudcommon"
	app_common "yunion.io/x/onecloud/pkg/cloudcommon/app"
	"yunion.io/x/onecloud/pkg/cloudcommon/consts"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	common_options "yunion.io/x/onecloud/pkg/cloudcommon/options"
	"yunion.io/x/onecloud/pkg/meterli/models"
	"yunion.io/x/onecloud/pkg/meterli/options"
	_ "yunion.io/x/onecloud/pkg/meterli/policy"
)

func StartService() {

	consts.DisableOpsLog()

	opts := &options.Options
	baseOpts := &opts.BaseOptions
	commonOpts := &opts.CommonOptions
	dbOpts := &opts.DBOptions
	common_options.ParseOptions(opts, os.Args, "meter-li.conf", apis.SERVICE_TYPE_METER_LI)

	app_common.InitAuth(commonOpts, func() {
		log.Infof("Auth complete!!")
	})

	app := app_common.InitApp(baseOpts, true)
	initHandlers(app)

	db.EnsureAppInitSyncDB(app, dbOpts, models.InitDB)
	defer cloudcommon.CloseDB()

	//models.StartNotifyToWebsocketWorker()
	sentry.CaptureMessage(fmt.Sprintf("%s sentry works!", app.GetName()))

	cron := cronman.InitCronJobManager(true, options.Options.CronJobWorkerCount)
	//cron.AddJobAtIntervals("CleanPendingDeleteServers", time.Duration(opts.PendingDeleteCheckSeconds)*time.Second, models.GuestManager.CleanPendingDeleteServers)
	//cron.AddJobAtIntervals("CleanPendingDeleteDisks", time.Duration(opts.PendingDeleteCheckSeconds)*time.Second, models.DiskManager.CleanPendingDeleteDisks)
	//cron.AddJobEveryFewDays("Test", 1, 0, 0, models.BillManager.CheckBill, true)

	cron.AddJobEveryFewHour("ComputeBill", 1, 0, 0, models.BillManager.CheckBill, false)
	//cron.AddJobAtIntervalsWithStartRun("ComputeBill", time.Duration(10)*time.Second, models.BillManager.CheckBill, false)
	cron.AddJobAtIntervalsWithStartRun("SyncResources", time.Duration(30)*time.Minute, models.BillResourceManager.SyncResources, false)
	go cron.Start()

	app_common.ServeForever(app, baseOpts)
}
