package tasks

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/apis/compute"
	apis "yunion.io/x/onecloud/pkg/apis/workflow"
	"yunion.io/x/onecloud/pkg/cloudcommon/cmdline"
	"yunion.io/x/onecloud/pkg/cloudcommon/db"
	"yunion.io/x/onecloud/pkg/cloudcommon/db/taskman"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
	computemod "yunion.io/x/onecloud/pkg/mcclient/modules/compute"
	"yunion.io/x/onecloud/pkg/mcclient/modules/identity"
	imagemod "yunion.io/x/onecloud/pkg/mcclient/modules/image"
	"yunion.io/x/onecloud/pkg/mcclient/modules/thirdparty"
	"yunion.io/x/onecloud/pkg/util/logclient"
	"yunion.io/x/onecloud/pkg/workflow/models"
	"yunion.io/x/onecloud/pkg/workflow/options"
	"yunion.io/x/pkg/errors"
)

type BpmProcessSubmitTask struct {
	taskman.STask
}

func init() {
	taskman.RegisterTask(BpmProcessSubmitTask{})
}

func (self *BpmProcessSubmitTask) taskFailed(ctx context.Context, workflow *models.SWorkflowProcessInstance, reason string) {
	log.Errorf("fail to send workflow %q", workflow.GetId())
	workflow.SetStatus(self.UserCred, apis.WORKFLOW_SUBMIT_STATUS_FAILED, reason)
	workflow.SetState(models.SUBMIT_FAIL)
	logclient.AddActionLogWithContext(ctx, workflow, logclient.ACT_BPM_SEND_APPLICATION, reason, self.UserCred, false)
	self.SetStageFailed(ctx, jsonutils.NewString(reason))
}

func (self *BpmProcessSubmitTask) OnInit(ctx context.Context, obj db.IStandaloneModel, body jsonutils.JSONObject) {
	workflow := obj.(*models.SWorkflowProcessInstance)
	if workflow.Status == apis.WORKFLOW_INSTANCE_STATUS_OK || workflow.Status == apis.WORKFLOW_INSTANCE_STATUS_SUCCESS {
		self.SetStageComplete(ctx, nil)
		return
	}
	//todo BPM
	r, err := self.submitProcess(ctx, workflow)
	if err != nil {
		//self.taskFailed(ctx, workflow, "fail to submit bpm process")
		self.taskFailed(ctx, workflow, err.Error())
		return
	}
	if r.Code != "000000" {
		self.taskFailed(ctx, workflow, r.Message)
		return
	}

	workflow.SetStatus(self.UserCred, apis.WORKFLOW_SUBMIT_STATUS_OK, "")
	workflow.SetState(models.PENDING)
	workflow.SetBpmExternalId(r.Data.ProcessInstanceId)
	logclient.AddActionLogWithContext(ctx, workflow, logclient.ACT_BPM_SEND_APPLICATION, "", self.UserCred, true)

	//self.SetStage("on_wait_guest_networks_ready", nil)
	//self.ScheduleRun(nil)
	//self.OnWaitGuestNetworksReady(ctx, obj, nil)

	self.SetStageComplete(ctx, nil)
}

func (self *BpmProcessSubmitTask) OnWaitGuestNetworksReady(ctx context.Context, obj db.IStandaloneModel, data jsonutils.JSONObject) {
	log.Infof("Guest %s network not ready!!")
	log.Infof("Guest %s network not ready!!")
	log.Infof("Guest %s network not ready!!")
}

type DataSet struct {
	Values   []*CodeValue
	DataCode string `json:"data_code"`
}

type CodeValue struct {
	Code  string
	Value string
}

type submitProcessResponse struct {
	State     string
	Message   string
	Code      string
	Timestamp string
	Data      DataResponse
}

type DataResponse struct {
	ProcessInstanceId string `json:"process_instance_id"`
	ProcessLocation   string `json:"process_location"`
	ProcessStatus     string `json:"process_status"`
	DataId            string `json:"data_id"`
}

