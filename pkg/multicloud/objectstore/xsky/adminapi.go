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
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"yunion.io/x/onecloud/pkg/util/hashcache"
	"yunion.io/x/pkg/gotypes"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/util/httputils"
)

const XMS_API_TOKEN = "bc4addf9febd425886ae95d9a1450b98"

type sManagerDelegate struct {
	buckets   *hashcache.Cache
	users     *hashcache.Cache
	keys      *hashcache.Cache
	userKeys  *hashcache.Cache
	s3LbGroup *hashcache.Cache
}

var managerDelegate *sManagerDelegate

func init() {
	managerDelegate = &sManagerDelegate{
		buckets:   hashcache.NewCache(2048, time.Minute*30),
		users:     hashcache.NewCache(1024, time.Minute*30),
		keys:      hashcache.NewCache(1024, time.Minute*30),
		userKeys:  hashcache.NewCache(1024, time.Minute*30),
		s3LbGroup: hashcache.NewCache(10, time.Hour*24),
	}
}

func (manager *sManagerDelegate) refreshCacheBuckets(buckets []sBucket) {
	for i := range buckets {
		manager.buckets.AtomicSet(buckets[i].Name, &buckets[i])
	}
}

func (manager *sManagerDelegate) getCacheBucket(name string) (*sBucket, error) {
	val := manager.buckets.Get(name)
	if !gotypes.IsNil(val) {
		return val.(*sBucket), nil
	}
	//manager.buckets.AtomicSet(bucket.Name, bucket)
	return nil, nil
}

func (manager *sManagerDelegate) setCacheBucket(bucket *sBucket) {
	manager.buckets.AtomicSet(bucket.Name, bucket)
}

func (manager *sManagerDelegate) refreshCacheUsers(users []sUser) {
	for i := range users {
		manager.users.AtomicSet(users[i].Name, &users[i])
	}
}

func (manager *sManagerDelegate) getCacheUser(name string) (*sUser, error) {
	val := manager.users.Get(name)
	if !gotypes.IsNil(val) {
		return val.(*sUser), nil
	}
	return nil, nil
}

func (manager *sManagerDelegate) setCacheUser(user *sUser) {
	manager.users.AtomicSet(user.Name, user)
}

func (manager *sManagerDelegate) refreshCacheKeys(keys []sKey) {
	for i := range keys {
		manager.keys.AtomicSet(keys[i].AccessKey, &keys[i])
	}
}

func (manager *sManagerDelegate) getCacheKey(ak string) (*sKey, error) {
	val := manager.keys.Get(ak)
	if !gotypes.IsNil(val) {
		return val.(*sKey), nil
	}
	return nil, nil
}

func (manager *sManagerDelegate) setCacheKey(key *sKey) {
	manager.keys.AtomicSet(key.AccessKey, key)
}

func (manager *sManagerDelegate) getCacheUserKey(uid int) ([]sKey, error) {
	val := manager.userKeys.Get(strconv.Itoa(uid))
	if !gotypes.IsNil(val) {
		return val.([]sKey), nil
	}
	return nil, nil
}

func (manager *sManagerDelegate) setCacheUserKey(uid int, keys []sKey) {
	manager.userKeys.AtomicSet(strconv.Itoa(uid), keys)
}

func (manager *sManagerDelegate) getCacheS3LbGroup() (*sS3LbGroupResponse, error) {
	val := manager.s3LbGroup.Get("s3_load_balancer_groups")
	if !gotypes.IsNil(val) {
		return val.(*sS3LbGroupResponse), nil
	}
	return nil, nil
}

func (manager *sManagerDelegate) setCacheS3LbGroup(group *sS3LbGroupResponse) {
	manager.s3LbGroup.AtomicSet("s3_load_balancer_groups", group)
}

type SXskyAdminApi struct {
	endpoint string
	username string
	password string
	token    *sLoginResponse
	client   *http.Client
	debug    bool
}

func newXskyAdminApi(user, passwd, ep string, debug bool) *SXskyAdminApi {
	return &SXskyAdminApi{
		endpoint: ep,
		username: user,
		password: passwd,
		// xsky use notimeout client so as to download/upload large files
		client: httputils.GetAdaptiveTimeoutClient(),
		debug:  debug,
	}
}

