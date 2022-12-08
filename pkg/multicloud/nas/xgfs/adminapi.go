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

package xgfs

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/util/hashcache"
	"yunion.io/x/pkg/gotypes"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/util/httputils"
)

type sManagerDelegate struct {
	files           *hashcache.Cache
	nfsShares       *hashcache.Cache
	clients         *hashcache.Cache
	dfsGatewayGroup *hashcache.Cache
}

var managerDelegate *sManagerDelegate

func init() {
	managerDelegate = &sManagerDelegate{
		files:           hashcache.NewCache(2048, time.Minute*15),
		nfsShares:       hashcache.NewCache(2048, time.Minute*15),
		clients:         hashcache.NewCache(2048, time.Minute*15),
		dfsGatewayGroup: hashcache.NewCache(10, time.Hour*24),
	}
}

func (manager *sManagerDelegate) refreshCacheFiles(files []sFile) {
	for i := range files {
		manager.files.AtomicSet(files[i].Name, &files[i])
	}
}

func (manager *sManagerDelegate) getCacheFile(name string) (*sFile, error) {
	val := manager.files.Get(name)
	if !gotypes.IsNil(val) {
		return val.(*sFile), nil
	}
	return nil, nil
}

func (manager *sManagerDelegate) setCacheFile(file *sFile) {
	manager.files.AtomicSet(file.Name, file)
}

func (manager *sManagerDelegate) refreshCacheShares(shares []sDfsNfsShare) {
	for i := range shares {
		manager.nfsShares.AtomicSet(shares[i].Name, &shares[i])
	}
}

func (manager *sManagerDelegate) getCacheShare(name string) (*sDfsNfsShare, error) {
	val := manager.nfsShares.Get(name)
	if !gotypes.IsNil(val) {
		return val.(*sDfsNfsShare), nil
	}
	return nil, nil
}

func (manager *sManagerDelegate) setCacheShare(share *sDfsNfsShare) {
	manager.nfsShares.AtomicSet(share.Name, share)
}

//func (manager *sManagerDelegate) refreshCacheClients(clients []sClient) {
//	for i := range keys {
//		manager.keys.AtomicSet(keys[i].AccessKey, &keys[i])
//	}
//}
//
//func (manager *sManagerDelegate) getCacheKey(ak string) (*sKey, error) {
//	val := manager.keys.Get(ak)
//	if !gotypes.IsNil(val) {
//		return val.(*sKey), nil
//	}
//	return nil, nil
//}
//
//func (manager *sManagerDelegate) setCacheKey(key *sKey) {
//	manager.keys.AtomicSet(key.AccessKey, key)
//}

func (manager *sManagerDelegate) getCacheDfsGatewayGroup() (*sGatewayGroupResponse, error) {
	val := manager.dfsGatewayGroup.Get("dfs_gateway_groups")
	if !gotypes.IsNil(val) {
		return val.(*sGatewayGroupResponse), nil
	}
	return nil, nil
}

func (manager *sManagerDelegate) setCacheDfsGatewayGroup(group *sGatewayGroupResponse) {
	manager.dfsGatewayGroup.AtomicSet("dfs_gateway_groups", group)
}

type SXgfsAdminApi struct {
	endpoint string
	username string
	password string
	xmsToken string
	token    *sLoginResponse
	client   *http.Client
	debug    bool
}

func newXgfsAdminApi(user, passwd, ep, token string, debug bool) *SXgfsAdminApi {
	return &SXgfsAdminApi{
		endpoint: ep,
		username: user,
		password: passwd,
		xmsToken: token,
		client:   httputils.GetDefaultClient(),
		debug:    debug,
	}
}

func getJsonBodyReader(body jsonutils.JSONObject) io.Reader {
	var reqBody io.Reader
	if body != nil {
		reqBody = strings.NewReader(body.String())
	}
	return reqBody
}

func (api *SXgfsAdminApi) httpClient() *http.Client {
	return api.client
}

