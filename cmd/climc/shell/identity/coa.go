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

package identity

import (
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modules/thirdparty"
)

func init() {
	type CoaUserGetOptions struct {
		ID string `json:"-"`
	}
	R(&CoaUserGetOptions{}, "coa-user-show", "Show coa user", func(s *mcclient.ClientSession, opts *CoaUserGetOptions) error {
		user, err := thirdparty.CoaUsers.Get(s, opts.ID, nil)
		if err != nil {
			return err
		}
		printObject(user)
		return nil
	})
	R(&CoaUserGetOptions{}, "coa-department-show", "Show coa department info", func(s *mcclient.ClientSession, opts *CoaUserGetOptions) error {
		dep, err := thirdparty.CoaUsers.GetDepartment(s, opts.ID, nil)
		if err != nil {
			return err
		}
		printObject(dep)
		return nil
	})
	type CoaSendMarkdownMessageOptions struct {
		Platform     int    `json:"platform"`
		FeishuUserId string `json:"feishu_user_id"`
		Content      string `json:"content"`
	}
	R(&CoaSendMarkdownMessageOptions{}, "coa-message-send", "Send coa markdown message", func(s *mcclient.ClientSession, opts *CoaSendMarkdownMessageOptions) error {
		//jsonutils.Marshal(opts).(*jsonutils.JSONDict), nil
		//contentTemplate := "**IT运维通知**\n您申请的虚拟机资源开通成功\n虚拟机名称: %s\n虚拟机IP地址为: %s\n___\n请通过[堡垒机](https://jumpserver.it.lixiangoa.com/)登录服务器\n[堡垒机使用说明文档](https://li.feishu.cn/wiki/wikcnNGQ59902Pfaku6dPYTlOHf)\n如果您使用中遇到任何问题\n可以通过【IT机器人】进行反馈"
		//content := fmt.Sprintf(contentTemplate, "test", "0.0.0.1")
		//kwargs := jsonutils.NewDict()
		//kwargs.Add(jsonutils.NewString(content), "content")
		//kwargs.Add(jsonutils.NewInt(2), "platform")
		//kwargs.Add(jsonutils.NewString("6976628827972109861"), "feishu_user_id")
		//dep, err := modules.CoaUsers.SendMarkdownMessage(s, kwargs)
		dep, err := thirdparty.CoaUsers.SendMarkdownMessage(s, jsonutils.Marshal(opts).(*jsonutils.JSONDict))
		if err != nil {
			return err
		}
		printObject(dep)
		return nil
	})

	//type ServiceCertificateListOptions struct {
	//	options.BaseListOptions
	//}
	//R(&ServiceCertificateListOptions{}, "service-cert-list", "List service certs", func(s *mcclient.ClientSession, opts *ServiceCertificateListOptions) error {
	//	params, err := options.ListStructToParams(opts)
	//	if err != nil {
	//		return err
	//	}
	//	result, err := modules.ServiceCertificatesV3.List(s, params)
	//	if err != nil {
	//		return err
	//	}
	//	printList(result, modules.ServiceCertificatesV3.GetColumns(s))
	//	return nil
	//})
	//type ServiceCertificateDeleteOptions struct {
	//	ID string `json:"-"`
	//}
	//R(&ServiceCertificateDeleteOptions{}, "service-cert-delete", "Delete service cert", func(s *mcclient.ClientSession, opts *ServiceCertificateDeleteOptions) error {
	//	cert, err := modules.ServiceCertificatesV3.Delete(s, opts.ID, nil)
	//	if err != nil {
	//		return err
	//	}
	//	printObject(cert)
	//	return nil
	//})
}