func getJsonBodyReader(body jsonutils.JSONObject) io.Reader {
	var reqBody io.Reader
	if body != nil {
		reqBody = strings.NewReader(body.String())
	}
	return reqBody
}

func (api *SXskyAdminApi) httpClient() *http.Client {
	return api.client
}

func (api *SXskyAdminApi) jsonRequest(ctx context.Context, method httputils.THttpMethod, path string, hdr http.Header, body jsonutils.JSONObject) (http.Header, jsonutils.JSONObject, error) {
	urlStr := strings.TrimRight(api.endpoint, "/") + "/" + strings.TrimLeft(path, "/")
	req, err := http.NewRequest(string(method), urlStr, getJsonBodyReader(body))
	if err != nil {
		return nil, nil, errors.Wrap(err, "http.NewRequest")
	}
	if hdr != nil {
		for k, vs := range hdr {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
	}

	req.Header.Set("xms-auth-token", XMS_API_TOKEN)
	//if api.isValidToken() {
	//	req.Header.Set("xms-auth-token", api.token.Token.Uuid)
	//}

	if api.debug {
		log.Debugf("request: %s %s %s %s", method, urlStr, req.Header, body)
	}
	resp, err := api.client.Do(req)

	var bodyStr string
	if body != nil {
		bodyStr = body.String()
	}
	return httputils.ParseJSONResponse(bodyStr, resp, err, api.debug)
}

type sLoginResponse struct {
	Token struct {
		Create  time.Time
		Expires time.Time
		Roles   []string
		User    struct {
			Create             time.Time
			Name               string
			Email              string
			Enabled            bool
			Id                 int
			PasswordLastUpdate time.Time
		}
		Uuid  string
		Valid bool
	}
}

func (api *SXskyAdminApi) isValidToken() bool {
	if api.token != nil && len(api.token.Token.Uuid) > 0 && api.token.Token.Expires.After(time.Now()) {
		return true
	} else {
		return false
	}
}

func (api *SXskyAdminApi) auth(ctx context.Context) (*sLoginResponse, error) {
	input := STokenCreateReq{}
	input.Auth.Identity.Password.User.Name = api.username
	input.Auth.Identity.Password.User.Password = api.password

	_, resp, err := api.jsonRequest(ctx, httputils.POST, "/api/v1/auth/tokens", nil, jsonutils.Marshal(input))
	if err != nil {
		return nil, errors.Wrap(err, "api.jsonRequest")
	}
	output := sLoginResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return nil, errors.Wrap(err, "resp.Unmarshal")
	}
	return &output, err
}

type STokenCreateReq struct {
	Auth STokenCreateReqAuth `json:"auth"`
}

type STokenCreateReqAuth struct {
	Identity STokenCreateReqAuthIdentity `json:"identity"`
}

type STokenCreateReqAuthIdentity struct {
	// password for auth
	Password SAuthPasswordReq `json:"password,omitempty"`
	// token for auth
	Token SAuthTokenReq `json:"token,omitempty"`
}

type SAuthPasswordReq struct {
	User SAuthPasswordReqUser `json:"user"`
}

type SAuthPasswordReqUser struct {
	// user email for auth
	Email string `json:"email,omitempty"`
	// user id for auth
	Id int64 `json:"id,omitzero"`
	// user name or email for auth
	Name string `json:"name,omitempty"`
	// password for auth
	Password string `json:"password"`
}

type SAuthTokenReq struct {
	// uuid of authorized token
	Uuid string `json:"uuid"`
}

func (api *SXskyAdminApi) authRequest(ctx context.Context, method httputils.THttpMethod, path string, hdr http.Header, body jsonutils.JSONObject) (http.Header, jsonutils.JSONObject, error) {
	//if !api.isValidToken() {
	//	loginResp, err := api.auth(ctx)
	//	if err != nil {
	//		return nil, nil, errors.Wrap(err, "api.auth")
	//	}
	//	api.token = loginResp
	//}
	return api.jsonRequest(ctx, method, path, hdr, body)
}