func (self *BpmProcessSubmitTask) submitProcess(ctx context.Context, workflow *models.SWorkflowProcessInstance) (*submitProcessResponse, error) {

	s := auth.GetAdminSession(ctx, options.Options.Region)
	userV3, err := identity.UsersV3.Get(s, workflow.Initiator, nil)
	if err != nil {
		self.taskFailed(ctx, workflow, "fail to get userv3 info")
		return nil, err
	}
	extra, err := userV3.Get(modules.Extra)
	if err != nil {
		self.taskFailed(ctx, workflow, "fail to get userv3 extra")
		return nil, err
	}
	staffId, err := extra.GetString("staff_id")
	if err != nil {
		self.taskFailed(ctx, workflow, "fail to get userv3 staff_id")
		return nil, err
	}
	if staffId == "" {
		self.taskFailed(ctx, workflow, "userv3 staff id empty")
		return nil, err
	}
	coaUser, err := thirdparty.CoaUsers.Get(s, staffId, nil)
	if err != nil {
		self.taskFailed(ctx, workflow, "fail to get coa user info")
		return nil, err
	}
	departmentId, _ := coaUser.GetString("department_id")
	feishuUserId, _ := coaUser.GetString("feishu_user_id")
	//httpclient := httputils.GetDefaultClient()

	body := jsonutils.NewDict()
	body.Set("tenant_id", jsonutils.NewString("li"))
	body.Set("current_user_id", jsonutils.NewString(feishuUserId))
	body.Set("app_id", jsonutils.NewString("app_3qedx9grhl"))

	switch workflow.Key {
	case modules.APPLY_MACHINE:
		body.Set("form_key", jsonutils.NewString("li_form_qs1pnmcz5g"))
		body.Set("process_key", jsonutils.NewString("li_cloud_apply"))
		dataSet, err := self.getApplyDataSet(ctx, workflow, departmentId, feishuUserId)
		if err != nil {
			self.taskFailed(ctx, workflow, "fail to get apply data set")
			return nil, err
		}
		body.Set("data_set", jsonutils.Marshal(dataSet))
	case modules.APPLY_SERVER_CHANGECONFIG:
		body.Set("form_key", jsonutils.NewString("li_form_ne54dpv5fj"))
		body.Set("process_key", jsonutils.NewString("li_cloud_changeconfig"))
		dataSet, err := self.getChangeConfigDataSet(ctx, workflow, departmentId, feishuUserId)
		if err != nil {
			self.taskFailed(ctx, workflow, "fail to get change config data set")
			return nil, err
		}
		body.Set("data_set", jsonutils.Marshal(dataSet))
	case modules.APPLY_BUCKET:
		body.Set("form_key", jsonutils.NewString("li_form_4mvrleec5t"))
		body.Set("process_key", jsonutils.NewString("li_bucket_apply"))
		dataSet, err := self.getBucketApplyDataSet(ctx, workflow, departmentId, feishuUserId)
		if err != nil {
			self.taskFailed(ctx, workflow, "fail to get apply data set")
			return nil, err
		}
		body.Set("data_set", jsonutils.Marshal(dataSet))
	case modules.APPLY_LOADBALANCER:
		body.Set("form_key", jsonutils.NewString("li_form_rkwq8l8xqj"))
		body.Set("process_key", jsonutils.NewString("li_lb_apply"))
		dataSet, err := self.getLoadbalancerApplyDataSet(ctx, workflow, departmentId, feishuUserId)
		if err != nil {
			self.taskFailed(ctx, workflow, "fail to get loadbalancer apply data set")
			return nil, err
		}
		body.Set("data_set", jsonutils.Marshal(dataSet))
	}

	//var appId = "li_test_itxtzh"
	//var secretKey = "8baa8b7b161f1af1"
	//var appId = "li_itxtzh"
	//var secretKey = "d71da5813650108c"
	//bpmUrl := "https://bpm.lixiangoa.com/apps/api/v1/openapi/process"
	//_, resp, err := httputils.JSONRequest(httpclient, ctx, httputils.POST, bpmUrl+"/submitProcess", header, body, true)
	resp, err := thirdparty.BpmProcess.SubmitProcess(s, body)
	if err != nil {
		log.Errorf("BpmSubmitProcess: %v", err)
		return nil, err
	}

	d := new(submitProcessResponse)
	err = resp.Unmarshal(d)
	if err != nil {
		log.Errorf("BpmSubmitResponse Unmarshal: %v", err)
		return nil, errors.Wrap(err, "Unmarshal")
	}
	return d, nil
}

