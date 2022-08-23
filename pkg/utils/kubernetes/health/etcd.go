// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package health

import (
	"fmt"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"k8s.io/utils/pointer"
)

// CheckEtcd checks whether the given Etcd is healthy.
// An Etcd is considered healthy if its ready field in status is true and the BackupReady condition doesn't report false.
func CheckEtcd(etcd *druidv1alpha1.Etcd) error {
	if !pointer.BoolDeref(etcd.Status.Ready, false) {
		return fmt.Errorf("etcd %q is not ready yet", etcd.Name)
	}

	for _, cond := range etcd.Status.Conditions {
		if cond.Type != druidv1alpha1.ConditionTypeBackupReady {
			continue
		}

		// TODO(timuthy): Check for cond.Status != druidv1alpha1.ConditionTrue as soon as https://github.com/gardener/etcd-druid/issues/413 is resolved.
		if cond.Status == druidv1alpha1.ConditionFalse {
			return fmt.Errorf("backup for etcd %q is reported as unready: %s", etcd.Name, cond.Message)
		}
	}

	return nil
}
