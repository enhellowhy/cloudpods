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
	"context"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"strconv"
	"strings"
	"yunion.io/x/onecloud/pkg/util/fileutils2"
	"yunion.io/x/onecloud/pkg/util/procutils"

	"github.com/gorilla/mux"

	"yunion.io/x/log"
	"yunion.io/x/pkg/util/signalutils"
	_ "yunion.io/x/sqlchemy/backends"

	api "yunion.io/x/onecloud/pkg/apis/webconsole"
	"yunion.io/x/onecloud/pkg/appsrv"
	"yunion.io/x/onecloud/pkg/cloudcommon"
	app_common "yunion.io/x/onecloud/pkg/cloudcommon/app"
	"yunion.io/x/onecloud/pkg/cloudcommon/cronman"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	common_options "yunion.io/x/onecloud/pkg/cloudcommon/options"
	"yunion.io/x/onecloud/pkg/webconsole"
	"yunion.io/x/onecloud/pkg/webconsole/models"
	o "yunion.io/x/onecloud/pkg/webconsole/options"
	"yunion.io/x/onecloud/pkg/webconsole/server"
)

func ensureBinExists(binPath string) {
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		log.Fatalf("Binary %s not exists", binPath)
	}
}

func StartService() {

	opts := &o.Options
	commonOpts := &o.Options.CommonOptions
	common_options.ParseOptions(opts, os.Args, "webconsole.conf", api.SERVICE_TYPE)

	if opts.ApiServer == "" {
		log.Fatalf("--api-server must specified")
	}
	_, err := url.Parse(opts.ApiServer)
	if err != nil {
		log.Fatalf("invalid --api-server %s", opts.ApiServer)
	}

	for _, binPath := range []string{opts.IpmitoolPath, opts.SshToolPath, opts.SshpassToolPath} {
		//for _, binPath := range []string{opts.IpmitoolPath, opts.SshToolPath} {
		ensureBinExists(binPath)
	}

	app_common.InitAuth(commonOpts, func() {
		log.Infof("Auth complete")
	})

	common_options.StartOptionManager(opts, opts.ConfigSyncPeriodSeconds, api.SERVICE_TYPE, api.SERVICE_VERSION, o.OnOptionsChange)

	//s3fs is daemon, otherwise it will be blocked here.
	initS3()

	registerSigTraps()
	start()
}

func initS3() {
	url := o.Options.S3Endpoint
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		prefix := "http://"
		if o.Options.S3UseSSL {
			prefix = "https://"
		}
		url = prefix + url
	}
	//err := s3.Init(
	//	url,
	//	o.Options.S3AccessKey,
	//	o.Options.S3SecretKey,
	//	o.Options.S3BucketName,
	//	o.Options.S3UseSSL,
	//)
	//if err != nil {
	//	log.Fatalf("failed init s3 client %s", err)
	//}
	func() {
		fd, err := os.OpenFile("/tmp/s3-pass", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			log.Fatalf("failed open s3 pass file %s", err)
		}
		defer fd.Close()
		_, err = fd.WriteString(fmt.Sprintf("%s:%s", o.Options.S3AccessKey, o.Options.S3SecretKey))
		if err != nil {
			log.Fatalf("failed write s3 pass file")
		}
	}()
	if !fileutils2.Exists(o.Options.S3MountPoint) {
		err := os.MkdirAll(o.Options.S3MountPoint, 0755)
		if err != nil {
			log.Fatalf("fail to create %s: %s", o.Options.S3MountPoint, err)
		}
	}

	out, err := procutils.NewCommand("s3fs",
		//o.Options.S3BucketName, o.Options.S3MountPoint, "-f", //this is frontend
		o.Options.S3BucketName, o.Options.S3MountPoint,
		"-o", fmt.Sprintf("passwd_file=/tmp/s3-pass,use_path_request_style,url=%s", url)).Output()
	if err != nil {
		log.Fatalf("failed mount s3fs %s %s", err, out)
	}
}

func registerSigTraps() {
	signalutils.SetDumpStackSignal()
	signalutils.StartTrap()
}

func start() {
	baseOpts := &o.Options.BaseOptions

	// commonOpts := &o.Options.CommonOptions
	app := app_common.InitApp(baseOpts, true)
	dbOpts := &o.Options.DBOptions

	cloudcommon.InitDB(dbOpts)

	webconsole.InitHandlers(app)

	db.EnsureAppSyncDB(app, dbOpts, models.InitDB)

	root := mux.NewRouter()
	root.UseEncodedPath()

	// api handler
	root.PathPrefix(webconsole.ApiPathPrefix).Handler(app)

	srv := server.NewConnectionServer()
	// websocket command text console handler
	root.Handle(webconsole.ConnectPathPrefix, srv)

	// websockify graphic console handler
	root.Handle(webconsole.WebsockifyPathPrefix, srv)

	// websocketproxy handler
	root.Handle(webconsole.WebsocketProxyPathPrefix, srv)

	// misc handler
	addMiscHandlers(root)

	cron := cronman.InitCronJobManager(true, o.Options.CronJobWorkerCount)

	cron.AddJobEveryFewHour("AutoPurgeSplitable", 4, 30, 0, db.AutoPurgeSplitable, false)

	cron.Start()
	defer cron.Stop()

	addr := net.JoinHostPort(o.Options.Address, strconv.Itoa(o.Options.Port))
	log.Infof("Start listen on %s", addr)
	if o.Options.EnableSsl {
		err := http.ListenAndServeTLS(addr,
			o.Options.SslCertfile,
			o.Options.SslKeyfile,
			root)
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("%v", err)
		}
	} else {
		err := http.ListenAndServe(addr, root)
		if err != nil {
			log.Fatalf("%v", err)
		}
	}
}

func addMiscHandlers(root *mux.Router) {
	adapterF := func(appHandleFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			appHandleFunc(context.TODO(), w, r)
		}
	}

	// ref: pkg/appsrv/appsrv:addDefaultHandlers
	root.HandleFunc("/version", adapterF(appsrv.VersionHandler))
	root.HandleFunc("/stats", adapterF(appsrv.StatisticHandler))
	root.HandleFunc("/ping", adapterF(appsrv.PingHandler))
	root.HandleFunc("/worker_stats", adapterF(appsrv.WorkerStatsHandler))

	// pprof handler
	root.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)
}
