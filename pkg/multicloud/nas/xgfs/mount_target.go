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
	"time"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/cloudprovider"
	"yunion.io/x/onecloud/pkg/multicloud/nas"
	"yunion.io/x/pkg/errors"
)

type SDfsNfsShare struct {
	fs *SXgfsFileSystem
	*nas.SMountTarget
	action string
}

func (self *SDfsNfsShare) GetId() int {
	return self.Id
}

func (self *SDfsNfsShare) GetActionStatus() string {
	return self.action
}

func (self *SDfsNfsShare) Delete() error {
	_, err := self.fs.DeleteDfsNfsShare(self.GetId())
	if err != nil {
		log.Errorf("DeleteDfsNfsShare err %s", err)
		return err
	}
	return cloudprovider.Wait(time.Second*3, time.Minute*5, func() (bool, error) {
		mts, err := self.fs.GetMountTargets()
		if err != nil {
			log.Errorf("DeleteDfsNfsShare err %s", err)
			return false, errors.Wrapf(err, "iFs.GetMountTargets")
		}
		if len(mts) == 0 {
			return true, nil
		}
		return false, nil
	})
}

//func (self *SDfsNfsShare) DeleteMountTarget(fsId, id string) error {
//	params := map[string]string{
//		"RegionId":          self.RegionId,
//		"FileSystemId":      fsId,
//		"MountTargetDomain": id,
//	}
//	_, err := self.nasRequest("DeleteMountTarget", params)
//	return errors.Wrapf(err, "DeleteMountTarget")
//}
