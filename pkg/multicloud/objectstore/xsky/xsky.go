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

package xsky

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"yunion.io/x/jsonutils"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/s3cli"

	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/multicloud/objectstore"
	"yunion.io/x/onecloud/pkg/util/httputils"
)

type SXskyClient struct {
	*objectstore.SObjectStoreClient

	adminApi *SXskyAdminApi

	adminUser   *sUser
	initAccount string
}

func parseAccount(account string) (user string, accessKey string) {
	accountInfo := strings.Split(account, "/")
	user = accountInfo[0]
	if len(accountInfo) > 1 {
		accessKey = strings.Join(accountInfo[1:], "/")
	}
	return
}

func NewXskyClient(cfg *objectstore.ObjectStoreClientConfig) (*SXskyClient, error) {
	usrname, accessKey := parseAccount(cfg.GetAccessKey())
	adminApi := newXskyAdminApi(
		usrname,
		cfg.GetAccessSecret(),
		cfg.GetEndpoint(),
		cfg.GetDebug(),
	)
	httputils.SetClientProxyFunc(adminApi.httpClient(), cfg.GetCloudproviderConfig().ProxyFunc)

	gwEp, err := adminApi.getS3GatewayEndpoint(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "adminApi.getS3GatewayIP")
	}

	var usr *sUser
	var key *sKey
	if len(accessKey) > 0 {
		//usr, key, err = adminApi.findUserByAccessKey(context.Background(), accessKey)
		usr, key, err = adminApi.getUserByAccessKey(context.Background(), accessKey)
		if err != nil {
			return nil, errors.Wrap(err, "adminApi.findUserByAccessKey")
		}
	} else {
		usr, key, err = adminApi.findFirstUserWithAccessKey(context.Background())
		if err != nil {
			return nil, errors.Wrap(err, "adminApi.findFirstUserWithAccessKey")
		}
	}

	s3store, err := objectstore.NewObjectStoreClientAndFetch(
		objectstore.NewObjectStoreClientConfig(
			gwEp, key.AccessKey, key.SecretKey,
		).Debug(cfg.GetDebug()).CloudproviderConfig(cfg.GetCloudproviderConfig()),
		false,
	)
	if err != nil {
		return nil, errors.Wrap(err, "NewObjectStoreClient")
	}

	client := SXskyClient{
		SObjectStoreClient: s3store,
		adminApi:           adminApi,
		adminUser:          usr,
	}

	if len(accessKey) > 0 {
		client.initAccount = cfg.GetAccessKey()
	}

	client.SetVirtualObject(&client)

	err = client.FetchBuckets()
	if err != nil {
		return nil, errors.Wrap(err, "fetchBuckets")
	}

	return &client, nil
}

func (cli *SXskyClient) GetSubAccounts() ([]cloudprovider.SSubAccount, error) {
	if len(cli.initAccount) > 0 {
		return []cloudprovider.SSubAccount{
			{
				Account:      cli.initAccount,
				Name:         cli.adminUser.Name,
				HealthStatus: api.CLOUD_PROVIDER_HEALTH_NORMAL,
			},
		}, nil
	}
	ctx := context.Background()
	usrs, err := cli.adminApi.getUsers(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "api.getUsers")
	}
	subAccounts := make([]cloudprovider.SSubAccount, 0)
	for i := range usrs {
		keys, err := cli.adminApi.getUserKeys(ctx, usrs[i].Id)
		if err != nil {
			return nil, errors.Wrap(err, "api.getUserKeys")
		}
		ak := usrs[i].getMinKey(keys)
		if len(ak) > 0 {
			subAccount := cloudprovider.SSubAccount{
				Account:      fmt.Sprintf("%s/%s", cli.adminApi.username, ak),
				Name:         usrs[i].Name,
				HealthStatus: api.CLOUD_PROVIDER_HEALTH_NORMAL,
			}
			subAccounts = append(subAccounts, subAccount)
		}
	}
	return subAccounts, nil
}

