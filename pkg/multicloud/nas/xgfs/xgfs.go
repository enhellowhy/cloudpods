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
	"strings"
	"time"
	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/multicloud/nas"
	"yunion.io/x/onecloud/pkg/util/httputils"
	"yunion.io/x/pkg/errors"
	"yunion.io/x/pkg/utils"
)

type SXgfsClient struct {
	*nas.SNasClient

	adminApi *SXgfsAdminApi
}

func parseAccount(account string) (user string, accessKey string) {
	accountInfo := strings.Split(account, "/")
	user = accountInfo[0]
	if len(accountInfo) > 1 {
		accessKey = strings.Join(accountInfo[1:], "/")
	}
	return
}

func NewXgfsClient(cfg *nas.NasClientConfig) (*SXgfsClient, error) {
	//usrname, accessKey := parseAccount(cfg.GetAccessKey())
	usrname, _ := parseAccount(cfg.GetAccessKey())
	adminApi := newXgfsAdminApi(
		usrname,
		cfg.GetAccessSecret(),
		cfg.GetEndpoint(),
		cfg.GetAccessToken(),
		cfg.GetDebug(),
	)
	httputils.SetClientProxyFunc(adminApi.httpClient(), cfg.GetCloudproviderConfig().ProxyFunc)

	//gwEp, err := adminApi.getS3GatewayEndpoint(context.Background())
	//if err != nil {
	//	return nil, errors.Wrap(err, "adminApi.getS3GatewayIP")
	//}

	cli, err := nas.NewNasClient(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "NewObjectStoreClient")
	}

	client := SXgfsClient{
		SNasClient: cli,
		adminApi:   adminApi,
	}

	client.SetVirtualObject(&client)

	return &client, nil
}

func (cli *SXgfsClient) GetSubAccounts() ([]cloudprovider.SSubAccount, error) {
	subAccount := cloudprovider.SSubAccount{
		Account:      cli.adminApi.username,
		Name:         cli.GetCloudproviderConfig().Name,
		HealthStatus: api.CLOUD_PROVIDER_HEALTH_NORMAL,
	}
	return []cloudprovider.SSubAccount{subAccount}, nil
}

func (cli *SXgfsClient) GetSubAccountById(id int) (cloudprovider.SSubAccount, error) {
	subAccount := cloudprovider.SSubAccount{
		Account:      cli.adminApi.username,
		Name:         cli.GetCloudproviderConfig().Name,
		HealthStatus: api.CLOUD_PROVIDER_HEALTH_NORMAL,
	}
	return subAccount, nil
}

func (cli *SXgfsClient) GetAccountId() string {
	// AccountId不能重复，Account可以重复，所以使用token保证唯一性
	//return cli.adminApi.username
	return cli.adminApi.xmsToken
}

func (cli *SXgfsClient) GetVersion() string {
	return ""
}

func (cli *SXgfsClient) About() jsonutils.JSONObject {
	about := jsonutils.NewDict()
	about.Add(jsonutils.Marshal(cli.adminApi.username), "admin_user")
	return about
}

func (cli *SXgfsClient) GetProvider() string {
	return api.CLOUD_PROVIDER_XGFS
}