type sUser struct {
	BucketNum             int
	BucketQuotaMaxObjects int
	BucketQuotaMaxSize    int64
	Create                time.Time
	DisplayName           string
	Email                 string
	Id                    int
	MaxBuckets            int
	Name                  string
	OpMask                string
	Status                string
	Suspended             bool
	Update                time.Time
	UserQuotaMaxObjects   int
	UserQuotaMaxSize      int64
	Samples               []sSample
	//Keys                  []sKey
}

//func (u sUser) getMinKey() string {
//	minKey := ""
//	for i := range u.Keys {
//		if len(minKey) == 0 || minKey > u.Keys[i].AccessKey {
//			minKey = u.Keys[i].AccessKey
//		}
//	}
//	return minKey
//}

func (u sUser) getMinKey(keys []sKey) string {
	minKey := ""
	for i := range keys {
		if len(minKey) == 0 || minKey > keys[i].AccessKey {
			minKey = keys[i].AccessKey
		}
	}
	return minKey
}

type sSample struct {
	LocalAllocatedObjects    int64
	ExternalAllocatedSize    int64
	LocalAllocatedSize       int64
	ExternalAllocatedObjects int64
	AllocatedObjects         int64
	AllocatedSize            int64
	Create                   time.Time
	DelOpsPm                 int
	ListOpsPm                int
	DownLatencyUs            int
	UpLatencyUs              int
	RxBandwidthKbyte         int
	RxOpsPm                  int
	TxBandwidthKbyte         int
	TxOpsPm                  int
	TotalDelOps              int
	TotalDelSuccessOps       int
	TotalRxBytes             int64
	TotalRxOps               int
	TotalRxSuccessOps        int
	TotalTxBytes             int64
	TotalTxOps               int
	TotalTxSuccessKbyte      int
	StorageClasses           map[string]sStorageClasses
}

type sStorageClasses struct {
	AllocatedObjects int64
	AllocatedSize    int64
	ClassName        string
}

type sPaging struct {
	Count      int
	Limit      int
	Offset     int
	TotalCount int
}

type sUsersResponse struct {
	OsUsers []sUser
	Paging  sPaging
}

type UserSortObjectsSet []sUser

func (users UserSortObjectsSet) Len() int {
	return len(users)
}

func (users UserSortObjectsSet) Swap(i, j int) {
	users[i], users[j] = users[j], users[i]
}

func (users UserSortObjectsSet) Less(i, j int) bool {
	return users[i].Samples[0].AllocatedObjects > users[j].Samples[0].AllocatedObjects
}

type UserSortSizeSet []sUser

func (users UserSortSizeSet) Len() int {
	return len(users)
}

func (users UserSortSizeSet) Swap(i, j int) {
	users[i], users[j] = users[j], users[i]
}

func (users UserSortSizeSet) Less(i, j int) bool {
	return users[i].Samples[0].AllocatedSize > users[j].Samples[0].AllocatedSize
}

type UserSortBucketSet []sUser

func (users UserSortBucketSet) Len() int {
	return len(users)
}

func (users UserSortBucketSet) Swap(i, j int) {
	users[i], users[j] = users[j], users[i]
}

func (users UserSortBucketSet) Less(i, j int) bool {
	return users[i].BucketNum > users[j].BucketNum
}

func (api *SXskyAdminApi) getUsers(ctx context.Context) ([]sUser, error) {
	usrs := make([]sUser, 0)
	totalCount := 0
	for totalCount <= 0 || len(usrs) < totalCount {
		_, resp, err := api.authRequest(ctx, httputils.GET, fmt.Sprintf("/api/v1/os-users/?limit=1000&offset=%d", len(usrs)), nil, nil)
		if err != nil {
			return nil, errors.Wrap(err, "api.authRequest")
		}
		output := sUsersResponse{}
		err = resp.Unmarshal(&output)
		if err != nil {
			return nil, errors.Wrap(err, "resp.Unmarshal")
		}
		usrs = append(usrs, output.OsUsers...)
		totalCount = output.Paging.TotalCount
	}
	return usrs, nil
}

func (api *SXskyAdminApi) getUserByNameId(ctx context.Context, name string, id int) (*sUser, error) {
	user, _ := managerDelegate.getCacheUser(name)
	if user != nil {
		return user, nil
	}
	u, err := api.getUserById(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, "api.authRequest.getUserByNameId")
	}
	//managerDelegate.setCacheUser(u)

	return u, nil
}

