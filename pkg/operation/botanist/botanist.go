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
	"errors"
	"fmt"
	"sort"
	"strings"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
func (b *Botanist) RegisterAsSeed(protected, visible *bool) error {
	if b.Shoot.Info.Spec.DNS.Domain == nil {
		return errors.New("cannot register Shoot as Seed if it does not specify a domain")
	}

	k8sNetworks := b.Shoot.GetK8SNetworks()
	if k8sNetworks == nil {
		return errors.New("could not retrieve K8SNetworks from the Shoot resource")
	}

	var (
		secretData      = b.Shoot.Secret.Data
		secretName      = fmt.Sprintf("seed-%s", b.Shoot.Info.Name)
		secretNamespace = common.GardenNamespace
		controllerKind  = gardenv1beta1.SchemeGroupVersion.WithKind("Shoot")
		ownerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(b.Shoot.Info, controllerKind),
		}
	)

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
			Protected: protected,
			Visible:   visible,
		},
	}
	_, err := b.K8sGardenClient.GardenClientset().GardenV1beta1().Seeds().Create(seed)
	if apierrors.IsAlreadyExists(err) {
		_, err = b.K8sGardenClient.GardenClientset().GardenV1beta1().Seeds().Update(seed)
	}
	return err
}

// UnregisterAsSeed unregisters a Shoot cluster as a Seed in the Garden cluster.
func (b *Botanist) UnregisterAsSeed() error {
	seed, err := b.K8sGardenClient.GardenClientset().GardenV1beta1().Seeds().Get(b.Shoot.Info.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if err := b.K8sGardenClient.GardenClientset().GardenV1beta1().Seeds().Delete(seed.Name, nil); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := b.K8sGardenClient.DeleteSecret(seed.Spec.SecretRef.Namespace, seed.Spec.SecretRef.Name); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