func (api *SXgfsAdminApi) jsonRequest(ctx context.Context, method httputils.THttpMethod, path string, hdr http.Header, body jsonutils.JSONObject) (http.Header, jsonutils.JSONObject, error) {
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

	req.Header.Set("xms-auth-token", api.xmsToken)
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

func (api *SXgfsAdminApi) isValidToken() bool {
	if api.token != nil && len(api.token.Token.Uuid) > 0 && api.token.Token.Expires.After(time.Now()) {
		return true
	} else {
		return false
	}
}

func (api *SXgfsAdminApi) auth(ctx context.Context) (*sLoginResponse, error) {
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

func (api *SXgfsAdminApi) authRequest(ctx context.Context, method httputils.THttpMethod, path string, hdr http.Header, body jsonutils.JSONObject) (http.Header, jsonutils.JSONObject, error) {
	//if !api.isValidToken() {
	//	loginResp, err := api.auth(ctx)
	//	if err != nil {
	//		return nil, nil, errors.Wrap(err, "api.auth")
	//	}
	//	api.token = loginResp
	//}
	return api.jsonRequest(ctx, method, path, hdr, body)
}

type sCluster struct {
	Create  time.Time
	Update  time.Time
	Default bool
	Version string
	Status  string
	FsId    string
	Name    string
	Type    string
}

type sClusterResponse struct {
	Cluster sCluster
}

func (api *SXgfsAdminApi) getCluster(ctx context.Context) (sCluster, error) {
	_, resp, err := api.authRequest(ctx, httputils.GET, fmt.Sprintf("/api/v1/cluster"), nil, nil)
	if err != nil {
		return sCluster{}, errors.Wrap(err, "api.authRequest")
	}
	output := sClusterResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return sCluster{}, errors.Wrap(err, "resp.Unmarshal")
	}

	return output.Cluster, nil
}

type sDfsRootFs struct {
	Create time.Time
	Update time.Time
	Status string
	Id     int
	Name   string
}

type sRootFsResponse struct {
	DfsRootfses []sDfsRootFs
	Paging      sPaging
}

func (api *SXgfsAdminApi) getRootFses(ctx context.Context) ([]sDfsRootFs, error) {
	_, resp, err := api.authRequest(ctx, httputils.GET, fmt.Sprintf("/api/v1/dfs-rootfses/?limit=-1"), nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "api.authRequest")
	}
	output := sRootFsResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return nil, errors.Wrap(err, "resp.Unmarshal")
	}

	return output.DfsRootfses, nil
}

type sPaging struct {
	Count      int
	Limit      int
	Offset     int
	TotalCount int
}

type sDfsNfsShare struct {
	Create  time.Time
	Update  time.Time
	Cluster struct {
		FsId string
		Id   int
		Name string
	}
	DfsGatewayGroup struct {
		Id   int
		Name string
	}
	DfsPath struct {
		Id   int
		Name string
	}
	DfsRootfs struct {
		Id   int
		Name string
	}
	Id           int
	Name         string
	Status       string
	ActionStatus string
}

type sDfsNfsSharesResponse struct {
	DfsNfsShares []sDfsNfsShare
	Paging       sPaging
}

func (api *SXgfsAdminApi) getDfsNfsShares(ctx context.Context, path string) ([]sDfsNfsShare, error) {
	shares := make([]sDfsNfsShare, 0)
	totalCount := 0
	for totalCount <= 0 || len(shares) < totalCount {
		_, resp, err := api.authRequest(ctx, httputils.GET, fmt.Sprintf("/api/v1/dfs-nfs-shares/?limit=1000&offset=%d&path=%s", len(shares), path), nil, nil)
		if err != nil {
			return nil, errors.Wrap(err, "api.authRequest")
		}
		output := sDfsNfsSharesResponse{}
		err = resp.Unmarshal(&output)
		if err != nil {
			return nil, errors.Wrap(err, "resp.Unmarshal")
		}
		totalCount = output.Paging.TotalCount
		if totalCount == 0 {
			return nil, nil
		}
		shares = append(shares, output.DfsNfsShares...)
	}
	return shares, nil
}

