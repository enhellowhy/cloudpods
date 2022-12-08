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

package provider

import (
	"context"
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/multicloud/nas"
	"yunion.io/x/pkg/errors"

	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/mcclient"
)

type SNasProviderFactory struct {
	cloudprovider.SPremiseBaseProviderFactory
}

func (self *SNasProviderFactory) GetId() string {
	return api.CLOUD_PROVIDER_NAS
}

func (self *SNasProviderFactory) GetName() string {
	return api.CLOUD_PROVIDER_NAS
}

func (self *SNasProviderFactory) ValidateCreateCloudaccountData(ctx context.Context, userCred mcclient.TokenCredential, input cloudprovider.SCloudaccountCredential) (cloudprovider.SCloudaccount, error) {
	output := cloudprovider.SCloudaccount{}
	if len(input.AccessKeyId) == 0 {
		return output, errors.Wrap(httperrors.ErrMissingParameter, "access_key_id")
	}
	if len(input.AccessKeySecret) == 0 {
		return output, errors.Wrap(httperrors.ErrMissingParameter, "access_key_secret")
	}
	if len(input.Endpoint) == 0 {
		return output, errors.Wrap(httperrors.ErrMissingParameter, "endpoint")
	}
	output.Account = input.AccessKeyId
	output.Secret = input.AccessKeySecret
	output.AccessUrl = input.Endpoint
	return output, nil
}

func (self *SNasProviderFactory) ValidateUpdateCloudaccountCredential(ctx context.Context, userCred mcclient.TokenCredential, input cloudprovider.SCloudaccountCredential, cloudaccount string) (cloudprovider.SCloudaccount, error) {
	output := cloudprovider.SCloudaccount{}
	if len(input.AccessKeyId) == 0 {
		return output, errors.Wrap(httperrors.ErrMissingParameter, "access_key_id")
	}
	if len(input.AccessKeySecret) == 0 {
		return output, errors.Wrap(httperrors.ErrMissingParameter, "access_key_secret")
	}
	output = cloudprovider.SCloudaccount{
		Account: input.AccessKeyId,
		Secret:  input.AccessKeySecret,
	}
	return output, nil
}

func (self *SNasProviderFactory) GetProvider(cfg cloudprovider.ProviderConfig) (cloudprovider.ICloudProvider, error) {
	options := cloudprovider.SXskyExtraOptions{}
	if cfg.Options != nil {
		err := cfg.Options.Unmarshal(&options)
		if err != nil {
			return nil, err
			//log.Debugf("cfg.Options.Unmarshal %s", err)
		}
	}
	client, err := nas.NewNasClient(
		nas.NewNasClientConfig(
			cfg.URL, cfg.Account, cfg.Secret, &options,
		).CloudproviderConfig(cfg),
	)
	if err != nil {
		return nil, err
	}
	return NewNasProvider(self, client), nil
}

func (f *SNasProviderFactory) GetClientRC(info cloudprovider.SProviderInfo) (map[string]string, error) {
	return map[string]string{
		"NAS_ACCESS_KEY": info.Account,
		"NAS_SECRET":     info.Secret,
		"NAS_ACCESS_URL": info.Url,
		"NAS_REGION":     api.CLOUD_PROVIDER_NAS,
	}, nil
}

func init() {
	factory := SNasProviderFactory{}
	cloudprovider.RegisterFactory(&factory)
}

type SNasProvider struct {
	cloudprovider.SBaseProvider
	client nas.INasProvider
}

func NewNasProvider(factory cloudprovider.ICloudProviderFactory, client nas.INasProvider) *SNasProvider {
	return &SNasProvider{
		SBaseProvider: cloudprovider.NewBaseProvider(factory),
		client:        client,
	}
}

func (self *SNasProvider) GetIRegions() []cloudprovider.ICloudRegion {
	return nil
}

func (self *SNasProvider) GetIRegionById(id string) (cloudprovider.ICloudRegion, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (self *SNasProvider) GetBalance() (float64, string, error) {
	return 0.0, api.CLOUD_PROVIDER_HEALTH_NORMAL, cloudprovider.ErrNotSupported
}

func (self *SNasProvider) GetOnPremiseIRegion() (cloudprovider.ICloudRegion, error) {
	return self.client, nil
}

func (self *SNasProvider) GetIProjects() ([]cloudprovider.ICloudProject, error) {
	return nil, cloudprovider.ErrNotSupported
}

func (self *SNasProvider) GetSysInfo() (jsonutils.JSONObject, error) {
	return self.client.About(), nil
}

func (self *SNasProvider) GetVersion() string {
	return self.client.GetVersion()
}

func (self *SNasProvider) GetSubAccounts() ([]cloudprovider.SSubAccount, error) {
	return self.client.GetSubAccounts()
}

func (self *SNasProvider) GetSubAccountById(id int) (cloudprovider.SSubAccount, error) {
	return self.client.GetSubAccountById(id)
}

func (self *SNasProvider) GetAccountId() string {
	return self.client.GetAccountId()
}

func (self *SNasProvider) GetStorageClasses(regionId string) []string {
	return []string{}
}

func (self *SNasProvider) GetBucketCannedAcls(regionId string) []string {
	return nil
}

func (self *SNasProvider) GetObjectCannedAcls(regionId string) []string {
	return nil
}

func (self *SNasProvider) GetCapabilities() []string {
	return self.client.GetCapabilities()
}
