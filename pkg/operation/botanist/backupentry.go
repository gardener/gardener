// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corebackupentry "github.com/gardener/gardener/pkg/operation/botanist/backupentry"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	extensionsbackupentry "github.com/gardener/gardener/pkg/operation/botanist/extensions/backupentry"
	"github.com/gardener/gardener/pkg/operation/common"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultCoreBackupEntry creates the default deployer for the core.gardener.cloud/v1beta1.BackupEntry resource.
func (b *Botanist) DefaultCoreBackupEntry(gardenClient client.Client) component.DeployWaiter {
	ownerRef := metav1.NewControllerRef(b.Shoot.Info, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot"))
	ownerRef.BlockOwnerDeletion = pointer.BoolPtr(false)

	return corebackupentry.New(
		b.Logger,
		gardenClient,
		&corebackupentry.Values{
			Namespace:         b.Shoot.Info.Namespace,
			Name:              common.GenerateBackupEntryName(b.Shoot.Info.Status.TechnicalID, b.Shoot.Info.Status.UID),
			OwnerReference:    ownerRef,
			SeedName:          pointer.StringPtr(b.Seed.Info.Name),
			OverwriteSeedName: b.isRestorePhase(),
			BucketName:        string(b.Seed.Info.UID),
		},
		corebackupentry.DefaultInterval,
		corebackupentry.DefaultTimeout,
	)
}

// DefaultExtensionsBackupEntry creates the default deployer for the extensions.gardener.cloud/v1alpha1.BackupEntry
// custom resource.
func (b *Botanist) DefaultExtensionsBackupEntry(seedClient client.Client) extensionsbackupentry.BackupEntry {
	return extensionsbackupentry.New(
		b.Logger,
		seedClient,
		&extensionsbackupentry.Values{
			Name: common.GenerateBackupEntryName(b.Shoot.Info.Status.TechnicalID, b.Shoot.Info.Status.UID),
		},
		extensionsbackupentry.DefaultInterval,
		extensionsbackupentry.DefaultSevereThreshold,
		extensionsbackupentry.DefaultTimeout,
	)
}