type createDfsNfsShareResponse struct {
	DfsNfsShare sDfsNfsShare
}

func (api *SXgfsAdminApi) createDfsNfsShare(ctx context.Context, opts *cloudprovider.SNfsShareCreateOptions) (sDfsNfsShare, error) {
	fses, err := api.getRootFses(ctx)
	if err != nil {
		return sDfsNfsShare{}, errors.Wrap(err, "api.getRootFses")
	}
	if len(fses) == 0 {
		return sDfsNfsShare{}, fmt.Errorf("no root fs found")
	}

	opts.DfsRootfsId = fses[0].Id
	body := jsonutils.NewDict()
	body.Set("dfs_nfs_share", jsonutils.Marshal(opts))
	_, resp, err := api.authRequest(ctx, httputils.POST, fmt.Sprintf("/api/v1/dfs-nfs-shares/?allow_path_create=false"), nil, body)
	if err != nil {
		return sDfsNfsShare{}, errors.Wrap(err, "api.authRequest")
	}
	output := createDfsNfsShareResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return sDfsNfsShare{}, errors.Wrap(err, "resp.Unmarshal")
	}

	return output.DfsNfsShare, nil
}

func (api *SXgfsAdminApi) deleteDfsNfsShare(ctx context.Context, shareId int) (sDfsNfsShare, error) {
	_, resp, err := api.authRequest(ctx, httputils.DELETE, fmt.Sprintf("/api/v1/dfs-nfs-shares/%d?with_directory=false", shareId), nil, nil)
	if err != nil {
		return sDfsNfsShare{}, errors.Wrap(err, "api.authRequest")
	}
	output := createDfsNfsShareResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return sDfsNfsShare{}, errors.Wrap(err, "resp.Unmarshal")
	}

	return output.DfsNfsShare, nil
}

func (api *SXgfsAdminApi) addDfsNfsShareAcl(ctx context.Context, opts *cloudprovider.SNfsShareAclAddOptions, shareId int) (sDfsNfsShare, error) {
	body := jsonutils.NewDict()
	body.Set("dfs_nfs_share", jsonutils.Marshal(opts))
	path := fmt.Sprintf("/api/v1/dfs-nfs-shares/%d:add-acls", shareId)
	_, resp, err := api.authRequest(ctx, httputils.POST, path, nil, body)
	if err != nil {
		return sDfsNfsShare{}, errors.Wrap(err, "api.authRequest")
	}
	output := createDfsNfsShareResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return sDfsNfsShare{}, errors.Wrap(err, "resp.Unmarshal")
	}

	return output.DfsNfsShare, nil
}

func (api *SXgfsAdminApi) updateDfsNfsShareAcl(ctx context.Context, opts *cloudprovider.SNfsShareAclUpdateOptions, shareId int) (sDfsNfsShare, error) {
	body := jsonutils.NewDict()
	body.Set("dfs_nfs_share", jsonutils.Marshal(opts))
	path := fmt.Sprintf("/api/v1/dfs-nfs-shares/%d:update-acls", shareId)
	_, resp, err := api.authRequest(ctx, httputils.POST, path, nil, body)
	if err != nil {
		return sDfsNfsShare{}, errors.Wrap(err, "api.authRequest")
	}
	output := createDfsNfsShareResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return sDfsNfsShare{}, errors.Wrap(err, "resp.Unmarshal")
	}

	return output.DfsNfsShare, nil
}