func (api *SXskyAdminApi) getUserById(ctx context.Context, id int) (*sUser, error) {
	_, resp, err := api.authRequest(ctx, httputils.GET, fmt.Sprintf("/api/v1/os-users/%d", id), nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "api.authRequest.getUserById")
	}
	output := new(sUserResponse)
	err = resp.Unmarshal(output)
	if err != nil {
		return nil, errors.Wrap(err, "resp.Unmarshal")
	}
	return &output.OsUser, nil
}

type patchUserInput struct {
	OsUser struct {
		OpMask     string
		MaxBuckets int
	}
}

func (api *SXskyAdminApi) patchUserById(ctx context.Context, id int, maxBuckets int) (*sUser, error) {
	input := &patchUserInput{}
	input.OsUser.OpMask = OPS_ALL
	input.OsUser.MaxBuckets = maxBuckets
	_, resp, err := api.authRequest(ctx, httputils.PATCH, fmt.Sprintf("/api/v1/os-users/%d", id), nil, jsonutils.Marshal(input))
	if err != nil {
		return nil, errors.Wrap(err, "api.authRequest.getUserById")
	}
	output := new(sUserResponse)
	err = resp.Unmarshal(output)
	if err != nil {
		return nil, errors.Wrap(err, "resp.Unmarshal")
	}
	return &output.OsUser, nil
}

type sUserSamplesResponse struct {
	OsUserSamples []sSample
}

func (api *SXskyAdminApi) getUsersSamples(ctx context.Context, from, interval string, id int) ([]sSample, error) {
	h, _ := strconv.Atoi(strings.TrimSuffix(from, "h"))
	durationBegin := url.QueryEscape(time.Now().Add(time.Duration(-h) * time.Hour).Format(time.RFC3339))
	//durationBegin := time.Now().Add(time.Duration(-h) * time.Hour).String()
	durationEnd := url.QueryEscape(time.Now().Format(time.RFC3339))
	_, resp, err := api.authRequest(ctx, httputils.GET, fmt.Sprintf("/api/v1/os-users/%d/samples?duration_begin=%s&duration_end=%s&period=%s", id, durationBegin, durationEnd, interval), nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "api.authRequest")
	}
	output := sUserSamplesResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return nil, errors.Wrap(err, "resp.Unmarshal")
	}

	return output.OsUserSamples, nil
}

type sBucketSamplesResponse struct {
	OsBucketSamples []sSample
}

func (api *SXskyAdminApi) getBucketSamples(ctx context.Context, name, from, interval string) ([]sSample, error) {
	bucket, err := api.getBucketByName(ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, "api.getBucketByName")
	}
	h, _ := strconv.Atoi(strings.TrimSuffix(from, "h"))
	durationBegin := url.QueryEscape(time.Now().Add(time.Duration(-h) * time.Hour).Format(time.RFC3339))
	//durationBegin := time.Now().Add(time.Duration(-h) * time.Hour).String()
	durationEnd := url.QueryEscape(time.Now().Format(time.RFC3339))
	_, resp, err := api.authRequest(ctx, httputils.GET, fmt.Sprintf("/api/v1/os-buckets/%d/samples?duration_begin=%s&duration_end=%s&period=%s", bucket.Id, durationBegin, durationEnd, interval), nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "api.authRequest")
	}
	output := sBucketSamplesResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return nil, errors.Wrap(err, "resp.Unmarshal")
	}

	return output.OsBucketSamples, nil
}

type sKey struct {
	AccessKey string
	Create    time.Time
	Id        int
	Reserved  bool
	SecretKey string
	Status    string
	Type      string
	Update    time.Time
	User      struct {
		Id   int
		Name string
	}
}

type sKeysResponse struct {
	OsKeys []sKey
	Paging sPaging
}

