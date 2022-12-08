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
	api "yunion.io/x/onecloud/pkg/apis/compute"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/multicloud/nas"
	"yunion.io/x/onecloud/pkg/multicloud/nas/provider"
	"yunion.io/x/onecloud/pkg/multicloud/nas/xgfs"
)

type SXgfsProviderFactory struct {
	provider.SNasProviderFactory
}

func (self *SXgfsProviderFactory) GetId() string {
	return api.CLOUD_PROVIDER_XGFS
}

func (self *SXgfsProviderFactory) GetName() string {
	return api.CLOUD_PROVIDER_XGFS
}

func (self *SXgfsProviderFactory) GetProvider(cfg cloudprovider.ProviderConfig) (cloudprovider.ICloudProvider, error) {
	options := cloudprovider.SXskyExtraOptions{}
	if cfg.Options != nil {
		err := cfg.Options.Unmarshal(&options)
		if err != nil {
			return nil, err
		}
	}
	client, err := xgfs.NewXgfsClient(
		nas.NewNasClientConfig(
			cfg.URL, cfg.Account, cfg.Secret, &options,
		).CloudproviderConfig(cfg),
	)
	if err != nil {
		return nil, err
	}
	return provider.NewNasProvider(self, client), nil
}

func (self *SXgfsProviderFactory) GetClientRC(info cloudprovider.SProviderInfo) (map[string]string, error) {
	options := cloudprovider.SXskyExtraOptions{}
	if info.Options != nil {
		err := info.Options.Unmarshal(&options)
		if err != nil {
			return nil, err
		}
	}
	client, err := xgfs.NewXgfsClient(
		nas.NewNasClientConfig(
			info.Url, info.Account, info.Secret, &options,
		),
	)
	if err != nil {
		return nil, err
	}
	return client.GetClientRC(), nil
}

func init() {
	factory := SXgfsProviderFactory{}
	cloudprovider.RegisterFactory(&factory)
}