func (cli *SXskyClient) GetSubAccountById(id int) (cloudprovider.SSubAccount, error) {
	if len(cli.initAccount) > 0 {
		return cloudprovider.SSubAccount{
			Account:      cli.initAccount,
			Name:         cli.adminUser.Name,
			HealthStatus: api.CLOUD_PROVIDER_HEALTH_NORMAL,
		}, nil
	}
	ctx := context.Background()
	usr, err := cli.adminApi.getUserById(ctx, id)
	if err != nil {
		return cloudprovider.SSubAccount{}, errors.Wrap(err, "api.getUserById")
	}
	if usr.Status != api.USER_STATUS_ACTIVE {
		return cloudprovider.SSubAccount{}, fmt.Errorf("user %s status is not active", usr.Name)
	}
	subAccount := cloudprovider.SSubAccount{}
	if usr != nil {
		keys, err := cli.adminApi.getUserKeys(ctx, usr.Id)
		if err != nil {
			return cloudprovider.SSubAccount{}, errors.Wrap(err, "api.getUserKeys.GetSubAccountById")
		}
		ak := usr.getMinKey(keys)
		if len(ak) > 0 {
			subAccount = cloudprovider.SSubAccount{
				Account:      fmt.Sprintf("%s/%s", cli.adminApi.username, ak),
				Name:         usr.Name,
				HealthStatus: api.CLOUD_PROVIDER_HEALTH_NORMAL,
			}
		}
	}
	return subAccount, nil
}

func (cli *SXskyClient) GetAccountId() string {
	return cli.adminApi.username
}

func (cli *SXskyClient) GetVersion() string {
	return ""
}

func (cli *SXskyClient) About() jsonutils.JSONObject {
	about := jsonutils.NewDict()
	if cli.adminUser != nil {
		about.Add(jsonutils.Marshal(cli.adminUser), "admin_user")
	}
	return about
}

func (cli *SXskyClient) GetProvider() string {
	return api.CLOUD_PROVIDER_XSKY
}

func (cli *SXskyClient) NewBucket(bucket s3cli.BucketInfo) cloudprovider.ICloudBucket {
	if cli.SObjectStoreClient == nil {
		return nil
	}
	generalBucket := cli.SObjectStoreClient.NewBucket(bucket)
	return &SXskyBucket{
		SBucket: generalBucket.(*objectstore.SBucket),
		client:  cli,
	}
}

//type sUserSortBucket struct {
//	BucketNum   int
//	DisplayName string
//}
//
//type sUserSortObjects struct {
//	AllocatedObjects int
//	DisplayName      string
//}
//
//type sUserSortSize struct {
//	AllocatedSize int64
//	DisplayName   string
//}

type sUserSort struct {
	Value int64
	Name  string
}

type sBucketSort struct {
	Value int64
	Name  string
}