func (self *BpmProcessSubmitTask) getApplyDataSet(ctx context.Context, workflow *models.SWorkflowProcessInstance, departmentId, feishuUserId string) (DataSet, error) {
	params, err := jsonutils.ParseString(workflow.Setting)
	if err != nil {
		log.Errorf("fail to parse workflow setting, %v", err)
		return DataSet{}, err
	}

	var diskSizeDesc, pzgg, os string
	//diskBackendMap := map[string]string{
	//	"local": "本地磁盘",
	//	"rbd":   "Ceph RBD",
	//}
	diskTypeMap := map[string]string{
		"sys": "系统盘：",
		//"data": "数据盘%d：",
		"data": "数据盘：",
	}
	count := 0

	input, err := cmdline.FetchServerCreateInputByJSON(params)
	if err != nil {
		log.Errorf("fail to fetch FetchServerCreateInputByJSON, %v", err)
		return DataSet{}, err
	}
	instanceTypeFamily := strings.Split(input.InstanceType, ".")[1]
	switch instanceTypeFamily {
	case "g1":
		pzgg = fmt.Sprintf("%s(通用型 %d核%dGB)", input.InstanceType, input.VcpuCount, input.VmemSize/1024)
	case "c1":
		pzgg = fmt.Sprintf("%s(计算优化型 %d核%dGB)", input.InstanceType, input.VcpuCount, input.VmemSize/1024)
	case "r1":
		pzgg = fmt.Sprintf("%s(内存优化型 %d核%dGB)", input.InstanceType, input.VcpuCount, input.VmemSize/1024)
	default:
		pzgg = fmt.Sprintf("%s(通用型 %d核%dGB)", input.InstanceType, input.VcpuCount, input.VmemSize/1024)
	}
	s := auth.GetAdminSession(ctx, options.Options.Region)

	for _, disk := range input.Disks {
		//if i == len(input.Disks)-1 {
		//	diskSizeDesc += strconv.Itoa(disk.SizeMb/1024) + " GB"
		//} else {
		//	diskSizeDesc += strconv.Itoa(disk.SizeMb/1024) + ","
		//}
		var prefix string
		switch disk.DiskType {
		case "sys":
			prefix = diskTypeMap[disk.DiskType]
			if disk.ImageId != "" {
				image, _ := imagemod.Images.GetById(s, disk.ImageId, nil)
				os, _ = image.GetString("name")
			} else if input.Cdrom != "" {
				image, _ := imagemod.Images.GetById(s, input.Cdrom, nil)
				os, _ = image.GetString("name")
			} else {
				log.Errorf("No bootable disk information provided for %s", input.GenerateName)
				return DataSet{}, errors.Errorf("No bootable disk information provided for %s", input.GenerateName)
			}
		case "data":
			//prefix = fmt.Sprintf(diskTypeMap[disk.DiskType], disk.Index)
			prefix = diskTypeMap[disk.DiskType]
		}
		//diskSizeDesc += prefix + strconv.Itoa(disk.SizeMb/1024) + " GB(" + diskBackendMap[disk.Backend] + ")\n"
		diskSizeDesc += prefix + strconv.Itoa(disk.SizeMb/1024) + " GB\n"
	}
	count = input.Count

	dataSet := DataSet{
		Values: []*CodeValue{
			&CodeValue{
				Code:  "ywbm",
				Value: workflow.Id,
			},
			&CodeValue{
				Code:  "zjm",
				Value: input.GenerateName,
			},
			&CodeValue{
				Code:  "czxt",
				Value: os,
			},
			&CodeValue{
				Code:  "zylx",
				Value: modules.WorkflowProcessDefinitionsMap[workflow.Type],
			},
			&CodeValue{
				Code:  "pzgg",
				Value: pzgg,
			},
			&CodeValue{
				Code:  "cpdx",
				Value: diskSizeDesc,
			},
			&CodeValue{
				Code:  "sl",
				Value: strconv.Itoa(count),
			},
			&CodeValue{
				Code:  "sqr",
				Value: feishuUserId,
			},
			&CodeValue{
				Code:  "sqbm",
				Value: departmentId,
			},
			&CodeValue{
				Code:  "sqsj",
				Value: workflow.CreatedAt.Local().Format("2006-01-02 15:04:05"),
			},
			&CodeValue{
				Code:  "sqyy",
				Value: workflow.Description,
			},
		},
		DataCode: "li_app_3qedx9grhl_formtable_main_636",
	}

	return dataSet, nil
}

