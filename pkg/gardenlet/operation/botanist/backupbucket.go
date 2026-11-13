// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	corebackupbucket "github.com/gardener/gardener/pkg/component/garden/backupbucket"
)

// DefaultCoreBackupBucket creates the default deployer for the core.gardener.cloud/v1beta1.BackupBucket resource.
func (b *Botanist) DefaultCoreBackupBucket() corebackupbucket.Interface {
	if !b.Shoot.IsSelfHosted() {
		return nil
	}

	backupConfig := v1beta1helper.GetBackupConfigForShoot(b.Shoot.GetInfo(), nil)
	name := string(b.Shoot.GetInfo().Status.UID)
	if backupConfig != nil && backupConfig.BucketName != nil {
		name = *backupConfig.BucketName
	}
	return corebackupbucket.New(b.Logger, b.GardenClient, &corebackupbucket.Values{
		Name:          name,
		Config:        backupConfig,
		DefaultRegion: b.Shoot.GetInfo().Spec.Region,
		Clock:         b.Clock,
		Shoot:         b.Shoot.GetInfo(),
	}, corebackupbucket.DefaultInterval, corebackupbucket.DefaultTimeout)
}
