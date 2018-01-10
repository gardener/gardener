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
	"fmt"
	"strings"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
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