func (cli *SXskyClient) GetObjectStoreStats() (map[string]interface{}, error) {
	ctx := context.Background()
	oss, err := cli.adminApi.getObjectStorage(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "api.getObjectStorage GetObjectStoreStats")
	}
	usages := make(map[string]interface{})
	usages["xsky.eos.name"] = oss.Name
	usages["xsky.eos.id"] = oss.Id
	usages["xsky.eos.dns"] = oss.DnsNames
	usages["xsky.eos.action.status"] = oss.ActionStatus
	usages["xsky.eos.status"] = oss.Status
	usages["xsky.eos.indexpool.name"] = oss.IndexPool.Name
	usages["xsky.eos.createtime"] = oss.Create
	usages["xsky.eos.updatetime"] = oss.Update
	if len(oss.Samples) > 0 {
		usages["xsky.eos.allocated.objects"] = oss.Samples[0].AllocatedObjects
		usages["xsky.eos.allocated.size"] = oss.Samples[0].AllocatedSize
		usages["xsky.eos.local.allocated.objects"] = oss.Samples[0].LocalAllocatedObjects
		usages["xsky.eos.local.allocated.size"] = oss.Samples[0].LocalAllocatedSize
		usages["xsky.eos.external.allocated.objects"] = oss.Samples[0].ExternalAllocatedObjects
		usages["xsky.eos.external.allocated.size"] = oss.Samples[0].ExternalAllocatedSize
	}
	users, err := cli.adminApi.getUsers(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "api.getUsers GetObjectStoreStats")
	}
	usbs := make([]sUserSort, 0)
	sort.Sort(UserSortBucketSet(users))
	for _, user := range users {
		var us sUserSort
		us.Value = int64(user.BucketNum)
		us.Name = user.DisplayName
		usbs = append(usbs, us)
	}

	usos := make([]sUserSort, 0)
	sort.Sort(UserSortObjectsSet(users))
	for _, user := range users {
		var us sUserSort
		us.Value = user.Samples[0].AllocatedObjects
		us.Name = user.DisplayName
		usos = append(usos, us)
	}

	usss := make([]sUserSort, 0)
	sort.Sort(UserSortSizeSet(users))
	for _, user := range users {
		var us sUserSort
		us.Value = user.Samples[0].AllocatedSize
		us.Name = user.DisplayName
		usss = append(usss, us)
	}
	usages["xsky.eos.user.allocated.size"] = usss
	usages["xsky.eos.user.allocated.objects"] = usos
	usages["xsky.eos.user.allocated.bucket"] = usbs
	usages["xsky.eos.users"] = len(users)

	buckets, err := cli.adminApi.getBuckets(ctx, false)
	if err != nil {
		return nil, errors.Wrap(err, "api.getBuckets GetObjectStoreStats")
	}
	bsos := make([]sBucketSort, 0)
	sort.Sort(BucketSortObjectsSet(buckets))
	for _, bucket := range buckets {
		var bs sBucketSort
		bs.Value = bucket.Samples[0].AllocatedObjects
		bs.Name = bucket.Name
		bsos = append(bsos, bs)
	}

	bsss := make([]sBucketSort, 0)
	sort.Sort(BucketSortSizeSet(buckets))
	for _, bucket := range buckets {
		var bs sBucketSort
		bs.Value = bucket.Samples[0].AllocatedSize
		bs.Name = bucket.Name
		bsss = append(bsss, bs)
	}

	usages["xsky.eos.bucket.allocated.size"] = bsss
	usages["xsky.eos.bucket.allocated.objects"] = bsos
	usages["xsky.eos.buckets"] = len(buckets)
	return usages, nil
}

func (cli *SXskyClient) GetObjectStoreUserStats() (map[string]interface{}, error) {
	usages := make(map[string]interface{})
	usages["xsky.eos.os_users.bucket.num"] = cli.adminUser.BucketNum
	usages["xsky.eos.os_users.bucket.max"] = cli.adminUser.MaxBuckets
	usages["xsky.eos.os_users.user.quota.max.objects"] = cli.adminUser.UserQuotaMaxObjects
	usages["xsky.eos.os_users.user.quota.max.size"] = cli.adminUser.UserQuotaMaxSize
	usages["xsky.eos.os_users.bucket.quota.max.objects"] = cli.adminUser.BucketQuotaMaxObjects
	usages["xsky.eos.os_users.bucket.quota.max.size"] = cli.adminUser.BucketQuotaMaxSize
	if len(cli.adminUser.Samples) > 0 {
		sample := cli.adminUser.Samples[0]
		usages["xsky.eos.os_users.allocated.objects"] = sample.AllocatedObjects
		usages["xsky.eos.os_users.allocated.size"] = sample.AllocatedSize
		usages["xsky.eos.os_users.local.allocated.objects"] = sample.LocalAllocatedObjects
		usages["xsky.eos.os_users.local.allocated.size"] = sample.LocalAllocatedSize
		usages["xsky.eos.os_users.external.allocated.objects"] = sample.ExternalAllocatedObjects
		usages["xsky.eos.os_users.external.allocated.size"] = sample.ExternalAllocatedSize
	}

	bucketList := make([]SBucketAllStats, 0)
	buckets, _ := cli.GetIBuckets()
	for _, bucket := range buckets {
		stats := bucket.(*SXskyBucket).GetAllStats()
		bucketList = append(bucketList, stats)
	}
	usages["xsky.eos.os_users.buckets"] = bucketList

	//usages["xsky.eos.buckets"] = len(buckets)
	return usages, nil
}

