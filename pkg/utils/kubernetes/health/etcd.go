// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"fmt"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"k8s.io/utils/ptr"
)

// CheckEtcd checks whether the given Etcd is healthy.
// An Etcd is considered healthy if its ready field in status is true and the BackupReady condition doesn't report false.
func CheckEtcd(etcd *druidcorev1alpha1.Etcd) error {
	if !ptr.Deref(etcd.Status.Ready, false) {
		return fmt.Errorf("etcd %q is not ready yet", etcd.Name)
	}

	for _, cond := range etcd.Status.Conditions {
		if cond.Type != druidcorev1alpha1.ConditionTypeBackupReady {
			continue
		}

		if cond.Status != druidcorev1alpha1.ConditionTrue {
			return fmt.Errorf("backup for etcd %q is reported as unready: %s", etcd.Name, cond.Message)
		}
	}

	return nil
}
