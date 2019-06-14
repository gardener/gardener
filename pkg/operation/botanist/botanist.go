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
	"errors"
	"fmt"
	"sort"
	"strings"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	if o.Shoot != nil && o.Shoot.Info.Spec.DNS.Domain != nil {
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

// RegisterAsSeed registers a Shoot cluster as a Seed in the Garden cluster.
func (b *Botanist) RegisterAsSeed(protected, visible *bool, minimumVolumeSize *string, blockCIDRs []gardencorev1alpha1.CIDR) error {
	if b.Shoot.Info.Spec.DNS.Domain == nil {
		return errors.New("cannot register Shoot as Seed if it does not specify a domain")
	}

	k8sNetworks, err := b.Shoot.GetK8SNetworks()
	if err != nil {
		return fmt.Errorf("could not retrieve K8SNetworks from the Shoot resource: %v", err)
	}

	var (
		secretData      = b.Shoot.Secret.Data
		secretName      = fmt.Sprintf("seed-%s", b.Shoot.Info.Name)
		secretNamespace = common.GardenNamespace
		controllerKind  = gardenv1beta1.SchemeGroupVersion.WithKind("Shoot")
		ownerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(b.Shoot.Info, controllerKind),
		}
		annotations map[string]string
	)

	if minimumVolumeSize != nil {
		annotations = map[string]string{
			common.AnnotatePersistentVolumeMinimumSize: *minimumVolumeSize,
		}
	}

	secretData["kubeconfig"] = b.Secrets["kubecfg"].Data["kubeconfig"]

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            secretName,
			Namespace:       secretNamespace,
			OwnerReferences: ownerReferences,
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}

	if _, err := b.K8sGardenClient.CreateSecretObject(secret, true); err != nil {
		return err
	}

	seed := &gardenv1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name:            b.Shoot.Info.Name,
			OwnerReferences: ownerReferences,
			Annotations:     annotations,
			Labels: map[string]string{
				common.GardenRole:   common.GardenRoleSeed,
				common.GardenerRole: common.GardenRoleSeed,
			},
		},
		Spec: gardenv1beta1.SeedSpec{
			Cloud: gardenv1beta1.SeedCloud{
				Profile: b.Shoot.Info.Spec.Cloud.Profile,
				Region:  b.Shoot.Info.Spec.Cloud.Region,
			},
			IngressDomain: fmt.Sprintf("%s.%s", common.IngressPrefix, *(b.Shoot.Info.Spec.DNS.Domain)),
			SecretRef: corev1.SecretReference{
				Name:      secretName,
				Namespace: secretNamespace,
			},
			Networks: gardenv1beta1.SeedNetworks{
				Pods:     *k8sNetworks.Pods,
				Services: *k8sNetworks.Services,
				Nodes:    *k8sNetworks.Nodes,
			},
			BlockCIDRs: blockCIDRs,
			Protected:  protected,
			Visible:    visible,
		},
	}
	_, err = b.K8sGardenClient.Garden().GardenV1beta1().Seeds().Create(seed)
	if apierrors.IsAlreadyExists(err) {
		_, err = b.K8sGardenClient.Garden().GardenV1beta1().Seeds().Update(seed)
	}
	return err
}

// UnregisterAsSeed unregisters a Shoot cluster as a Seed in the Garden cluster.
func (b *Botanist) UnregisterAsSeed() error {
	seed, err := b.K8sGardenClient.Garden().GardenV1beta1().Seeds().Get(b.Shoot.Info.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if err := b.K8sGardenClient.Garden().GardenV1beta1().Seeds().Delete(seed.Name, nil); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := b.K8sGardenClient.DeleteSecret(seed.Spec.SecretRef.Namespace, seed.Spec.SecretRef.Name); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// RequiredExtensionsExist checks whether all required extensions needed for an shoot operation exist.
func (b *Botanist) RequiredExtensionsExist() error {
	controllerInstallationList := &gardencorev1alpha1.ControllerInstallationList{}
	if err := b.K8sGardenClient.Client().List(context.TODO(), controllerInstallationList); err != nil {
		return err
	}

	requiredExtensions := b.computeRequiredExtensions()

	for _, controllerInstallation := range controllerInstallationList.Items {
		if controllerInstallation.Spec.SeedRef.Name != b.Seed.Info.Name {
			continue
		}

		controllerRegistration := &gardencorev1alpha1.ControllerRegistration{}
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

	requiredExtensions[extensionsv1alpha1.OperatingSystemConfigResource] = sets.NewString(string(b.Shoot.GetMachineImage().Name))

	if b.Garden.InternalDomain.Provider != gardenv1beta1.DNSUnmanaged {
		if requiredExtensions[dnsv1alpha1.DNSProviderKind] == nil {
			requiredExtensions[dnsv1alpha1.DNSProviderKind] = sets.NewString()
		}
		requiredExtensions[dnsv1alpha1.DNSProviderKind].Insert(b.Garden.InternalDomain.Provider)
	}

	if b.Shoot.ExternalDomain != nil && b.Shoot.ExternalDomain.Provider != gardenv1beta1.DNSUnmanaged {
		if requiredExtensions[dnsv1alpha1.DNSProviderKind] == nil {
			requiredExtensions[dnsv1alpha1.DNSProviderKind] = sets.NewString()
		}
		requiredExtensions[dnsv1alpha1.DNSProviderKind].Insert(b.Shoot.ExternalDomain.Provider)
	}

	for extensionType := range b.Shoot.Extensions {
		if requiredExtensions[extensionsv1alpha1.ExtensionResource] == nil {
			requiredExtensions[extensionsv1alpha1.ExtensionResource] = sets.NewString()
		}
		requiredExtensions[extensionsv1alpha1.ExtensionResource].Insert(extensionType)
	}

	requiredExtensions[extensionsv1alpha1.InfrastructureResource] = sets.NewString(string(b.Shoot.CloudProvider))
	requiredExtensions[extensionsv1alpha1.WorkerResource] = sets.NewString(string(b.Shoot.CloudProvider))

	return requiredExtensions
}