//LocalAllocatedObjects    int64
//ExternalAllocatedSize    int64
//LocalAllocatedSize       int64
//ExternalAllocatedObjects int64
//AllocatedObjects         int64
//AllocatedSize            int64
//Create                   time.Time
//DelOpsPm                 int
//RxBandwidthKbyte         int
//RxOpsPm                  int
//TxBandwidthKbyte         int
//TxOpsPm                  int
//TotalDelOps              int
//TotalDelSuccessOps       int
//TotalRxBytes             int64
//TotalRxOps               int
//TotalRxSuccessOps        int
//TotalTxBytes             int64
//TotalTxOps               int
//TotalTxSuccessKbyte      int

type Series struct {
	Columns []string
	Name    string
	Points  [][]int64
}

func GetSeries(name string) *Series {
	var series Series
	series.Name = name
	series.Columns = make([]string, 0)
	series.Columns = append(series.Columns, name)
	series.Columns = append(series.Columns, "time")
	series.Points = make([][]int64, 0)
	return &series
}

func (s *Series) shuffle(timePoint int64, value ...int64) {
	point := make([]int64, 0)
	point = append(point, value...)
	point = append(point, timePoint)
	s.Points = append(s.Points, point)
}

func (cli *SXskyClient) GetObjectStoreUserSamples(from, interval string) (map[string]interface{}, error) {
	ctx := context.Background()
	samples, err := cli.adminApi.getUsersSamples(ctx, from, interval, cli.adminUser.Id)
	if err != nil {
		return nil, errors.Wrap(err, "api.getObjectStorage GetObjectStoreStats")
	}
	allocatedSize := GetSeries("总容量")
	allocatedObjects := GetSeries("对象数")
	rxBandwidthByte := GetSeries("上传")
	txBandwidthByte := GetSeries("下载")
	rxOpsPm := GetSeries("上传请求")
	txOpsPm := GetSeries("下载请求")
	delOpsPm := GetSeries("删除请求")
	for i := range samples {
		allocatedSize.shuffle(samples[i].Create.Unix()*1000, samples[i].AllocatedSize)
		allocatedObjects.shuffle(samples[i].Create.Unix()*1000, samples[i].AllocatedObjects)
		rxBandwidthByte.shuffle(samples[i].Create.Unix()*1000, int64(samples[i].RxBandwidthKbyte)*1024)
		txBandwidthByte.shuffle(samples[i].Create.Unix()*1000, int64(samples[i].TxBandwidthKbyte)*1024)
		rxOpsPm.shuffle(samples[i].Create.Unix()*1000, int64(samples[i].RxOpsPm))
		txOpsPm.shuffle(samples[i].Create.Unix()*1000, int64(samples[i].TxOpsPm))
		delOpsPm.shuffle(samples[i].Create.Unix()*1000, int64(samples[i].DelOpsPm))
	}
	usages := make(map[string]interface{})
	usages["xsky.eos.os_users.samples.allocated.size"] = []*Series{allocatedSize}
	usages["xsky.eos.os_users.samples.allocated.objects"] = []*Series{allocatedObjects}
	usages["xsky.eos.os_users.samples.rx.bandwidth.byte"] = []*Series{rxBandwidthByte}
	usages["xsky.eos.os_users.samples.tx.bandwidth.byte"] = []*Series{txBandwidthByte}
	usages["xsky.eos.os_users.samples.rx.ops.pm"] = []*Series{rxOpsPm}
	usages["xsky.eos.os_users.samples.tx.ops.pm"] = []*Series{txOpsPm}
	usages["xsky.eos.os_users.samples.del.ops.pm"] = []*Series{delOpsPm}
	return usages, nil
}

