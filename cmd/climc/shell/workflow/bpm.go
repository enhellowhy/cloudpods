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
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modules/thirdparty"
)

func init() {
	type BpmProcessGetOptions struct {
		ID string `json:"-"`
	}
	R(&BpmProcessGetOptions{}, "bpm-process-history", "Show process history", func(s *mcclient.ClientSession, opts *BpmProcessGetOptions) error {
		info, err := thirdparty.BpmProcess.GetProcessHistorys(s, opts.ID)
		if err != nil {
			return err
		}
		printObject(info)
		return nil
	})
	R(&BpmProcessGetOptions{}, "bpm-process-svg", "Show coa department info", func(s *mcclient.ClientSession, opts *BpmProcessGetOptions) error {
		info, err := thirdparty.BpmProcess.GetProcessHistorys(s, opts.ID)
		if err != nil {
			return err
		}
		printObject(info)
		return nil
	})
	type CoaSendMarkdownMessageOptions struct {
		Platform     int    `json:"platform"`
		FeishuUserId string `json:"feishu_user_id"`
		Content      string `json:"content"`
	}
}