func (api *SXskyAdminApi) getKeys(ctx context.Context) ([]sKey, error) {
	keys := make([]sKey, 0)
	totalCount := 0
	for totalCount <= 0 || len(keys) < totalCount {
		_, resp, err := api.authRequest(ctx, httputils.GET, fmt.Sprintf("/api/v1/os-keys/?limit=1000&offset=%d", len(keys)), nil, nil)
		if err != nil {
			return nil, errors.Wrap(err, "api.authRequest")
		}
		output := sKeysResponse{}
		err = resp.Unmarshal(&output)
		if err != nil {
			return nil, errors.Wrap(err, "resp.Unmarshal")
		}
		keys = append(keys, output.OsKeys...)
		totalCount = output.Paging.TotalCount
	}
	return keys, nil
}

func (api *SXskyAdminApi) getUserKeys(ctx context.Context, id int) ([]sKey, error) {
	userKeys, _ := managerDelegate.getCacheUserKey(id)
	if userKeys != nil {
		return userKeys, nil
	}
	keys := make([]sKey, 0)
	totalCount := 0
	for totalCount <= 0 || len(keys) < totalCount {
		_, resp, err := api.authRequest(ctx, httputils.GET, fmt.Sprintf("/api/v1/os-keys/?limit=1000&offset=%d&user_id=%d", len(keys), id), nil, nil)
		if err != nil {
			return nil, errors.Wrap(err, "api.authRequest")
		}
		output := sKeysResponse{}
		err = resp.Unmarshal(&output)
		if err != nil {
			return nil, errors.Wrap(err, "resp.Unmarshal")
		}
		keys = append(keys, output.OsKeys...)
		totalCount = output.Paging.TotalCount
	}
	managerDelegate.setCacheUserKey(id, keys)
	return keys, nil
}

func (api *SXskyAdminApi) getUserByAccessKey(ctx context.Context, accessKey string) (*sUser, *sKey, error) {
	key, _ := managerDelegate.getCacheKey(accessKey)
	if key != nil {
		user, err := api.getUserByNameId(ctx, key.User.Name, key.User.Id)
		if err != nil {
			return nil, nil, errors.Wrap(err, "api.authRequest.getUserByNameId")
		}
		return user, key, nil
	}

	//return nil, nil, httperrors.ErrNotFound
	return api.findUserByAccessKey(ctx, accessKey)
}

func (api *SXskyAdminApi) findUserByAccessKey(ctx context.Context, accessKey string) (*sUser, *sKey, error) {
	// update user cache
	users, err := api.getUsers(ctx)
	//for i := range users {
	//	_, err := api.patchUserById(ctx, users[i].Id)
	//	if err != nil {
	//		return nil, nil, errors.Wrap(err, "api.patchUserById")
	//	}
	//}
	if err != nil {
		return nil, nil, errors.Wrap(err, "api.getUsers")
	}
	managerDelegate.refreshCacheUsers(users)
	// update key cache
	keys, err := api.getKeys(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "api.getKeys")
	}
	managerDelegate.refreshCacheKeys(keys)

	for i := range users {
		keys, err := api.getUserKeys(ctx, users[i].Id)
		if err != nil {
			return nil, nil, errors.Wrap(err, "api.getUserKeys")
		}
		for j := range keys {
			if keys[j].AccessKey == accessKey {
				return &users[i], &keys[j], nil
			}
		}
	}
	return nil, nil, httperrors.ErrNotFound
}

func (api *SXskyAdminApi) findFirstUserWithAccessKey(ctx context.Context) (*sUser, *sKey, error) {
	users, err := api.getUsers(ctx)
	//for i := range users {
	//	_, err := api.patchUserById(ctx, users[i].Id)
	//	if err != nil {
	//		return nil, nil, errors.Wrap(err, "api.patchUserById")
	//	}
	//}
	if err != nil {
		return nil, nil, errors.Wrap(err, "api.getUsers")
	}
	for i := range users {
		keys, err := api.getUserKeys(ctx, users[i].Id)
		if err != nil {
			return nil, nil, errors.Wrap(err, "api.getUserKeys")
		}
		if len(keys) > 0 {
			return &users[i], &keys[0], nil
		}
	}
	return nil, nil, httperrors.ErrNotFound
}

type SFlag struct {
	Versioned         bool
	VersionsSuspended bool
	Worm              bool
}