////////////////////////////////// FileSystems API ///////////////////////////////////
func (cli *SXgfsClient) GetICloudFiles(path, prefix, start string, limit int) ([]cloudprovider.ICloudFileSystem, bool, error) {
	ctx := context.Background()
	files, eof, err := cli.adminApi.getDfsFiles(ctx, false, false, false, limit, start, path, prefix)
	if err != nil {
		return nil, false, errors.Wrap(err, "api.getDfsFiles GetICloudFiles")
	}
	//fileSystems := make([]*SXgfsFileSystem, len(files))
	iFileSystems := make([]cloudprovider.ICloudFileSystem, len(files))
	for i, file := range files {
		fileSystem := new(nas.SFileSystem)
		fileSystem.FileSystemId = file.Path
		fileSystem.Path = file.Path
		fileSystem.FileSystemName = file.Name
		fileSystem.FileSystemType = "standard"
		fileSystem.Files = file.Files
		fileSystem.Size = file.Size
		fileSystem.StorageType = cli.GetStorageType()
		fileSystem.MountTargetCountLimit = 1
		fileSystem.FileQuota = 0
		fileSystem.Capacity = 0
		if file.Shared && len(file.Shares) > 0 {
			if utils.IsInStringArray("nfs", file.Shares) {
				fileSystem.ProtocolType = "NFS"
			} else {
				fileSystem.ProtocolType = file.Shares[0]
			}
		} else {
			fileSystem.ProtocolType = "-"
		}
		fileSystem.Shared = file.Shared
		fileSystem.ModifyTime = file.Modify
		fileSystem.Path = file.Path

		//fileSystems[i].SFileSystem = fileSystem // nil pointer
		//fileSystems[i].client = cli
		fs := SXgfsFileSystem{
			SFileSystem: fileSystem,
			client:      cli,
		}
		//fileSystems[i] = &fs
		iFileSystems[i] = &fs
	}

	return iFileSystems, eof, nil
}

func (cli *SXgfsClient) GetICloudFileSystems() ([]cloudprovider.ICloudFileSystem, error) {
	return cli.getIFileSystems()
}

func (self *SXgfsClient) getIFileSystems() ([]cloudprovider.ICloudFileSystem, error) {
	fileSystems, err := self.GetFileSystems()
	if err != nil {
		return nil, errors.Wrap(err, "GetFileSystems")
	}
	iFileSystems := make([]cloudprovider.ICloudFileSystem, len(fileSystems))
	for i := range fileSystems {
		iFileSystems[i] = fileSystems[i]
	}
	return iFileSystems, nil
}

func (cli *SXgfsClient) GetFileSystems() ([]*SXgfsFileSystem, error) {
	ctx := context.Background()
	files, err := cli.adminApi.getRootDfsFiles(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "api.getRootDfsFiles ListFileSystems")
	}
	fileSystems := make([]*SXgfsFileSystem, len(files))
	for i, file := range files {
		fileSystem := new(nas.SFileSystem)
		fileSystem.FileSystemId = file.Path
		fileSystem.Path = file.Path
		fileSystem.FileSystemName = file.Name
		fileSystem.FileSystemType = "standard"
		fileSystem.Files = file.Files
		fileSystem.Size = file.Size
		fileSystem.StorageType = cli.GetStorageType()
		fileSystem.MountTargetCountLimit = 1
		fileSystem.FileQuota = 0
		fileSystem.Capacity = 0
		if file.Shared && len(file.Shares) > 0 {
			if utils.IsInStringArray("nfs", file.Shares) {
				fileSystem.ProtocolType = "NFS"
			} else {
				fileSystem.ProtocolType = file.Shares[0]
			}
		} else {
			fileSystem.ProtocolType = "NFS"
		}

		//fileSystems[i].SFileSystem = fileSystem // nil pointer
		//fileSystems[i].client = cli
		fs := SXgfsFileSystem{
			SFileSystem: fileSystem,
			client:      cli,
		}
		fileSystems[i] = &fs
	}

	return fileSystems, nil
}