func (api *SXgfsAdminApi) removeDfsNfsShareAcl(ctx context.Context, opts *cloudprovider.SNfsShareAclRemoveOptions, shareId int) (sDfsNfsShare, error) {
	body := jsonutils.NewDict()
	body.Set("dfs_nfs_share", jsonutils.Marshal(opts))
	path := fmt.Sprintf("/api/v1/dfs-nfs-shares/%d:remove-acls", shareId)
	_, resp, err := api.authRequest(ctx, httputils.POST, path, nil, body)
	if err != nil {
		return sDfsNfsShare{}, errors.Wrap(err, "api.authRequest")
	}
	output := createDfsNfsShareResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return sDfsNfsShare{}, errors.Wrap(err, "resp.Unmarshal")
	}

	return output.DfsNfsShare, nil
}

type sDfsNfsShareAclsResponse struct {
	DfsNfsShareAcls []sDfsNfsShareAcl
	Paging          sPaging
}

type sDfsNfsShareAcl struct {
	Create  time.Time
	Update  time.Time
	Cluster struct {
		FsId string
		Id   int
		Name string
	}
	DfsNfsShare struct {
		Id   int
		Name string
	}
	FsClient struct {
		Id   int
		Name string
	}
	Id         int
	Permission string
	Type       string
	Sync       bool
	RootSquash bool
	AllSquash  bool
}

func (api *SXgfsAdminApi) getDfsNfsShareAcls(ctx context.Context, shareId int) ([]sDfsNfsShareAcl, error) {
	acls := make([]sDfsNfsShareAcl, 0)
	totalCount := 0
	for totalCount <= 0 || len(acls) < totalCount {
		_, resp, err := api.authRequest(ctx, httputils.GET, fmt.Sprintf("/api/v1/dfs-nfs-share-acls/?limit=1000&offset=%d&dfs_nfs_share_id=%d", len(acls), shareId), nil, nil)
		if err != nil {
			return nil, errors.Wrap(err, "api.authRequest")
		}
		output := sDfsNfsShareAclsResponse{}
		err = resp.Unmarshal(&output)
		if err != nil {
			return nil, errors.Wrap(err, "resp.Unmarshal")
		}
		totalCount = output.Paging.TotalCount
		if totalCount == 0 {
			return nil, fmt.Errorf("share id %s no acl???", shareId)
		}
		acls = append(acls, output.DfsNfsShareAcls...)
	}
	return acls, nil
}

func (api *SXgfsAdminApi) getDfsNfsShareAclsById(ctx context.Context, shareId, id int) ([]sDfsNfsShareAcl, error) {
	acls := make([]sDfsNfsShareAcl, 0)
	_, resp, err := api.authRequest(ctx, httputils.GET, fmt.Sprintf("/api/v1/dfs-nfs-share-acls/?limit=1000&offset=0&dfs_nfs_share_id=%d&q=id:%d", shareId, id), nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "api.authRequest")
	}
	output := sDfsNfsShareAclsResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return nil, errors.Wrap(err, "resp.Unmarshal")
	}
	totalCount := output.Paging.TotalCount
	if totalCount == 0 {
		return nil, fmt.Errorf("share id %d no acl???", id)
	}
	if totalCount > 1 {
		return nil, fmt.Errorf("share id %d has more than 1 acl???", id)
	}
	acls = append(acls, output.DfsNfsShareAcls...)
	return acls, nil
}

type SFlag struct {
	Versioned         bool
	VersionsSuspended bool
	Worm              bool
}

type sFile struct {
	Access    time.Time
	Change    time.Time
	Modify    time.Time
	DfsRootfs struct {
		Id   string
		Name string
	}
	Shared bool
	Shares []string
	Worm   struct {
		AutoLockTime time.Time
		State        string
		ExpireTime   time.Time
	}
	QuotaNum         int
	Name             string
	DfsPathId        int
	TotalSnapshotNum int
	SnapshotNum      int
	Path             string
	Parent           string
	Size             int64
	Inode            int64
	Files            int64
	Type             string
}

type sFilesResponse struct {
	DfsFiles []sFile
	Eof      bool
}

type sFileStatResponse struct {
	DfsFile sFile
}

