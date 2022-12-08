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

package nas

import (
	"yunion.io/x/jsonutils"
	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudcommon/object"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/multicloud"
	"yunion.io/x/pkg/util/secrules"
)

type NasClientConfig struct {
	cpcfg   cloudprovider.ProviderConfig
	options *cloudprovider.SXskyExtraOptions

	endpoint     string
	accessKey    string
	accessSecret string
	//accessToken  string

	debug bool
}

func NewNasClientConfig(endpoint, accessKey, accessSecret string, options *cloudprovider.SXskyExtraOptions) *NasClientConfig {
	cfg := &NasClientConfig{
		endpoint:     endpoint,
		accessKey:    accessKey,
		accessSecret: accessSecret,
		options:      options,
	}
	return cfg
}

func (cfg *NasClientConfig) CloudproviderConfig(cpcfg cloudprovider.ProviderConfig) *NasClientConfig {
	cfg.cpcfg = cpcfg
	return cfg
}

func (cfg *NasClientConfig) Debug(debug bool) *NasClientConfig {
	cfg.debug = debug
	return cfg
}

func (cfg *NasClientConfig) GetCloudproviderConfig() cloudprovider.ProviderConfig {
	return cfg.cpcfg
}

func (cfg *NasClientConfig) GetEndpoint() string {
	return cfg.endpoint
}

func (cfg *NasClientConfig) GetAccessKey() string {
	return cfg.accessKey
}

func (cfg *NasClientConfig) GetAccessToken() string {
	return cfg.options.XmsAuthToken
}

func (cfg *NasClientConfig) GetStorageType() string {
	return cfg.options.StorageType
}

func (cfg *NasClientConfig) GetAccessSecret() string {
	return cfg.accessSecret
}

func (cfg *NasClientConfig) GetDebug() bool {
	return cfg.debug
}

func (cfg *NasClientConfig) GetSyncOnly() bool {
	return cfg.options.SyncOnly
}

type SNasClient struct {
	object.SObject

	*NasClientConfig

	cloudprovider.SFakeOnPremiseRegion
	multicloud.SRegion

	ownerId   string
	ownerName string

	iFileSystems []cloudprovider.ICloudFileSystem
}

func NewNasClient(cfg *NasClientConfig) (*SNasClient, error) {
	client := SNasClient{
		NasClientConfig: cfg,
	}
	//client.SetVirtualObject(&client)

	return &client, nil
}

func (cli *SNasClient) GetSubAccounts() ([]cloudprovider.SSubAccount, error) {
	subAccount := cloudprovider.SSubAccount{
		Account:      cli.accessKey,
		Name:         cli.cpcfg.Name,
		HealthStatus: api.CLOUD_PROVIDER_HEALTH_NORMAL,
	}
	return []cloudprovider.SSubAccount{subAccount}, nil
}

func (cli *SNasClient) GetSubAccountById(id int) (cloudprovider.SSubAccount, error) {
	subAccount := cloudprovider.SSubAccount{
		Account:      cli.accessKey,
		Name:         cli.cpcfg.Name,
		HealthStatus: api.CLOUD_PROVIDER_HEALTH_NORMAL,
	}
	return subAccount, nil
}

func (cli *SNasClient) GetAccountId() string {
	return cli.ownerId
}

func (cli *SNasClient) GetIRegion() cloudprovider.ICloudRegion {
	return cli.GetVirtualObject().(cloudprovider.ICloudRegion)
}

func (cli *SNasClient) GetINasProvider() INasProvider {
	return cli.GetVirtualObject().(INasProvider)
}

func (cli *SNasClient) GetI18n() cloudprovider.SModelI18nTable {
	table := cloudprovider.SModelI18nTable{}
	table["name"] = cloudprovider.NewSModelI18nEntry(cli.GetName()).CN(cli.GetName())
	return table
}

func (cli *SNasClient) GetVersion() string {
	return ""
}

func (cli *SNasClient) About() jsonutils.JSONObject {
	about := jsonutils.NewDict()
	return about
}

func (cli *SNasClient) GetProvider() string {
	return api.CLOUD_PROVIDER_NAS
}

func (cli *SNasClient) GetCloudEnv() string {
	return ""
}

////////////////////////////// INasProvider //////////////////////////////
func (cli *SNasClient) GetEndpoint() string {
	return cli.endpoint
}

func (cli *SNasClient) GetClientRC() map[string]string {
	return map[string]string{
		"NAS_ACCESS_KEY": cli.accessKey,
		"NAS_SECRET":     cli.accessSecret,
		"NAS_ACCESS_URL": cli.endpoint,
		"NAS_REGION":     api.CLOUD_PROVIDER_NAS,
	}
}

///////////////////////////////// fake impletementations //////////////////////