func (cli *SXgfsClient) GetFileSystem(id string) (*SXgfsFileSystem, error) {
	ctx := context.Background()
	file, err := cli.adminApi.getRootDfsFile(ctx, id)
	if err != nil {
		if errors.Cause(err) == cloudprovider.ErrNotFound {
			return nil, errors.Wrapf(cloudprovider.ErrNotFound, id)
		}
		return nil, errors.Wrap(err, "api.getRootDfsFile GetFileSystem")
	}
	fileSystem := new(nas.SFileSystem)
	fileSystem.FileSystemId = file.Path
	fileSystem.Path = file.Path
	fileSystem.FileSystemName = file.Name
	fileSystem.FileSystemType = "standard"
	fileSystem.Files = file.Files
	fileSystem.Size = file.Size
	fileSystem.StorageType = cli.GetStorageType()
	fileSystem.MountTargetCountLimit = 1
	if file.Shared && len(file.Shares) > 0 {
		if utils.IsInStringArray("nfs", file.Shares) {
			fileSystem.ProtocolType = "NFS"
		} else {
			fileSystem.ProtocolType = file.Shares[0]
		}
	} else {
		fileSystem.ProtocolType = "NFS"
	}

	if file.QuotaNum == 0 {
		fileSystem.FileQuota = 0
		fileSystem.Capacity = 0
	} else {
		qs, err := cli.adminApi.getDfsQuotas(ctx, file.DfsPathId)
		if err != nil {
			log.Errorf("client.getDfsQuotas err %s", err)
			return nil, errors.Wrap(err, "api.getDfsQuotas GetFileSystem")
		}
		if len(qs) == 0 {
			log.Infof("DfsQuota %s no equal 1, is %d", file.Path, len(qs))
			fileSystem.FileQuota = 0
			fileSystem.Capacity = 0
		} else if len(qs) > 1 {
			log.Errorf("DfsQuota %s more than 1, is %d", file.Path, len(qs))
			return nil, errors.Errorf("getDfsQuotas dfsQuota more than 1, is %d", len(qs))
		} else {
			fileSystem.FileQuota = qs[0].FilesHardQuota
			fileSystem.Capacity = qs[0].SizeHardQuota
		}
		//if qs[0].Status == "active" {
		//	return nil, nil
		//}
	}

	//xgfsFileSystem := new(SXgfsFileSystem)
	//xgfsFileSystem.SFileSystem = fileSystem
	//xgfsFileSystem.client = cli
	xgfsFileSystem := SXgfsFileSystem{
		SFileSystem: fileSystem,
		client:      cli,
	}
	return &xgfsFileSystem, nil
}

func (cli *SXgfsClient) DeleteFileSystem(id string) error {
	ctx := context.Background()
	_, err := cli.adminApi.getRootDfsFile(ctx, id)
	if err != nil {
		if errors.Cause(err) == cloudprovider.ErrNotFound {
			return nil
		}
		return errors.Wrap(err, "api.getRootDfsFile DeleteFileSystem")
	}
	_, err = cli.adminApi.deleteDfsFile(ctx, id)
	if err != nil {
		return errors.Wrap(err, "api.deleteDfsFile DeleteFileSystem")
	}

	return nil
}

func (cli *SXgfsClient) GetICloudFileSystemById(id string) (cloudprovider.ICloudFileSystem, error) {
	fileSystem, err := cli.GetFileSystem(id)
	if err != nil {
		if errors.Cause(err) == cloudprovider.ErrNotFound {
			return nil, errors.Wrapf(cloudprovider.ErrNotFound, id)
		}
		return nil, errors.Wrap(err, "client.GetFileSystem")
	}
	return fileSystem, nil
}

func (cli *SXgfsClient) CreateICloudFileSystem(opts *cloudprovider.FileSystemCraeteOptions) (cloudprovider.ICloudFileSystem, error) {
	ctx := context.Background()
	path := "/" + opts.Name
	_, err := cli.adminApi.createDfsFile(ctx, path)
	if err != nil {
		return nil, errors.Wrap(err, "client.createDfsFile")
	}

	fileSystem, err := cli.GetFileSystem(path)
	if err != nil {
		return nil, errors.Wrap(err, "client.GetFileSystem")
	}
	// quota
	if opts.Capacity > 0 {
		quota, err := cli.adminApi.createDfsQuota(ctx, path, opts.Capacity)
		if err != nil {
			log.Errorf("client.createDfsQuota err %s", err)
			return fileSystem, errors.Errorf("SetQuotaError createDfsQuota")
		}
		if quota.Status != "active" {
			for i := 0; i < 3; i++ {
				time.Sleep(5 * time.Second)
				qs, err := cli.adminApi.getDfsQuotas(ctx, quota.DfsPath.Id)
				if err != nil {
					log.Errorf("client.createDfsQuota err %s", err)
					return fileSystem, errors.Errorf("SetQuotaError createDfsQuota")
				}
				if len(qs) != 1 {
					log.Errorf("DfsQuota %s no equal 1, is %d", quota.DfsPath.Name, len(qs))
					return fileSystem, errors.Errorf("SetQuotaError dfsQuota no equal 1, is %d", len(qs))
				}
				if qs[0].Status == "active" {
					return fileSystem, nil
				}
			}
		}
	}

	return fileSystem, nil
}