func (api *SXgfsAdminApi) getRootDfsFile(ctx context.Context, id string) (sFile, error) {
	//id is path like "/" + name
	return api.getDfsFile(ctx, id)
}

func (api *SXgfsAdminApi) getDfsFile(ctx context.Context, path string) (sFile, error) {
	body := jsonutils.NewDict()
	body.Set("path", jsonutils.NewString(path))
	_, resp, err := api.authRequest(ctx, httputils.POST, fmt.Sprintf("/api/v1/dfs-files/:stat"), nil, body)
	if err != nil {
		jerr, ok := err.(*httputils.JSONClientError)
		if ok && jerr.Code == 404 {
			return sFile{}, errors.Wrapf(cloudprovider.ErrNotFound, path)
		}
		return sFile{}, errors.Wrap(err, "api.authRequest")
	}
	output := sFileStatResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return sFile{}, errors.Wrap(err, "resp.Unmarshal")
	}

	return output.DfsFile, nil
}

func (api *SXgfsAdminApi) getRootDfsFiles(ctx context.Context) ([]sFile, error) {
	nas := []sFile{}
	var start string
	for {
		parts, eof, err := api.getDfsFiles(ctx, false, false, false, 100, start, "/", "")
		if err != nil {
			return nil, errors.Wrapf(err, "api.getDfsFiles")
		}
		nas = append(nas, parts...)
		if eof {
			break
		}
		start = nas[len(nas)-1].Name
	}

	return nas, nil
}

func (api *SXgfsAdminApi) getDfsFiles(ctx context.Context, refresh, reverse, hidden bool, limit int, start, path, prefix string) ([]sFile, bool, error) {
	files := make([]sFile, 0)
	body := jsonutils.NewDict()
	body.Set("page_up", jsonutils.NewBool(false))
	body.Set("type", jsonutils.NewString("dir"))
	//custom
	body.Set("limit", jsonutils.NewInt(int64(limit)))
	body.Set("reverse", jsonutils.NewBool(reverse))
	body.Set("hidden", jsonutils.NewBool(hidden))
	body.Set("start", jsonutils.NewString(start))
	body.Set("path", jsonutils.NewString(path))
	body.Set("prefix", jsonutils.NewString(prefix))
	_, resp, err := api.authRequest(ctx, httputils.POST, fmt.Sprintf("/api/v1/dfs-files"), nil, body)
	if err != nil {
		return nil, false, errors.Wrap(err, "api.authRequest")
	}
	output := sFilesResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return nil, false, errors.Wrap(err, "resp.Unmarshal")
	}

	if !output.Eof {
		log.Warningf("path '%s' get dfs files eof false", path)
	}
	files = append(files, output.DfsFiles...)
	if refresh {
		managerDelegate.refreshCacheFiles(files)
	}
	return files, output.Eof, nil
}

type sDirectoryResponse struct {
	DfsDirectory sDfsDirectory
}

type sDfsDirectory struct {
	DfsRootfs       sDfsRootFs
	DirectoryResult sDirectoryResult
}

type sDirectoryResult struct {
	Directory sFile
	Result    string
}

type sDfsDirectoryParam struct {
	DfsRootfsId int
	Path        string
}

func (api *SXgfsAdminApi) createDfsFile(ctx context.Context, path string) (sFile, error) {
	fses, err := api.getRootFses(ctx)
	if err != nil {
		return sFile{}, errors.Wrap(err, "api.getRootFses")
	}
	if len(fses) == 0 {
		return sFile{}, fmt.Errorf("no root fs found")
	}

	params := sDfsDirectoryParam{
		DfsRootfsId: fses[0].Id,
		Path:        path,
	}
	body := jsonutils.NewDict()
	body.Set("dfs_directory", jsonutils.Marshal(params))
	_, resp, err := api.authRequest(ctx, httputils.POST, fmt.Sprintf("/api/v1/dfs-directories/:mkdir"), nil, body)
	if err != nil {
		return sFile{}, errors.Wrap(err, "api.authRequest")
	}
	output := sDirectoryResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return sFile{}, errors.Wrap(err, "resp.Unmarshal")
	}
	if output.DfsDirectory.DirectoryResult.Result != "success" {
		return sFile{}, fmt.Errorf("create 'Result' is %s", output.DfsDirectory.DirectoryResult.Result)
	}

	file := output.DfsDirectory.DirectoryResult.Directory
	if file.Path != path {
		return sFile{}, fmt.Errorf("the file is %s, not equal %s", file.Path, path)
	}

	return file, nil
}