func (cli *SXskyClient) GetObjectStoreBucketSamples(name, from, interval string) (map[string]interface{}, error) {
	ctx := context.Background()
	samples, err := cli.adminApi.getBucketSamples(ctx, name, from, interval)
	if err != nil {
		return nil, errors.Wrap(err, "api.getObjectStorage GetObjectStoreStats")
	}
	allocatedSize := GetSeries("总容量")
	allocatedObjects := GetSeries("对象数")
	rxBandwidthByte := GetSeries("上传")
	txBandwidthByte := GetSeries("下载")
	rxOpsPm := GetSeries("上传请求")
	txOpsPm := GetSeries("下载请求")
	listOpsPm := GetSeries("列表请求")
	delOpsPm := GetSeries("删除请求")
	upLatencyMs := GetSeries("上传")
	downLatencyMs := GetSeries("下载")
	latencyMs := new(Series)
	{
		series := new(Series)
		series.Name = "延迟统计"
		series.Columns = make([]string, 0)
		series.Columns = append(series.Columns, "上传")
		series.Columns = append(series.Columns, "下载")
		series.Columns = append(series.Columns, "time")
		series.Points = make([][]int64, 0)
		latencyMs = series
	}
	for i := range samples {
		allocatedSize.shuffle(samples[i].Create.Unix()*1000, samples[i].AllocatedSize)
		allocatedObjects.shuffle(samples[i].Create.Unix()*1000, samples[i].AllocatedObjects)
		rxBandwidthByte.shuffle(samples[i].Create.Unix()*1000, int64(samples[i].RxBandwidthKbyte)*1024)
		txBandwidthByte.shuffle(samples[i].Create.Unix()*1000, int64(samples[i].TxBandwidthKbyte)*1024)
		rxOpsPm.shuffle(samples[i].Create.Unix()*1000, int64(samples[i].RxOpsPm))
		txOpsPm.shuffle(samples[i].Create.Unix()*1000, int64(samples[i].TxOpsPm))
		listOpsPm.shuffle(samples[i].Create.Unix()*1000, int64(samples[i].ListOpsPm))
		delOpsPm.shuffle(samples[i].Create.Unix()*1000, int64(samples[i].DelOpsPm))
		upLatencyMs.shuffle(samples[i].Create.Unix()*1000, int64(samples[i].UpLatencyUs/1000))
		downLatencyMs.shuffle(samples[i].Create.Unix()*1000, int64(samples[i].DownLatencyUs/1000))
		latencyMs.shuffle(samples[i].Create.Unix()*1000, int64(samples[i].UpLatencyUs/1000), int64(samples[i].DownLatencyUs/1000))
	}
	usages := make(map[string]interface{})
	usages["xsky.eos.os_buckets.samples.allocated.size"] = []*Series{allocatedSize}
	usages["xsky.eos.os_buckets.samples.allocated.objects"] = []*Series{allocatedObjects}
	usages["xsky.eos.os_buckets.samples.rx.bandwidth.byte"] = []*Series{rxBandwidthByte}
	usages["xsky.eos.os_buckets.samples.tx.bandwidth.byte"] = []*Series{txBandwidthByte}
	usages["xsky.eos.os_buckets.samples.rx.ops.pm"] = []*Series{rxOpsPm}
	usages["xsky.eos.os_buckets.samples.tx.ops.pm"] = []*Series{txOpsPm}
	usages["xsky.eos.os_buckets.samples.list.ops.pm"] = []*Series{listOpsPm}
	usages["xsky.eos.os_buckets.samples.del.ops.pm"] = []*Series{delOpsPm}
	//usages["xsky.eos.os_buckets.samples.latency.ms"] = []*Series{latencyMs}
	usages["xsky.eos.os_buckets.samples.latency.ms"] = []*Series{upLatencyMs, downLatencyMs}
	return usages, nil
}