//func (self *SXgfsClient) invalidateIFileSystems() {
//	self.iFileSystems = nil
//}

func (cli *SXgfsClient) GetDfsShares(fs *SXgfsFileSystem) ([]*SDfsNfsShare, error) {
	ctx := context.Background()
	shares, err := cli.adminApi.getDfsNfsShares(ctx, fs.FileSystemId)
	if err != nil {
		return nil, errors.Wrap(err, "api.getDfsNfsShares GetDfsShares")
	}
	dns := make([]*SDfsNfsShare, len(shares))
	for i, share := range shares {
		mountTarget := &nas.SMountTarget{
			FileSystemId: fs.FileSystemId,
			NetworkType:  "classic",
			Status:       share.Status,
			Id:           share.Id,
			Name:         share.Name,
			VpcId:        share.Cluster.FsId,
		}
		dfsNfsShare := SDfsNfsShare{
			SMountTarget: mountTarget,
			fs:           fs,
			action:       share.ActionStatus,
		}
		dns[i] = &dfsNfsShare
	}
	return dns, nil
}

func (cli *SXgfsClient) GetICloudNasSkus() ([]cloudprovider.ICloudNasSku, error) {
	ctx := context.Background()
	cluster, err := cli.adminApi.getCluster(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "api.getCluster GetICloudNasSkus")
	}
	sku := &nas.SNasSku{
		Name:        cluster.Name,
		Id:          cluster.FsId,
		StorageType: cli.GetStorageType(),
		Protocol:    "NFS",
		Provider:    cli.GetProvider(),
		SyncOnly:    cli.GetSyncOnly(),
	}
	return []cloudprovider.ICloudNasSku{sku}, nil
}

func (cli *SXgfsClient) GetICloudNasSkusByProviderId(id string) ([]cloudprovider.ICloudNasSku, error) {
	ctx := context.Background()
	cluster, err := cli.adminApi.getCluster(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "api.getCluster GetICloudNasSkus")
	}
	sku := &nas.SNasSku{
		Name:        cluster.Name,
		Id:          id,
		StorageType: cli.GetStorageType(),
		Protocol:    "NFS",
		Provider:    cli.GetProvider(),
		SyncOnly:    cli.GetSyncOnly(),
	}
	return []cloudprovider.ICloudNasSku{sku}, nil
}

func (cli *SXgfsClient) GetDfsShareAcls(shareId int) ([]*SDfsNfsShareAcl, error) {
	ctx := context.Background()
	acls, err := cli.adminApi.getDfsNfsShareAcls(ctx, shareId)
	if err != nil {
		return nil, errors.Wrap(err, "api.getDfsNfsShareAcls GetDfsShareAcls")
	}
	result := make([]*SDfsNfsShareAcl, 0)
	for _, acl := range acls {
		if acl.Type == "client_group" {
			continue
		}
		a := SDfsNfsShareAcl{
			Id:     acl.Id,
			Name:   acl.FsClient.Name,
			Sync:   acl.Sync,
			Status: api.NAS_STATUS_AVAILABLE,
		}
		// source
		c1, e1 := cli.adminApi.getFsClientById(ctx, acl.FsClient.Id)
		if e1 != nil {
			c2, e2 := cli.adminApi.getFsClientById(ctx, acl.FsClient.Id)
			if e2 != nil {
				log.Errorf("getFsClientById %d err %v", acl.FsClient.Id, e2)
				continue
				//a.Source = "0.0.0.0/0"
			} else {
				a.Source = c2.Ip
			}
		} else {
			log.Debugf("getFsClient ip %s via c1", c1.Ip)
			a.Source = c1.Ip
		}
		// AllSquash
		if acl.AllSquash {
			a.UserAccessType = "all_squash"
		} else {
			a.UserAccessType = "no_all_squash"
		}
		// RootSquash
		if acl.RootSquash {
			a.RootUserAccessType = "root_squash"
		} else {
			a.RootUserAccessType = "no_root_squash"
		}
		// RWAccessType
		if acl.Permission == "RW" {
			a.RWAccessType = "RW"
		} else {
			a.RWAccessType = "R"
		}
		//result[i] = &a
		result = append(result, &a)
	}
	return result, nil
}