func (self *BpmProcessSubmitTask) getChangeConfigDataSet(ctx context.Context, workflow *models.SWorkflowProcessInstance, departmentId, feishuUserId string) (DataSet, error) {
	params, err := jsonutils.ParseString(workflow.Setting)
	if err != nil {
		log.Errorf("fail to parse workflow setting, %v", err)
		return DataSet{}, err
	}

	var names, bgqpz, bghpz, os string
	//diskTypeMap := map[string]string{
	//	"sys":  "系统盘：",
	//	"data": "数据盘：",
	//}
	count := 0

	input := new(compute.ServerChangeConfigInput)
	err = params.Unmarshal(input)
	if err != nil {
		log.Errorf("fail to unmarshal ServerChangeConfigInput %v", err)
		return DataSet{}, err
	}
	s := auth.GetAdminSession(ctx, options.Options.Region)
	skuJson, err := computemod.ServerSkus.GetByName(s, input.InstanceType, nil)
	if err != nil {
		log.Errorf("fail to get server sku %v", err)
		return DataSet{}, err
	}
	if skuJson == nil || skuJson == jsonutils.JSONNull {
		log.Errorln("get empty server sku")
		return DataSet{}, fmt.Errorf("get empty server sku")
	}

	sku := new(compute.ServerSkuDetails)
	err = skuJson.Unmarshal(sku)
	if err != nil {
		log.Errorf("fail to unmarshal ServerSkuDetails %v", err)
		return DataSet{}, err
	}
	bghpz = fmt.Sprintf("%s(通用型 %d核%dGB)", input.InstanceType, sku.CpuCoreCount, sku.MemorySizeMB/1024)

	ids := strings.Split(workflow.Ids, ",")
	count = len(ids)
	for _, id := range ids {
		serverJson, err := computemod.Servers.GetById(s, id, nil)
		if err != nil {
			log.Errorf("unable to fetch server %v", err)
			return DataSet{}, err
		}
		serverDetail := new(compute.ServerDetails)
		err = serverJson.Unmarshal(serverDetail)
		if err != nil {
			log.Errorf("fail to unmarshal ServerDetails %v", err)
			return DataSet{}, err
		}
		names += serverDetail.Name + "\n"
		if os == "" {
			for _, info := range serverDetail.DisksInfo {
				if info.DiskType == "sys" {
					os = info.Image
					break
				}
			}
		}
		serverInfo := fmt.Sprintf("%s(通用型 %d核%dGB)", serverDetail.InstanceType, serverDetail.VcpuCount, serverDetail.VmemSize/1024)
		if count == 1 && len(input.Disks) > 0 {
			beforeDiskDesc := fmt.Sprintf("，磁盘总量：%dGB\n", serverDetail.DiskSizeMb/1024)
			serverInfo = serverInfo + beforeDiskDesc
			//diskSizeDesc = fmt.Sprintf("增加%d块数据盘：\n", len(input.Disks))
			diskSum := 0
			for _, disk := range input.Disks {
				//diskSizeDesc += strconv.Itoa(disk.SizeMb/1024) + " GB(" + diskBackendMap[disk.Backend] + ")\n"
				diskSum += disk.SizeMb
			}
			afterDiskDesc := fmt.Sprintf("，磁盘总量：%dGB", (int64(diskSum)+serverDetail.DiskSizeMb)/1024)
			bghpz = bghpz + afterDiskDesc
		} else {
			serverInfo = serverInfo + "\n"
		}
		bgqpz = bgqpz + serverInfo
	}

	dataSet := DataSet{
		Values: []*CodeValue{
			&CodeValue{
				Code:  "ywbm",
				Value: workflow.Id,
			},
			&CodeValue{
				Code:  "zjm",
				Value: names,
			},
			&CodeValue{
				Code:  "czxt",
				Value: os,
			},
			&CodeValue{
				Code:  "zylx",
				Value: modules.WorkflowProcessDefinitionsMap[workflow.Type],
			},
			&CodeValue{
				Code:  "bgqpz",
				Value: bgqpz,
			},
			&CodeValue{
				Code:  "bghpz",
				Value: bghpz,
			},
			&CodeValue{
				Code:  "sl",
				Value: strconv.Itoa(count),
			},
			&CodeValue{
				Code:  "sqr",
				Value: feishuUserId,
			},
			&CodeValue{
				Code:  "sqbm",
				Value: departmentId,
			},
			&CodeValue{
				Code:  "sqsj",
				Value: workflow.CreatedAt.Local().Format("2006-01-02 15:04:05"),
			},
			&CodeValue{
				Code:  "bgyy",
				Value: workflow.Description,
			},
		},
		DataCode: "li_app_3qedx9grhl_formtable_main_637",
	}

	return dataSet, nil
}