type sBucket struct {
	ActionStatus       string
	AllUserPermission  string
	AuthUserPermission string
	BucketPolicy       string
	Create             time.Time
	Flag               struct {
		Versioned         bool
		VersionsSuspended bool
		Worm              bool
	}
	Id int
	// LifeCycle
	MetadataSearchEnabled bool
	Name                  string
	NfsClientNum          int
	OsReplicationPathNum  int
	OsReplicationZoneNum  int
	// osZone
	OsZoneUuid string
	Owner      struct {
		Id   string
		Name string
	}
	OwnerPermission         string
	Policy                  sPolicy
	PolicyEnabled           bool
	QuotaMaxObjects         int
	QuotaMaxSize            int64
	LocalQuotaMaxObjects    int
	LocalQuotaMaxSize       int64
	ExternalQuotaMaxObjects int
	ExternalQuotaMaxSize    int64
	// RemteClusters
	ReplicationUuid string
	Samples         []sSample
	Shards          int
	Status          string
	Update          time.Time
	Virtual         bool
	// NfsGatewayMaps
}

type sPolicy struct {
	BucketNum int
	Compress  bool
	Create    time.Time
	Crypto    bool
	DataPool  struct {
		Id   int
		Name string
	}
	Default     bool
	Description string
	Id          int
	IndexPool   struct {
		Id   int
		Name string
	}
	Name                string
	ObjectSizeThreshold int64
	PolicyName          string
	Status              string
	Update              time.Time
}

type sBucketsResponse struct {
	OsBuckets []sBucket
	Paging    sPaging
}

type BucketSortObjectsSet []sBucket

func (buckets BucketSortObjectsSet) Len() int {
	return len(buckets)
}

func (buckets BucketSortObjectsSet) Swap(i, j int) {
	buckets[i], buckets[j] = buckets[j], buckets[i]
}

func (buckets BucketSortObjectsSet) Less(i, j int) bool {
	return buckets[i].Samples[0].AllocatedObjects > buckets[j].Samples[0].AllocatedObjects
}

type BucketSortSizeSet []sBucket

func (buckets BucketSortSizeSet) Len() int {
	return len(buckets)
}

func (buckets BucketSortSizeSet) Swap(i, j int) {
	buckets[i], buckets[j] = buckets[j], buckets[i]
}

func (buckets BucketSortSizeSet) Less(i, j int) bool {
	return buckets[i].Samples[0].AllocatedSize > buckets[j].Samples[0].AllocatedSize
}

func (api *SXskyAdminApi) getBuckets(ctx context.Context, refresh bool) ([]sBucket, error) {
	buckets := make([]sBucket, 0)
	totalCount := 0
	for totalCount <= 0 || len(buckets) < totalCount {
		_, resp, err := api.authRequest(ctx, httputils.GET, fmt.Sprintf("/api/v1/os-buckets/?limit=1000&offset=%d", len(buckets)), nil, nil)
		if err != nil {
			return nil, errors.Wrap(err, "api.authRequest")
		}
		output := sBucketsResponse{}
		err = resp.Unmarshal(&output)
		if err != nil {
			return nil, errors.Wrap(err, "resp.Unmarshal")
		}
		buckets = append(buckets, output.OsBuckets...)
		totalCount = output.Paging.TotalCount
	}
	if refresh {
		managerDelegate.refreshCacheBuckets(buckets)
	}
	return buckets, nil
}

func (api *SXskyAdminApi) getBucketByName(ctx context.Context, name string) (*sBucket, error) {
	bucket, _ := managerDelegate.getCacheBucket(name)
	if bucket != nil {
		return bucket, nil
	}
	_, resp, err := api.authRequest(ctx, httputils.GET, fmt.Sprintf("/api/v1/os-buckets/?name=%s", name), nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "api.authRequest.getBucketByName")
	}
	output := sBucketsResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return nil, errors.Wrap(err, "resp.Unmarshal.getBucketByName")
	}
	if len(output.OsBuckets) > 0 {
		managerDelegate.setCacheBucket(&output.OsBuckets[0])
		return &output.OsBuckets[0], nil
	}

	//buckets, err := api.getBuckets(ctx, true)
	//if err != nil {
	//	return nil, errors.Wrap(err, "api.getBuckets")
	//}
	//for i := range buckets {
	//	if buckets[i].Name == name {
	//		return &buckets[i], nil
	//	}
	//}
	return nil, cloudprovider.ErrNotFound
}

