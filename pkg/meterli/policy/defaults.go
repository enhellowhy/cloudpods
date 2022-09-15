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

package policy

import (
	"yunion.io/x/onecloud/pkg/apis"
	common_policy "yunion.io/x/onecloud/pkg/cloudcommon/policy"
	"yunion.io/x/onecloud/pkg/util/rbacutils"
)

const (
	PolicyActionGet     = common_policy.PolicyActionGet
	PolicyActionList    = common_policy.PolicyActionList
	PolicyActionCreate  = common_policy.PolicyActionCreate
	PolicyActionPerform = common_policy.PolicyActionPerform
)

var (
	predefinedDefaultPolicies = []rbacutils.SRbacPolicy{
		//{
		//	Auth:  true,
		//	Scope: rbacutils.ScopeSystem,
		//	Rules: []rbacutils.SRbacRule{
		//		{
		//			Service:  apis.SERVICE_TYPE_METER_LI,
		//			Resource: "rates",
		//			Action:   PolicyActionList,
		//			Result:   rbacutils.Allow,
		//		},
		//		{
		//			Service:  apis.SERVICE_TYPE_WORKFLOW,
		//			Resource: "workflow_process_instances",
		//			Action:   PolicyActionGet,
		//			Result:   rbacutils.Allow,
		//		},
		//		{
		//			Service:  apis.SERVICE_TYPE_WORKFLOW,
		//			Resource: "workflow_process_instances",
		//			Action:   PolicyActionPerform,
		//			Result:   rbacutils.Allow,
		//		},
		//		{
		//			Service:  apis.SERVICE_TYPE_WORKFLOW,
		//			Resource: "workflow_process_instances",
		//			Action:   PolicyActionCreate,
		//			Result:   rbacutils.Allow,
		//		},
		//	},
		//},
		{
			Auth:  true,
			Scope: rbacutils.ScopeSystem,
			Rules: []rbacutils.SRbacRule{
				{
					Service:  apis.SERVICE_TYPE_METER_LI,
					Resource: "rates",
					Action:   PolicyActionList,
					Result:   rbacutils.Allow,
				},
			},
		},
	}
)

func init() {
	common_policy.AppendDefaultPolicies(predefinedDefaultPolicies)
}