type sDfsDirectoryWithClean struct {
	DfsRootfsId int
	Path        string
	Clean       bool
}

func (api *SXgfsAdminApi) deleteDfsFile(ctx context.Context, path string) (sFile, error) {
	fses, err := api.getRootFses(ctx)
	if err != nil {
		return sFile{}, errors.Wrap(err, "api.getRootFses")
	}
	if len(fses) == 0 {
		return sFile{}, fmt.Errorf("no root fs found")
	}

	params := sDfsDirectoryWithClean{
		DfsRootfsId: fses[0].Id,
		Path:        path,
		Clean:       true,
	}
	body := jsonutils.NewDict()
	body.Set("dfs_directory", jsonutils.Marshal(params))
	_, resp, err := api.authRequest(ctx, httputils.POST, fmt.Sprintf("/api/v1/dfs-directories/:rmdir"), nil, body)
	if err != nil {
		return sFile{}, errors.Wrap(err, "api.authRequest")
	}
	output := sDirectoryResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return sFile{}, errors.Wrap(err, "resp.Unmarshal")
	}
	//if output.DfsDirectory.DirectoryResult.Result != "deleting" {
	//	return sFile{}, fmt.Errorf("delete 'Result' is %s", output.DfsDirectory.DirectoryResult.Result)
	//}

	file := output.DfsDirectory.DirectoryResult.Directory
	if file.Path != path {
		return sFile{}, fmt.Errorf("the file is %s, not equal %s", file.Path, path)
	}

	return file, nil
}

type sQuotaResponse struct {
	DfsQuota sDfsQuota
}

type sDfsQuota struct {
	Create    time.Time
	Update    time.Time
	DfsRootfs struct {
		Id   int
		Name string
	}
	DfsPath struct {
		Id   int
		Name string
	}
	Shared            bool
	Id                int
	FilesHardQuota    int64
	FilesSoftQuota    int64
	FilesSuggestQuota int64
	SizeHardQuota     int64
	SizeSoftQuota     int64
	SizeSuggestQuota  int64
	Inode             int64
	Status            string
	Type              string
}

type sDfsQuotaParam struct {
	DfsRootfsId   int
	Path          string
	SizeHardQuota int64
	Type          string
}

func (api *SXgfsAdminApi) createDfsQuota(ctx context.Context, path string, size int64) (sDfsQuota, error) {
	fses, err := api.getRootFses(ctx)
	if err != nil {
		return sDfsQuota{}, errors.Wrap(err, "api.getRootFses")
	}
	if len(fses) == 0 {
		return sDfsQuota{}, fmt.Errorf("no root fs found")
	}

	params := sDfsQuotaParam{
		DfsRootfsId:   fses[0].Id,
		Path:          path,
		SizeHardQuota: size,
		Type:          "directory",
	}
	body := jsonutils.NewDict()
	body.Set("dfs_quota", jsonutils.Marshal(params))
	_, resp, err := api.authRequest(ctx, httputils.POST, fmt.Sprintf("/api/v1/dfs-quotas?allow_path_create=false"), nil, body)
	if err != nil {
		return sDfsQuota{}, errors.Wrap(err, "api.authRequest")
	}
	output := sQuotaResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return sDfsQuota{}, errors.Wrap(err, "resp.Unmarshal")
	}

	return output.DfsQuota, nil
}

