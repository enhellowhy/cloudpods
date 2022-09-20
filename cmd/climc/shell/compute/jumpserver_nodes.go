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

package compute

import (
	"yunion.io/x/onecloud/cmd/climc/shell"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modules/thirdparty"
	"yunion.io/x/onecloud/pkg/mcclient/options"
)

func init() {
	cmd := shell.NewResourceCmd(&thirdparty.JsAsset)
	cmd.Create(&options.JumpServerAssetCreateOptions{})
	cmd.Delete(&options.BaseIdOptions{})
	cmd.Show(&options.BaseIdOptions{})
	R(&options.JumpServerNodeListOptions{}, "jsnode-list", "List jumpserver nodes", func(s *mcclient.ClientSession, args *options.JumpServerNodeListOptions) error {
		result, err := thirdparty.JsAsset.List(s, nil)
		if err != nil {
			return err
		}
		printList(result, thirdparty.JsAsset.GetColumns(s))
		return nil
	})
	type JumpServerAssetIpShowOptions struct {
		Ip string `help:"ip of the js asset"`
	}
	R(&JumpServerAssetIpShowOptions{}, "jsassets-ipshow", "Show js user details", func(s *mcclient.ClientSession, args *JumpServerAssetIpShowOptions) error {
		result, err := thirdparty.JsAsset.GetByIP(s, args.Ip, nil)
		if err != nil {
			return err
		}
		if result == nil {
			return nil
		}
		printObject(result)
		return nil
	})
	R(&options.JumpServerNodeListOptions{}, "jsuser-list", "List jumpserver users", func(s *mcclient.ClientSession, args *options.JumpServerNodeListOptions) error {
		result, err := thirdparty.JsAsset.ListUser(s, nil)
		if err != nil {
			return err
		}
		columns := []string{"ID", "Email", "Name", "Source"}
		printList(result, columns)
		return nil
	})
	type JumpServerUserShowOptions struct {
		Email string `help:"email of the js user"`
	}
	R(&JumpServerUserShowOptions{}, "jsuser-show", "Show js user details", func(s *mcclient.ClientSession, args *JumpServerUserShowOptions) error {
		result, err := thirdparty.JsAsset.GetUser(s, args.Email, nil)
		if err != nil {
			return err
		}
		if result == nil {
			return nil
		}
		printObject(result)
		return nil
	})
	R(&options.JumpServerUserCreateOptions{}, "jsuser-create", "Create jumpserver user", func(s *mcclient.ClientSession, args *options.JumpServerUserCreateOptions) error {
		params, err := args.Params()
		if err != nil {
			return err
		}
		result, err := thirdparty.JsAsset.CreateUser(s, params)
		if err != nil {
			return err
		}
		printObject(result)
		return nil
	})
	//cmd.Show(&options.RouteTableIdOptions{})
	//cmd.Update(&options.RouteTableUpdateOptions{})
	//cmd.Delete(&options.RouteTableIdOptions{})
	//cmd.Perform("syncstatus", &options.RouteTableIdOptions{})
	//cmd.Perform("add-routes", &options.RouteTableAddRoutesOptions{})
	//cmd.Perform("del-routes", &options.RouteTableDelRoutesOptions{})
	//cmd.Perform("purge", &options.RouteTableIdOptions{})
	R(&options.JumpServerNodeListOptions{}, "jsperms-list", "List jumpserver perms", func(s *mcclient.ClientSession, args *options.JumpServerNodeListOptions) error {
		result, err := thirdparty.JsAsset.ListAssetPerms(s, nil)
		if err != nil {
			return err
		}
		columns := []string{"ID", "Users", "Name", "Assets", "system_users"}
		printList(result, columns)
		return nil
	})
	type JumpServerPermsShowOptions struct {
		Name string `help:"name of the js perm"`
	}
	R(&JumpServerPermsShowOptions{}, "jsperms-show", "Show js perms details", func(s *mcclient.ClientSession, args *JumpServerPermsShowOptions) error {
		result, err := thirdparty.JsAsset.GetPermsByName(s, args.Name, nil)
		if err != nil {
			return err
		}
		if result == nil {
			return nil
		}
		printObject(result)
		return nil
	})
	R(&options.JumpServerAssetPermsCreateOptions{}, "jsperms-create", "Create jumpserver user", func(s *mcclient.ClientSession, args *options.JumpServerAssetPermsCreateOptions) error {
		params, err := args.Params()
		if err != nil {
			return err
		}
		result, err := thirdparty.JsAsset.CreatePerms(s, params)
		if err != nil {
			return err
		}
		printObject(result)
		return nil
	})
	R(&options.JumpServerAssetPermsUpdateOptions{}, "jsperms-patch", "Create jumpserver user", func(s *mcclient.ClientSession, args *options.JumpServerAssetPermsUpdateOptions) error {
		params, err := args.Params()
		if err != nil {
			return err
		}
		result, err := thirdparty.JsAsset.PatchPerms(s, args.Id, params)
		if err != nil {
			return err
		}
		printObject(result)
		return nil
	})
}