func (cli *SXgfsClient) GetDfsShareAclsById(shareId, id int) ([]*SDfsNfsShareAcl, error) {
	ctx := context.Background()
	acls, err := cli.adminApi.getDfsNfsShareAclsById(ctx, shareId, id)
	if err != nil {
		return nil, errors.Wrap(err, "api.getDfsNfsShareAclsById GetDfsShareAclsById")
	}
	result := make([]*SDfsNfsShareAcl, 0)
	for _, acl := range acls {
		if acl.Type == "client_group" {
			continue
		}
		a := SDfsNfsShareAcl{
			Id:     acl.Id,
			Name:   acl.FsClient.Name,
			Sync:   acl.Sync,
			Status: api.NAS_STATUS_AVAILABLE,
		}
		// source
		c1, e1 := cli.adminApi.getFsClientById(ctx, acl.FsClient.Id)
		if e1 != nil {
			c2, e2 := cli.adminApi.getFsClientById(ctx, acl.FsClient.Id)
			if e2 != nil {
				log.Errorf("getFsClientById %d err %v", acl.FsClient.Id, e2)
				continue
				//a.Source = "0.0.0.0/0"
			} else {
				a.Source = c2.Ip
			}
		} else {
			log.Debugf("getFsClient ip %s via c1", c1.Ip)
			a.Source = c1.Ip
		}
		// AllSquash
		if acl.AllSquash {
			a.UserAccessType = "all_squash"
		} else {
			a.UserAccessType = "no_all_squash"
		}
		// RootSquash
		if acl.RootSquash {
			a.RootUserAccessType = "root_squash"
		} else {
			a.RootUserAccessType = "no_root_squash"
		}
		// RWAccessType
		if acl.Permission == "RW" {
			a.RWAccessType = "RW"
		} else {
			a.RWAccessType = "R"
		}
		//result[i] = &a
		result = append(result, &a)
	}
	return result, nil
}

func (cli *SXgfsClient) GetFileSystemClientByIP(ctx context.Context, source string) ([]sFsClient, error) {
	fsClients, err := cli.adminApi.getFsClientsByIP(ctx, source)
	if err != nil {
		return nil, errors.Wrap(err, "client.getFsClients")
	}
	//for _, client := range fsClients {
	//	if client.Ip == source {
	//		return client, nil
	//	}
	//}
	//return sFsClient{}, fmt.Errorf("can not found fs client for %s", source)
	return fsClients, nil
}

func (cli *SXgfsClient) CreateFileSystemClient(ctx context.Context, name, source string) (sFsClient, error) {
	fsClient, err := cli.adminApi.createFsClient(ctx, name, source)
	if err != nil {
		return sFsClient{}, errors.Wrap(err, "client.createFsClient")
	}
	return fsClient, nil
}

func (cli *SXgfsClient) CreateDfsNfsShare(opts *cloudprovider.SNfsShareCreateOptions, fs *SXgfsFileSystem) (cloudprovider.ICloudMountTarget, error) {
	ctx := context.Background()
	s, err := cli.adminApi.createDfsNfsShare(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "client.createDfsNfsShare")
	}
	mountTarget := &nas.SMountTarget{
		FileSystemId: fs.FileSystemId,
		NetworkType:  "classic",
		Status:       s.Status,
		Id:           s.Id,
		Name:         s.Name,
		VpcId:        s.Cluster.FsId,
	}
	dfsNfsShare := SDfsNfsShare{
		SMountTarget: mountTarget,
		fs:           fs,
		action:       s.ActionStatus,
	}
	return &dfsNfsShare, nil
}

