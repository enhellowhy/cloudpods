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

package workflow

import (
	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
	"yunion.io/x/onecloud/pkg/mcclient/options"
)

func init() {
	/**
	 * 创建指定类型的流程定义
	 */
	type ProcessDefinitionCreateOptions struct {
		Type string `help:"definition type" required:"true" choices:"apply-machine|apply-server-changeconfig|apply-bucket|apply-loadbalancer"`
	}
	R(&ProcessDefinitionCreateOptions{}, "workflow-process-definition-create", "Create process definition",
		func(s *mcclient.ClientSession, args *ProcessDefinitionCreateOptions) error {
			//params := jsonutils.Marshal(args)
			params := jsonutils.NewDict()
			if len(args.Type) > 0 {
				params.Add(jsonutils.NewString(args.Type), "type")
				params.Add(jsonutils.NewString(args.Type), "key")
			}
			params.Add(jsonutils.NewString(modules.WorkflowProcessDefinitionsMap[args.Type]), "name")
			ret, err := modules.WorkflowProcessDefinitions.Create(s, params)
			if err != nil {
				return err
			}
			printObject(ret)
			return nil
		})
	/**
	 * 删除指定ID的流程定义
	 */
	type ProcessDefinitionOptions struct {
		ID string `help:"ID of process definition which belongs to the given process definition id"`
		//ID      string `help:"ID of process definition, deletes the process definition which belongs to the given process definition id"`
		//Cascade bool   `help:"Whether to cascade delete process instance, if set to true, all process instances (including) history are deleted" required:"true" choices:"true|false"`
	}
	R(&ProcessDefinitionOptions{}, "workflow-process-definition-delete", "Delete process definition by ID", func(s *mcclient.ClientSession, args *ProcessDefinitionOptions) error {
		result, e := modules.WorkflowProcessDefinitions.Delete(s, args.ID, nil)
		if e != nil {
			return e
		}
		printObject(result)
		return nil
	})

	/**
	 * 修改指定的流程定义的激活/挂起状态
	 */
	R(&ProcessDefinitionOptions{}, "workflow-process-definition-enable", "Enable an workflow process definition", func(s *mcclient.ClientSession, args *ProcessDefinitionOptions) error {
		r, err := modules.WorkflowProcessDefinitions.PerformAction(s, args.ID, "enable", nil)
		if err != nil {
			return err
		}
		printObject(r)
		return nil
	})

	R(&ProcessDefinitionOptions{}, "workflow-process-definition-disable", "Disable an workflow process definition", func(s *mcclient.ClientSession, args *ProcessDefinitionOptions) error {
		r, err := modules.WorkflowProcessDefinitions.PerformAction(s, args.ID, "disable", nil)
		if err != nil {
			return err
		}
		printObject(r)
		return nil
	})

	/**
	 * 列出流程定义
	 */
	type ProcessDefinitionListOptions struct {
		ProcessDefinitionKey string `help:"Key of process definition"`
		UserId               string `help:"ID of user who start the process definition"`
		options.BaseListOptions
	}
	R(&ProcessDefinitionListOptions{}, "workflow-process-definition-list", "List process definition", func(s *mcclient.ClientSession, args *ProcessDefinitionListOptions) error {
		params, err := options.ListStructToParams(args)
		if err != nil {
			return err
		}
		if len(args.ProcessDefinitionKey) > 0 {
			params.Add(jsonutils.NewString(args.ProcessDefinitionKey), "workflow-process_definition_key")
		}
		if len(args.UserId) > 0 {
			params.Add(jsonutils.NewString(args.UserId), "user_id")
		}
		result, err := modules.WorkflowProcessDefinitions.List(s, params)
		if err != nil {
			return err
		}
		printList(result, modules.WorkflowProcessDefinitions.GetColumns(s))
		return nil
	})

	/**
	 * 查看指定ID的流程定义
	 */
	type ProcessDefinitionShowOptions struct {
		ID string `help:"ID of the process definition"`
	}
	R(&ProcessDefinitionShowOptions{}, "workflow-process-definition-show", "Show process definition", func(s *mcclient.ClientSession, args *ProcessDefinitionShowOptions) error {
		result, err := modules.WorkflowProcessDefinitions.Get(s, args.ID, nil)
		if err != nil {
			return err
		}
		printObject(result)
		return nil
	})
}
