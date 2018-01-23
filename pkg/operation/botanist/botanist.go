// Copyright 2018 The Gardener Authors.
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
	"strings"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
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
	if o.Shoot.Info.Spec.DNS.Domain != nil {
		var (
			prefix            = fmt.Sprintf("%s-", common.GardenRoleDefaultDomain)
			defaultDomainKeys = o.GetSecretKeysOfRole(common.GardenRoleDefaultDomain)
		)
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

// InitializeSeedClients will use the Garden Kubernetes client to read the Seed Secret in the Garden
// cluster which contains a Kubeconfig that can be used to authenticate against the Seed cluster. With it,
// a Kubernetes client as well as a Chart renderer for the Seed cluster will be initialized and attached to
// the already existing Botanist object.
func (b *Botanist) InitializeSeedClients() error {
	k8sSeedClient, err := kubernetes.NewClientFromSecretObject(b.Seed.Secret)
	if err != nil {
		return err
	}
	chartSeedRenderer := chartrenderer.New(k8sSeedClient)

	// Check whether the Kubernetes version of the Seed cluster fulfills the minimal requirements.
	var minSeedVersion string
	switch b.Seed.CloudProvider {
	case gardenv1beta1.CloudProviderAzure:
		minSeedVersion = "1.8.6" // https://github.com/kubernetes/kubernetes/issues/56898
	default:
		minSeedVersion = "1.7"
	}
	seedVersionOK, err := utils.CompareVersions(k8sSeedClient.Version(), ">=", minSeedVersion)
	if err != nil {
		return err
	}
	if !seedVersionOK {
		return fmt.Errorf("the Kubernetes version of the Seed cluster must be at least %s", minSeedVersion)
	}

	b.Operation.K8sSeedClient = k8sSeedClient
	b.Operation.ChartSeedRenderer = chartSeedRenderer
	return nil
}

// InitializeShootClients will use the Seed Kubernetes client to read the gardener Secret in the Seed
// cluster which contains a Kubeconfig that can be used to authenticate against the Shoot cluster. With it,
// a Kubernetes client as well as a Chart renderer for the Shoot cluster will be initialized and attached to
// the already existing Botanist object.
func (b *Botanist) InitializeShootClients() error {
	k8sShootClient, err := kubernetes.NewClientFromSecret(b.K8sSeedClient, b.Shoot.SeedNamespace, "gardener")
	if err != nil {
		return err
	}
	chartShootRenderer := chartrenderer.New(k8sShootClient)

	b.Operation.K8sShootClient = k8sShootClient
	b.Operation.ChartShootRenderer = chartShootRenderer
	return nil
}

// RegisterAsSeed registers a Shoot cluster as a Seed in the Garden cluster.
func (b *Botanist) RegisterAsSeed() error {
	if b.Shoot.Info.Spec.DNS.Domain == nil {
		return errors.New("cannot register Shoot as Seed if it does not specify a domain")
	}

	var (
		secretData      = b.Shoot.Secret.Data
		secretName      = fmt.Sprintf("seed-%s", b.Shoot.Info.Name)
		secretNamespace = common.GardenNamespace
	)
	secretData["kubeconfig"] = b.Secrets["kubecfg"].Data["kubeconfig"]

	if _, err := b.K8sGardenClient.CreateSecret(secretNamespace, secretName, corev1.SecretTypeOpaque, secretData, true); err != nil {
		return err
	}

	k8sNetworks := b.Shoot.GetK8SNetworks()
	if k8sNetworks == nil {
		return errors.New("could not retrieve K8SNetworks from the Shoot resource")
	}

	_, err := b.K8sGardenClient.CreateSeed(&gardenv1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name: b.Shoot.Info.Name,
		},
		Spec: gardenv1beta1.SeedSpec{
			Cloud: gardenv1beta1.SeedCloud{
				Profile: b.Shoot.Info.Spec.Cloud.Profile,
				Region:  b.Shoot.Info.Spec.Cloud.Region,
			},
			Domain: *(b.Shoot.Info.Spec.DNS.Domain),
			SecretRef: gardenv1beta1.CrossReference{
				Name:      secretName,
				Namespace: secretNamespace,
			},
			Networks: *k8sNetworks,
		},
	})
	return err
}

// UnregisterAsSeed unregisters a Shoot cluster as a Seed in the Garden cluster.
func (b *Botanist) UnregisterAsSeed() error {
	seed, err := b.K8sGardenClient.GetSeed(b.Shoot.Info.Name)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	err = b.K8sGardenClient.DeleteSeed(seed.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	err = b.K8sGardenClient.DeleteSecret(seed.Spec.SecretRef.Namespace, seed.Spec.SecretRef.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