func (self *BpmProcessSubmitTask) getBucketApplyDataSet(ctx context.Context, workflow *models.SWorkflowProcessInstance, departmentId, feishuUserId string) (DataSet, error) {
	params, err := jsonutils.ParseString(workflow.Setting)
	if err != nil {
		log.Errorf("fail to parse workflow setting, %v", err)
		return DataSet{}, err
	}

	//var diskSizeDesc, pzgg string
	enabledMap := map[bool]string{
		true:  "开启",
		false: "未开启",
	}
	count := 1

	input := new(compute.BucketCreateInput)
	err = params.Unmarshal(input)
	if err != nil {
		log.Errorf("fail to unmarshal BucketCreateInput %v", err)
		return DataSet{}, err
	}

	var userName string
	if input.CloudproviderType == compute.CLOUD_PROVIDER_TYPE_SPECIFY {
		s := auth.GetAdminSession(ctx, options.Options.Region)
		providerJson, err := computemod.Cloudproviders.GetById(s, input.CloudproviderId, nil)
		if err != nil {
			log.Errorf("fail to get cloudprovider %v", err)
			return DataSet{}, err
		}
		providerDetail := new(compute.CloudproviderDetails)
		err = providerJson.Unmarshal(providerDetail)
		if err != nil {
			log.Errorf("fail to unmarshal providerDetail %v", err)
			return DataSet{}, err
		}
		userName = providerDetail.Name
	} else {
		userName = input.CloudproviderName
	}

	dataSet := DataSet{
		Values: []*CodeValue{
			&CodeValue{
				Code:  "ywbm",
				Value: workflow.Id,
			},
			&CodeValue{
				Code:  "zylx",
				Value: modules.WorkflowProcessDefinitionsMap[workflow.Type],
			},
			&CodeValue{
				Code:  "xbh",
				Value: enabledMap[input.Worm],
			},
			&CodeValue{
				Code:  "dbb",
				Value: enabledMap[input.Versioned],
			},
			&CodeValue{
				Code:  "rlsx",
				Value: getSizeStr(input.SizeBytesLimit),
			},
			&CodeValue{
				Code:  "dxsx",
				Value: getObjectCntStr(input.ObjectCntLimit),
			},
			&CodeValue{
				Code:  "sl",
				Value: strconv.Itoa(count),
			},
			&CodeValue{
				Code:  "sqr",
				Value: feishuUserId,
			},
			&CodeValue{
				Code:  "sqbm",
				Value: departmentId,
			},
			&CodeValue{
				Code:  "sqsj",
				Value: workflow.CreatedAt.Local().Format("2006-01-02 15:04:05"),
			},
			&CodeValue{
				Code:  "sqyy",
				Value: workflow.Description,
			},
			&CodeValue{
				Code:  "tmc",
				Value: input.Name,
			},
			&CodeValue{
				Code:  "dxyhmc",
				Value: userName,
			},
		},
		DataCode: "li_app_3qedx9grhl_formtable_main_638",
	}

	return dataSet, nil
}