func (cli *SXgfsClient) DeleteDfsNfsShare(shareId int, fs *SXgfsFileSystem) (cloudprovider.ICloudMountTarget, error) {
	ctx := context.Background()
	s, err := cli.adminApi.deleteDfsNfsShare(ctx, shareId)
	if err != nil {
		return nil, errors.Wrap(err, "client.deleteDfsNfsShare")
	}
	mountTarget := &nas.SMountTarget{
		FileSystemId: fs.FileSystemId,
		NetworkType:  "classic",
		Status:       s.Status,
		Id:           s.Id,
		Name:         s.Name,
		VpcId:        s.Cluster.FsId,
	}
	dfsNfsShare := SDfsNfsShare{
		SMountTarget: mountTarget,
		fs:           fs,
		action:       s.ActionStatus,
	}
	return &dfsNfsShare, nil
}

func (cli *SXgfsClient) AddDfsNfsShareAcl(opts *cloudprovider.SNfsShareAclAddOptions, fs *SXgfsFileSystem, shareId int) (cloudprovider.ICloudMountTarget, error) {
	ctx := context.Background()
	s, err := cli.adminApi.addDfsNfsShareAcl(ctx, opts, shareId)
	if err != nil {
		return nil, errors.Wrap(err, "client.addDfsNfsShareAcl")
	}
	mountTarget := &nas.SMountTarget{
		FileSystemId: fs.FileSystemId,
		NetworkType:  "classic",
		Status:       s.Status,
		Id:           s.Id,
		Name:         s.Name,
		VpcId:        s.Cluster.FsId,
	}
	dfsNfsShare := SDfsNfsShare{
		SMountTarget: mountTarget,
		fs:           fs,
		action:       s.ActionStatus,
	}
	return &dfsNfsShare, nil
}

func (cli *SXgfsClient) UpdateDfsNfsShareAcl(opts *cloudprovider.SNfsShareAclUpdateOptions, fs *SXgfsFileSystem, shareId int) (cloudprovider.ICloudMountTarget, error) {
	ctx := context.Background()
	s, err := cli.adminApi.updateDfsNfsShareAcl(ctx, opts, shareId)
	if err != nil {
		return nil, errors.Wrap(err, "client.updateDfsNfsShareAcl")
	}
	mountTarget := &nas.SMountTarget{
		FileSystemId: fs.FileSystemId,
		NetworkType:  "classic",
		Status:       s.Status,
		Id:           s.Id,
		Name:         s.Name,
		VpcId:        s.Cluster.FsId,
	}
	dfsNfsShare := SDfsNfsShare{
		SMountTarget: mountTarget,
		fs:           fs,
		action:       s.ActionStatus,
	}
	return &dfsNfsShare, nil
}

func (cli *SXgfsClient) RemoveDfsNfsShareAcl(opts *cloudprovider.SNfsShareAclRemoveOptions, fs *SXgfsFileSystem, shareId int) (cloudprovider.ICloudMountTarget, error) {
	ctx := context.Background()
	s, err := cli.adminApi.removeDfsNfsShareAcl(ctx, opts, shareId)
	if err != nil {
		return nil, errors.Wrap(err, "client.removeDfsNfsShareAcl")
	}
	mountTarget := &nas.SMountTarget{
		FileSystemId: fs.FileSystemId,
		NetworkType:  "classic",
		Status:       s.Status,
		Id:           s.Id,
		Name:         s.Name,
		VpcId:        s.Cluster.FsId,
	}
	dfsNfsShare := SDfsNfsShare{
		SMountTarget: mountTarget,
		fs:           fs,
		action:       s.ActionStatus,
	}
	return &dfsNfsShare, nil
}