func (cli *SXskyClient) GetObjectStoreBucketUsages(name string) (map[string]interface{}, error) {
	ctx := context.Background()
	bucket, err := cli.adminApi.getBucketByName(ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, "api.getBucketByName GetObjectStoreBucketUsages")
	}
	usages := make(map[string]interface{})
	usages["xsky.eos.os_buckets.usages.quota.max.objects"] = bucket.QuotaMaxObjects
	usages["xsky.eos.os_buckets.usages.quota.max.size"] = bucket.QuotaMaxSize
	usages["xsky.eos.os_buckets.usages.local.quota.max.objects"] = bucket.LocalQuotaMaxObjects
	usages["xsky.eos.os_buckets.usages.local.quota.max.size"] = bucket.LocalQuotaMaxSize
	usages["xsky.eos.os_buckets.usages.external.quota.max.objects"] = bucket.ExternalQuotaMaxObjects
	usages["xsky.eos.os_buckets.usages.external.quota.max.size"] = bucket.ExternalQuotaMaxSize
	if len(bucket.Samples) > 0 {
		usages["xsky.eos.os_buckets.usages.allocated.objects"] = bucket.Samples[0].AllocatedObjects
		usages["xsky.eos.os_buckets.usages.allocated.size"] = bucket.Samples[0].AllocatedSize
		usages["xsky.eos.os_buckets.usages.local.allocated.objects"] = bucket.Samples[0].LocalAllocatedObjects
		usages["xsky.eos.os_buckets.usages.local.allocated.size"] = bucket.Samples[0].LocalAllocatedSize
		usages["xsky.eos.os_buckets.usages.external.allocated.objects"] = bucket.Samples[0].ExternalAllocatedObjects
		usages["xsky.eos.os_buckets.usages.external.allocated.size"] = bucket.Samples[0].ExternalAllocatedSize
	}

	localStorageClasses := make([]sStorageClasses, 0)
	externalStorageClasses := make([]sStorageClasses, 0)
	for k, v := range bucket.Samples[0].StorageClasses {
		if v.ClassName == "boa" {
			externalStorageClasses = append(externalStorageClasses, bucket.Samples[0].StorageClasses[k])
		} else {
			localStorageClasses = append(localStorageClasses, bucket.Samples[0].StorageClasses[k])
		}
	}
	usages["xsky.eos.os_buckets.usages.local.storage.classes"] = localStorageClasses
	usages["xsky.eos.os_buckets.usages.external.storage.classes"] = externalStorageClasses
	return usages, nil
}

func (cli *SXskyClient) CreateBucket(name string, storageClassStr string, versioned, worm bool, sizeBytesLimit int64, objectCntLimit int, acl string) (s3cli.BucketInfo, error) {
	ctx := context.Background()
	s3Bucket := s3cli.BucketInfo{}
	bucket, err := cli.adminApi.createBucketByName(ctx, cli.adminUser.Id, name, storageClassStr, versioned, worm, sizeBytesLimit, objectCntLimit, acl)
	if err != nil {
		return s3cli.BucketInfo{}, errors.Wrap(err, "api.createBucketByName CreateBucket")
	}
	s3Bucket.Name = bucket.Name
	s3Bucket.CreationDate = bucket.Create
	return s3Bucket, nil
}

func (cli *SXskyClient) CreateUser(name string) (int, error) {
	ctx := context.Background()
	user, err := cli.adminApi.createUser(ctx, name)
	if err != nil {
		return -1, errors.Wrap(err, "api.createUser CreateUser")
	}
	return user.Id, nil
}

func (cli *SXskyClient) GetUserKey(name string) (string, string, error) {
	//ctx := context.Background()
	return cli.ObjectStoreClientConfig.GetAccessKey(), cli.ObjectStoreClientConfig.GetAccessSecret(), nil
	//user, err := cli.adminApi.getUserKeys(ctx, name)
	//if err != nil {
	//	return -1, errors.Wrap(err, "api.createUser CreateUser")
	//}
	//return user.Id, nil
}