func (cli *SNasClient) GetIZones() ([]cloudprovider.ICloudZone, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetIVpcs() ([]cloudprovider.ICloudVpc, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetIEips() ([]cloudprovider.ICloudEIP, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetIVpcById(id string) (cloudprovider.ICloudVpc, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetIZoneById(id string) (cloudprovider.ICloudZone, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetIEipById(id string) (cloudprovider.ICloudEIP, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetIVMById(id string) (cloudprovider.ICloudVM, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetIDiskById(id string) (cloudprovider.ICloudDisk, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) SyncSecurityGroup(secgroupId string, vpcId string, name string, desc string, rules []secrules.SecurityRule) (string, error) {
	return "", cloudprovider.ErrNotSupported
}

func (cli *SNasClient) CreateIVpc(opts *cloudprovider.VpcCreateOptions) (cloudprovider.ICloudVpc, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) CreateEIP(eip *cloudprovider.SEip) (cloudprovider.ICloudEIP, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetISnapshots() ([]cloudprovider.ICloudSnapshot, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetISnapshotById(snapshotId string) (cloudprovider.ICloudSnapshot, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) CreateSnapshotPolicy(*cloudprovider.SnapshotPolicyInput) (string, error) {
	return "", cloudprovider.ErrNotSupported
}

func (cli *SNasClient) DeleteSnapshotPolicy(string) error {
	return cloudprovider.ErrNotSupported
}

func (cli *SNasClient) ApplySnapshotPolicyToDisks(snapshotPolicyId string, diskId string) error {
	return cloudprovider.ErrNotSupported
}

func (cli *SNasClient) CancelSnapshotPolicyToDisks(snapshotPolicyId string, diskId string) error {
	return cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetISnapshotPolicies() ([]cloudprovider.ICloudSnapshotPolicy, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetISnapshotPolicyById(snapshotPolicyId string) (cloudprovider.ICloudSnapshotPolicy, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetIHosts() ([]cloudprovider.ICloudHost, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetIHostById(id string) (cloudprovider.ICloudHost, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetIStorages() ([]cloudprovider.ICloudStorage, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetIStorageById(id string) (cloudprovider.ICloudStorage, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetIStoragecaches() ([]cloudprovider.ICloudStoragecache, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetIStoragecacheById(id string) (cloudprovider.ICloudStoragecache, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetILoadBalancers() ([]cloudprovider.ICloudLoadbalancer, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetILoadBalancerAcls() ([]cloudprovider.ICloudLoadbalancerAcl, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetILoadBalancerCertificates() ([]cloudprovider.ICloudLoadbalancerCertificate, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetILoadBalancerById(loadbalancerId string) (cloudprovider.ICloudLoadbalancer, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetILoadBalancerAclById(aclId string) (cloudprovider.ICloudLoadbalancerAcl, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetILoadBalancerCertificateById(certId string) (cloudprovider.ICloudLoadbalancerCertificate, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) CreateILoadBalancer(loadbalancer *cloudprovider.SLoadbalancer) (cloudprovider.ICloudLoadbalancer, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) CreateILoadBalancerAcl(acl *cloudprovider.SLoadbalancerAccessControlList) (cloudprovider.ICloudLoadbalancerAcl, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) CreateILoadBalancerCertificate(cert *cloudprovider.SLoadbalancerCertificate) (cloudprovider.ICloudLoadbalancerCertificate, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetISkus() ([]cloudprovider.ICloudSku, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) CreateISku(opts *cloudprovider.SServerSkuCreateOption) (cloudprovider.ICloudSku, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetIBuckets() ([]cloudprovider.ICloudBucket, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) CreateIBucket(name string, storageClassStr string, acl string) error {
	return cloudprovider.ErrNotSupported
}

func (cli *SNasClient) DeleteIBucket(name string) error {
	return cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetIBucketById(name string) (cloudprovider.ICloudBucket, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) IBucketExist(name string) (bool, error) {
	return false, cloudprovider.ErrNotSupported
}

func (cli *SNasClient) GetIBucketByName(name string) (cloudprovider.ICloudBucket, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (self *SNasClient) GetCapabilities() []string {
	caps := []string{
		// cloudprovider.CLOUD_CAPABILITY_PROJECT,
		// cloudprovider.CLOUD_CAPABILITY_COMPUTE,
		// cloudprovider.CLOUD_CAPABILITY_NETWORK,
		// cloudprovider.CLOUD_CAPABILITY_LOADBALANCER,
		//cloudprovider.CLOUD_CAPABILITY_OBJECTSTORE,
		cloudprovider.CLOUD_CAPABILITY_NAS,
		// cloudprovider.CLOUD_CAPABILITY_RDS,
		// cloudprovider.CLOUD_CAPABILITY_CACHE,
		// cloudprovider.CLOUD_CAPABILITY_EVENT,
	}
	return caps
}
