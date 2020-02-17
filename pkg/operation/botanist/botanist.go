// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// New takes an operation object <o> and creates a new Botanist object. It checks whether the given Shoot DNS
// domain is covered by a default domain, and if so, it sets the <DefaultDomainSecret> attribute on the Botanist
// object.
func New(o *operation.Operation) (*Botanist, error) {
	b := &Botanist{
		Operation: o,
	}

	// Determine all default domain secrets and check whether the used Shoot domain matches a default domain.
	if o.Shoot != nil && o.Shoot.Info.Spec.DNS != nil && o.Shoot.Info.Spec.DNS.Domain != nil {
		var (
			prefix            = fmt.Sprintf("%s-", common.GardenRoleDefaultDomain)
			defaultDomainKeys = o.GetSecretKeysOfRole(common.GardenRoleDefaultDomain)
		)
		sort.Slice(defaultDomainKeys, func(i, j int) bool { return len(defaultDomainKeys[i]) >= len(defaultDomainKeys[j]) })
		for _, key := range defaultDomainKeys {
			defaultDomain := strings.SplitAfter(key, prefix)[1]
			if strings.HasSuffix(*(o.Shoot.Info.Spec.DNS.Domain), defaultDomain) {
				b.DefaultDomainSecret = b.Secrets[prefix+defaultDomain]
				break
			}
		}
	}

	if err := b.InitializeSeedClients(); err != nil {
		return nil, err
	}

	return b, nil
}

// RequiredExtensionsExist checks whether all required extensions needed for an shoot operation exist.
func (b *Botanist) RequiredExtensionsExist() error {
	controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
	if err := b.K8sGardenClient.Client().List(context.TODO(), controllerInstallationList); err != nil {
		return err
	}

	requiredExtensions := b.computeRequiredExtensions()

	for _, controllerInstallation := range controllerInstallationList.Items {
		if controllerInstallation.Spec.SeedRef.Name != b.Seed.Info.Name {
			continue
		}

		controllerRegistration := &gardencorev1beta1.ControllerRegistration{}
		if err := b.K8sGardenClient.Client().Get(context.TODO(), client.ObjectKey{Name: controllerInstallation.Spec.RegistrationRef.Name}, controllerRegistration); err != nil {
			return err
		}

		for extensionKind, extensionTypes := range requiredExtensions {
			for extensionType := range extensionTypes {
				if helper.IsResourceSupported(controllerRegistration.Spec.Resources, extensionKind, extensionType) && helper.IsControllerInstallationSuccessful(controllerInstallation) {
					extensionTypes.Delete(extensionType)
				}
			}
			if extensionTypes.Len() == 0 {
				delete(requiredExtensions, extensionKind)
			}
		}
	}

	if len(requiredExtensions) > 0 {
		return fmt.Errorf("extension controllers missing or unready: %+v", requiredExtensions)
	}

	return nil
}

func (b *Botanist) computeRequiredExtensions() map[string]sets.String {
	requiredExtensions := make(map[string]sets.String)

	machineImagesSet := sets.NewString()
	for _, worker := range b.Shoot.Info.Spec.Provider.Workers {
		if worker.Machine.Image != nil {
			machineImagesSet.Insert(string(worker.Machine.Image.Name))
		}
	}
	requiredExtensions[extensionsv1alpha1.OperatingSystemConfigResource] = machineImagesSet

	if !b.Shoot.DisableDNS {
		requiredExtensions[dnsv1alpha1.DNSProviderKind] = sets.NewString()
		if b.Garden.InternalDomain.Provider != "unmanaged" {
			requiredExtensions[dnsv1alpha1.DNSProviderKind].Insert(b.Garden.InternalDomain.Provider)
		}

		if b.Shoot.ExternalDomain != nil && b.Shoot.ExternalDomain.Provider != "unmanaged" {
			requiredExtensions[dnsv1alpha1.DNSProviderKind].Insert(b.Shoot.ExternalDomain.Provider)
		}

		if b.Shoot.Info.Spec.DNS != nil {
			for _, provider := range b.Shoot.Info.Spec.DNS.Providers {
				if provider.Type != nil && *provider.Type != core.DNSUnmanaged {
					requiredExtensions[dnsv1alpha1.DNSProviderKind].Insert(*provider.Type)
				}
			}
		}
	}

	for extensionType := range b.Shoot.Extensions {
		if requiredExtensions[extensionsv1alpha1.ExtensionResource] == nil {
			requiredExtensions[extensionsv1alpha1.ExtensionResource] = sets.NewString()
		}
		requiredExtensions[extensionsv1alpha1.ExtensionResource].Insert(extensionType)
	}

	requiredExtensions[extensionsv1alpha1.InfrastructureResource] = sets.NewString(string(b.Shoot.Info.Spec.Provider.Type))
	requiredExtensions[extensionsv1alpha1.ControlPlaneResource] = sets.NewString(string(b.Shoot.Info.Spec.Provider.Type))
	requiredExtensions[extensionsv1alpha1.NetworkResource] = sets.NewString(b.Shoot.Info.Spec.Networking.Type)
	requiredExtensions[extensionsv1alpha1.WorkerResource] = sets.NewString(string(b.Shoot.Info.Spec.Provider.Type))

	if b.Seed.Info.Spec.Backup != nil {
		requiredExtensions[extensionsv1alpha1.BackupBucketResource] = sets.NewString(string(b.Seed.Info.Spec.Backup.Provider))
		requiredExtensions[extensionsv1alpha1.BackupEntryResource] = sets.NewString(string(b.Seed.Info.Spec.Backup.Provider))
	}

	return requiredExtensions
}