//type createUserInput struct {
//	OsUser struct {
//		Name       string
//		MaxBuckets int
//	}
//}

//type sBucketQuotaInput struct {
//	OsBucket struct {
//		QuotaMaxSize    int64
//		QuotaMaxObjects int
//	}
//}
//
//func (api *SXgfsAdminApi) setBucketQuota(ctx context.Context, bucketId int, input sBucketQuotaInput) error {
//	_, _, err := api.authRequest(ctx, httputils.PATCH, fmt.Sprintf("/api/v1/os-buckets/%d", bucketId), nil, jsonutils.Marshal(&input))
//	if err != nil {
//		return errors.Wrap(err, "api.authRequest")
//	}
//	return nil
//}

type sDfsQuotasResponse struct {
	DfsQuotas []sDfsQuota
	Paging    sPaging
}

func (api *SXgfsAdminApi) getDfsQuotas(ctx context.Context, dfsPathId int) ([]sDfsQuota, error) {
	params := url.Values{}
	params.Add("limit", "-1")
	params.Add("offset", "0")
	params.Add("q", fmt.Sprintf("dfs_path.id:%d AND type:directory", dfsPathId))
	query := params.Encode()
	path := fmt.Sprintf("/api/v1/dfs-quotas/?%s", query)

	_, resp, err := api.authRequest(ctx, httputils.GET, path, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "api.authRequest getDfsQuotas")
	}
	output := sDfsQuotasResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return nil, errors.Wrap(err, "resp.Unmarshal sDfsQuotasResponse")
	}
	return output.DfsQuotas, nil
}

type sFsClientsResponse struct {
	FsClients []sFsClient
	Paging    sPaging
}

type sFsClient struct {
	Create  time.Time
	Update  time.Time
	Cluster struct {
		Id   int
		Name string
	}
	Id   int
	Name string
	Ip   string
}

func (api *SXgfsAdminApi) getFsClientsByIP(ctx context.Context, source string) ([]sFsClient, error) {
	params := url.Values{}
	params.Add("limit", "-1")
	params.Add("offset", "0")
	params.Add("q", fmt.Sprintf(`ip.raw:"%s"`, source))
	query := params.Encode()
	path := fmt.Sprintf("/api/v1/fs-clients/?%s", query)

	_, resp, err := api.authRequest(ctx, httputils.GET, path, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "api.authRequest getFsClients")
	}
	output := sFsClientsResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return nil, errors.Wrap(err, "resp.Unmarshal sFsClientsResponse")
	}
	return output.FsClients, nil
}

func (api *SXgfsAdminApi) getFsClientById(ctx context.Context, id int) (sFsClient, error) {
	path := fmt.Sprintf("/api/v1/fs-clients/%d", id)

	_, resp, err := api.authRequest(ctx, httputils.GET, path, nil, nil)
	if err != nil {
		return sFsClient{}, errors.Wrap(err, "api.authRequest getFsClientById")
	}
	output := struct {
		FsClient sFsClient
	}{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return sFsClient{}, errors.Wrap(err, "resp.Unmarshal getFsClientById")
	}
	return output.FsClient, nil
}

func (api *SXgfsAdminApi) createFsClient(ctx context.Context, name, source string) (sFsClient, error) {
	params := struct {
		Ip   string
		Name string
	}{
		Ip:   source,
		Name: fmt.Sprintf("%x", md5.Sum([]byte(source))),
		//Name: name,
	}
	body := jsonutils.NewDict()
	body.Set("fs_client", jsonutils.Marshal(params))

	_, resp, err := api.authRequest(ctx, httputils.POST, fmt.Sprintf("/api/v1/fs-clients/"), nil, body)
	if err != nil {
		return sFsClient{}, errors.Wrap(err, "api.authRequest createFsClient")
	}
	output := struct {
		FsClient sFsClient
	}{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return sFsClient{}, errors.Wrap(err, "resp.Unmarshal getFsClientById")
	}
	return output.FsClient, nil
}