func getSizeStr(size int64) string {
	if size == 0 {
		return "无限制"
	}

	var unit string
	switch {
	case size/(1024*1024*1024*1024) > 0:
		unit = " TB"
		return strconv.Itoa(int(size/(1024*1024*1024*1024))) + unit
	case size/(1024*1024*1024) > 0:
		unit = " GB"
		return strconv.Itoa(int(size/(1024*1024*1024))) + unit
	case size/(1024*1024) > 0:
		unit = " MB"
		return strconv.Itoa(int(size/(1024*1024))) + unit
	case size/1024 > 0:
		unit = " KB"
		return strconv.Itoa(int(size/1024)) + unit
	default:
		unit = " B"
		return strconv.Itoa(int(size)) + unit
	}
}

func getObjectCntStr(oc int) string {
	if oc == 0 {
		return "无限制"
	}

	var unit string
	switch {
	case oc/(10000*10000) > 0:
		unit = " 亿"
		return strconv.Itoa(oc/(10000*10000)) + unit
	case oc/100000 > 0:
		unit = " 万"
		return strconv.Itoa(oc/10000) + unit
	default:
		unit = " 个"
		return strconv.Itoa(oc) + unit
	}
}

func (self *BpmProcessSubmitTask) getLoadbalancerApplyDataSet(ctx context.Context, workflow *models.SWorkflowProcessInstance, departmentId, feishuUserId string) (DataSet, error) {
	params, err := jsonutils.ParseString(workflow.Setting)
	if err != nil {
		log.Errorf("fail to parse workflow setting, %v", err)
		return DataSet{}, err
	}

	count := 1

	input := new(compute.LoadbalancerCreateInput)
	err = params.Unmarshal(input)
	if err != nil {
		log.Errorf("fail to unmarshal LoadbalancerCreateInput %v", err)
		return DataSet{}, err
	}

	s := auth.GetAdminSession(ctx, options.Options.Region)

	projectJson, _ := identity.Projects.GetById(s, input.ProjectId, nil)
	projectName, _ := projectJson.GetString("name")

	networkJson, _ := computemod.Networks.GetById(s, input.NetworkId, nil)
	networkName, _ := networkJson.GetString("name")

	clusterJson, _ := computemod.LoadbalancerClusters.GetById(s, input.ClusterId, nil)
	clusterName, _ := clusterJson.GetString("name")

	dataSet := DataSet{
		Values: []*CodeValue{
			&CodeValue{
				Code:  "ywbm",
				Value: workflow.Id,
			},
			&CodeValue{
				Code:  "zylx",
				Value: modules.WorkflowProcessDefinitionsMap[workflow.Type],
			},
			&CodeValue{
				Code:  "slmc",
				Value: input.Name,
			},
			&CodeValue{
				Code:  "xm",
				Value: projectName,
			},
			&CodeValue{
				Code:  "jq",
				Value: clusterName,
			},
			&CodeValue{
				Code:  "wl",
				Value: networkName,
			},
			&CodeValue{
				Code:  "sl",
				Value: strconv.Itoa(count),
			},
			&CodeValue{
				Code:  "sqr",
				Value: feishuUserId,
			},
			&CodeValue{
				Code:  "sqbm",
				Value: departmentId,
			},
			&CodeValue{
				Code:  "sqsj",
				Value: workflow.CreatedAt.Local().Format("2006-01-02 15:04:05"),
			},
			&CodeValue{
				Code:  "sqyy",
				Value: workflow.Description,
			},
		},
		DataCode: "li_app_3qedx9grhl_formtable_main_639",
	}

	return dataSet, nil
}