type createBucketInput struct {
	OsBucket createBucketDetail
}

type createBucketDetail struct {
	Flag struct {
		Versioned         bool
		VersionsSuspended bool
		Worm              bool
	}
	Name                    string
	OwnerId                 int
	PolicyId                int
	QuotaMaxObjects         int
	QuotaMaxSize            int64
	LocalQuotaMaxObjects    int
	LocalQuotaMaxSize       int64
	ExternalQuotaMaxObjects int
	ExternalQuotaMaxSize    int64
}

type sBucketResponse struct {
	OsBucket sBucket
}

func (api *SXskyAdminApi) createBucketByName(ctx context.Context, userId int, name string, storageClassStr string, versioned, worm bool, sizeBytesLimit int64, objectCntLimit int, acl string) (*sBucket, error) {

	input := &createBucketInput{}
	input.OsBucket.Name = name
	input.OsBucket.OwnerId = userId
	input.OsBucket.PolicyId = 1
	input.OsBucket.Flag.Versioned = versioned
	input.OsBucket.Flag.Worm = worm
	input.OsBucket.QuotaMaxObjects = objectCntLimit
	input.OsBucket.QuotaMaxSize = sizeBytesLimit
	_, resp, err := api.authRequest(ctx, httputils.POST, "/api/v1/os-buckets/", nil, jsonutils.Marshal(input))
	if err != nil {
		return nil, errors.Wrap(err, "api.authRequest")
	}
	output := new(sBucketResponse)
	err = resp.Unmarshal(output)
	if err != nil {
		return nil, errors.Wrap(err, "resp.Unmarshal.createBucketByName")
	}
	managerDelegate.setCacheBucket(&output.OsBucket)
	return &output.OsBucket, nil
}

type createUserInput struct {
	OsUser struct {
		Name       string
		MaxBuckets int
	}
}

type sUserResponse struct {
	OsUser sUser
}

func (api *SXskyAdminApi) createUser(ctx context.Context, name string) (*sUser, error) {

	input := &createUserInput{}
	input.OsUser.Name = name
	input.OsUser.MaxBuckets = 1
	_, resp, err := api.authRequest(ctx, httputils.POST, "/api/v1/os-users/", nil, jsonutils.Marshal(input))
	if err != nil {
		return nil, errors.Wrap(err, "api.authRequest os-users")
	}
	output := new(sUserResponse)
	err = resp.Unmarshal(output)
	if err != nil {
		return nil, errors.Wrap(err, "resp.Unmarshal.createUser")
	}
	//keys, err := api.getUserKeys(ctx, output.OsUser.Id)
	//if err != nil {
	//	return nil, errors.Wrap(err, "api.getUserKeys")
	//}
	return &output.OsUser, nil
}

type sBucketQuotaInput struct {
	OsBucket struct {
		QuotaMaxSize    int64
		QuotaMaxObjects int
	}
}

func (api *SXskyAdminApi) setBucketQuota(ctx context.Context, bucketId int, input sBucketQuotaInput) error {
	_, _, err := api.authRequest(ctx, httputils.PATCH, fmt.Sprintf("/api/v1/os-buckets/%d", bucketId), nil, jsonutils.Marshal(&input))
	if err != nil {
		return errors.Wrap(err, "api.authRequest")
	}
	return nil
}

type sObjectStorage struct {
	ActionStatus string
	Create       time.Time
	Update       time.Time
	Id           int
	Name         string
	IndexPool    struct {
		Id   int
		Name string
	}
	DnsNames []string
	Samples  []sObjectStorageSample
	Status   string
}

type sObjectStorageSample struct {
	LocalAllocatedObjects    int64
	ExternalAllocatedSize    int64
	AllocatedSize            int64
	Create                   time.Time
	LocalAllocatedSize       int64
	ExternalAllocatedObjects int64
	AllocatedObjects         int64
}

type sObjectStorageResponse struct {
	ObjectStorage sObjectStorage
}