type sDfsGatewayGroup struct {
	ActionStatus string
	Cluster      struct {
		FsId string
		Id   int
		Name string
	}
	Create           time.Time
	Description      string
	Id               int
	Name             string
	Port             int
	Types            []string
	Status           string
	VipBalanceStatus string
	Update           time.Time
	DfsGateways      []sDfsGateway `json:"dfs_gateways"`
	DfsVips          []sDfsVip     `json:"dfs_vips"`
	GatewayNum       int
	VipNum           int
}

type sDfsVip struct {
	ActionStatus string
	Create       time.Time
	Update       time.Time
	Cluster      struct {
		FsId string
		Id   int
		Name string
	}
	DfsGatewayGroup struct {
		Id   int
		Name string
	}
	Host struct {
		AdminIp string
		Id      int
		Name    string
	}
	Id      int
	Vip     string
	VipMask int
}

type sDfsGateway struct {
	Create  time.Time
	Cluster struct {
		FsId string
		Id   int
		Name string
	}
	DfsGatewayGroup struct {
		Id   int
		Name string
	}
	Host struct {
		AdminIp string
		Id      int
		Name    string
	}
	Id        int
	HeartIp   string
	HeartMask int
	ConnNum   int
	Samples   []struct {
		MemUsagePercent      float64
		CpuUtil              float64
		Create               time.Time
		ReadBandwidthKbytes  float64
		ReadIops             int64
		WriteBandwidthKbytes float64
		WriteIops            int64
	}
	NetworkAddress struct {
		Id   int
		Ip   string
		Mask int
	}
	PublicNetworkAddress struct {
		Id   int
		Ip   string
		Mask int
	}
	Status string
	Update time.Time
}

type sGatewayGroupResponse struct {
	DfsGatewayGroups []sDfsGatewayGroup `json:"dfs_gateway_groups"`
	Paging           sPaging
}

func (api *SXgfsAdminApi) getDfsGatewayGroup(ctx context.Context) ([]sDfsGatewayGroup, error) {
	groups := make([]sDfsGatewayGroup, 0)
	_, resp, err := api.authRequest(ctx, httputils.GET, fmt.Sprintf("/api/v1/dfs-gateway-groups/?limit=-1"), nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "api.authRequest")
	}
	output := sGatewayGroupResponse{}
	err = resp.Unmarshal(&output)
	if err != nil {
		return nil, errors.Wrap(err, "resp.Unmarshal")
	}
	managerDelegate.setCacheDfsGatewayGroup(&output)
	groups = append(groups, output.DfsGatewayGroups...)
	return groups, nil
}

func (api *SXgfsAdminApi) getDfsGateways(ctx context.Context) ([]sDfsGateway, error) {
	var err error
	groups := make([]sDfsGatewayGroup, 0)
	cache, _ := managerDelegate.getCacheDfsGatewayGroup()
	if cache == nil {
		groups, err = api.getDfsGatewayGroup(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "api.getDfsGatewayGroup")
		}
	} else {
		groups = append(groups, cache.DfsGatewayGroups...)
	}
	gateways := make([]sDfsGateway, 0)
	for i := range groups {
		gateways = append(gateways, groups[i].DfsGateways...)
	}
	if len(gateways) == 0 {
		return nil, errors.Wrap(httperrors.ErrNotFound, "empty dfs gateways")
	}
	//gateway := gateways[rand.Intn(len(gateways))]
	//return g.GetGatewayEndpoint(), nil
	return gateways, nil
}

//func (lb sS3LoadBalancer) GetGatewayEndpoint() string {
//	if lb.SslCertificate.ForceHttps == false {
//		return fmt.Sprintf("http://%s:%d", lb.Vip, lb.Port)
//	} else {
//		return fmt.Sprintf("https://%s:%d", lb.Vip, lb.HttpsPort)
//	}
//}
//
