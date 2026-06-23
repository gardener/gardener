// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	corebackupbucket "github.com/gardener/gardener/pkg/component/garden/backupbucket"
)

// DefaultCoreBackupBucket creates the default deployer for the core.gardener.cloud/v1beta1.BackupBucket resource.
func (b *Botanist) DefaultCoreBackupBucket() corebackupbucket.Interface {
	return corebackupbucket.New(b.Logger, b.GardenClient, &corebackupbucket.Values{
		Name:          string(b.Shoot.GetInfo().Status.UID),
		Config:        v1beta1helper.GetBackupConfigForShoot(b.Shoot.GetInfo(), nil),
		DefaultRegion: b.Shoot.GetInfo().Spec.Region,
		Clock:         b.Clock,
		Shoot:         b.Shoot.GetInfo(),
	}, corebackupbucket.DefaultInterval, corebackupbucket.DefaultTimeout)
}