func (api *SXskyAdminApi) getObjectStorage(ctx context.Context) (*sObjectStorage, error) {
	_, resp, err := api.authRequest(ctx, httputils.GET, fmt.Sprint("/api/v1/cluster/object-storage"), nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "api.authRequest getObjectStorage")
	}
	output := sObjectStorageResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return nil, errors.Wrap(err, "resp.Unmarshal sObjectStorageResponse")
	}
	return &output.ObjectStorage, nil
}

type sS3LbGroup struct {
	ActionStatus    string
	Create          time.Time
	Description     string
	HttpsPort       int
	Id              int
	Name            string
	Port            int
	Roles           []string
	SearchHttpsPort int
	SearchPort      int
	Status          string
	SyncPort        int
	Update          time.Time
	S3LoadBalancers []sS3LoadBalancer `json:"s3_load_balancers"`
}

type sS3LoadBalancer struct {
	Create      time.Time
	Description string
	Group       struct {
		Id     int
		Name   string
		Status string
	}
	Host struct {
		AdminIp string
		Id      int
		Name    string
	}
	HttpsPort     int
	Id            int
	InterfaceName string
	Ip            string
	Name          string
	Port          int
	Roles         []string
	Samples       []struct {
		ActiveAconnects     int
		CpuUtil             float64
		Create              time.Time
		DownBandwidthKbytes int64
		FailureRequests     int
		MemUsagePercent     float64
		SuccessRequests     int
		UpBandwidthKbyte    int64
	}
	SearchHttpsPort int
	SearchPort      int
	//SslCertificate  interface{}
	SslCertificate struct {
		ForceHttps bool
		Id         int
		Name       string
	}
	Status   string
	SyncPort int
	Update   time.Time
	Vip      string
	VipMask  int
	Vips     string
}

func (lb sS3LoadBalancer) GetGatewayEndpoint() string {
	if lb.SslCertificate.ForceHttps == false {
		return fmt.Sprintf("http://%s:%d", lb.Vip, lb.Port)
	} else {
		return fmt.Sprintf("https://%s:%d", lb.Vip, lb.HttpsPort)
	}
}

type sS3LbGroupResponse struct {
	S3LoadBalancerGroups []sS3LbGroup `json:"s3_load_balancer_groups"`
	Paging               sPaging
}

func (api *SXskyAdminApi) getS3LbGroup(ctx context.Context) ([]sS3LbGroup, error) {
	lbGroups := make([]sS3LbGroup, 0)
	totalCount := 0
	for totalCount <= 0 || len(lbGroups) < totalCount {
		_, resp, err := api.authRequest(ctx, httputils.GET, fmt.Sprintf("/api/v1/s3-load-balancer-groups/?limit=1000&offset=%d", len(lbGroups)), nil, nil)
		if err != nil {
			return nil, errors.Wrap(err, "api.authRequest")
		}
		output := sS3LbGroupResponse{}
		err = resp.Unmarshal(&output)
		if err != nil {
			return nil, errors.Wrap(err, "resp.Unmarshal")
		}
		managerDelegate.setCacheS3LbGroup(&output)
		lbGroups = append(lbGroups, output.S3LoadBalancerGroups...)
		totalCount = output.Paging.TotalCount
	}
	return lbGroups, nil
}

func (api *SXskyAdminApi) getS3GatewayEndpoint(ctx context.Context) (string, error) {
	var err error
	s3LbGrps := make([]sS3LbGroup, 0)
	cache, _ := managerDelegate.getCacheS3LbGroup()
	if cache == nil {
		s3LbGrps, err = api.getS3LbGroup(ctx)
		if err != nil {
			return "", errors.Wrap(err, "api.getS3LbGroup")
		}
	} else {
		s3LbGrps = append(s3LbGrps, cache.S3LoadBalancerGroups...)
	}
	lbs := make([]sS3LoadBalancer, 0)
	for i := range s3LbGrps {
		lbs = append(lbs, s3LbGrps[i].S3LoadBalancers...)
	}
	if len(lbs) == 0 {
		return "", errors.Wrap(httperrors.ErrNotFound, "empty S3 Lb group")
	}
	lb := lbs[rand.Intn(len(lbs))]
	return lb.GetGatewayEndpoint(), nil
}
