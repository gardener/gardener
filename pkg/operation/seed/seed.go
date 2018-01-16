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

package seed

import (
	"fmt"
	"path/filepath"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// New takes a <k8sGardenClient>, the <k8sGardenInformers> and a <seed> manifest, and creates a new Seed representation.
// It will add the CloudProfile and identify the cloud provider.
func New(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.Interface, seed *gardenv1beta1.Seed) (*Seed, error) {
	secret, err := k8sGardenClient.GetSecret(seed.Spec.SecretRef.Namespace, seed.Spec.SecretRef.Name)
	if err != nil {
		return nil, err
	}

	cloudProfile, err := k8sGardenInformers.CloudProfiles().Lister().Get(seed.Spec.Cloud.Profile)
	if err != nil {
		return nil, err
	}

	seedObj := &Seed{
		Info:         seed,
		Secret:       secret,
		CloudProfile: cloudProfile,
	}

	cloudProvider, err := helper.DetermineCloudProviderInProfile(cloudProfile.Spec)
	if err != nil {
		return nil, err
	}
	seedObj.CloudProvider = cloudProvider

	return seedObj, nil
}

// NewFromName creates a new Seed object based on the name of a Seed manifest.
func NewFromName(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.Interface, seedName string) (*Seed, error) {
	seed, err := k8sGardenInformers.Seeds().Lister().Get(seedName)
	if err != nil {
		return nil, err
	}
	return New(k8sGardenClient, k8sGardenInformers, seed)
}

// List returns a list of Seed clusters (along with the referenced secrets).
func List(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.Interface) ([]*Seed, error) {
	var seedList []*Seed

	list, err := k8sGardenInformers.Seeds().Lister().List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, obj := range list {
		seed, err := New(k8sGardenClient, k8sGardenInformers, obj)
		if err != nil {
			return nil, err
		}
		seedList = append(seedList, seed)
	}

	return seedList, nil
}

// BootstrapClusters bootstraps the Seeds cluster and deploys various required manifests.
func BootstrapClusters(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.Interface, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector) error {
	const chartName = "seed-bootstrap"

	kubeStateMetricsImage, err := imageVector.FindImage("kube-state-metrics", k8sGardenClient.Version())
	if err != nil {
		return err
	}
	addonResizer, err := imageVector.FindImage("addon-resizer", k8sGardenClient.Version())
	if err != nil {
		return err
	}

	seedList, err := List(k8sGardenClient, k8sGardenInformers)
	if err != nil {
		return err
	}

	for _, seed := range seedList {
		logger.Logger.Infof("Bootstrapping Seed cluster %s...", seed.Info.Name)

		k8sSeedClient, err := kubernetes.NewClientFromSecretObject(seed.Secret)
		if err != nil {
			return err
		}
		if err := common.ApplyChart(
			k8sSeedClient,
			chartrenderer.New(k8sSeedClient),
			filepath.Join("charts", chartName),
			chartName,
			metav1.NamespaceSystem,
			nil,
			map[string]interface{}{
				"cloudProvider": seed.CloudProvider,
				"images": map[string]interface{}{
					"kube-state-metrics": kubeStateMetricsImage.String(),
					"addon-resizer":      addonResizer.String(),
				},
			},
		); err != nil {
			return err
		}

		err = common.EnsureImagePullSecrets(k8sSeedClient, metav1.NamespaceSystem, secrets, true, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetIngressFQDN returns the fully qualified domain name of ingress sub-resource for the Seed cluster. The
// end result is '<subDomain>.<shoot-name>.<garden-namespace>.ingress.<seed-fqdn>'. It must not exceed 64
// characters in length (see RFC-5280 for details).
func (s *Seed) GetIngressFQDN(subDomain, shootName, shootNamespace string) (string, error) {
	result := fmt.Sprintf("%s.%s.%s.ingress.%s", subDomain, shootName, shootNamespace, s.Info.Spec.Domain)
	if len(result) > 64 {
		return "", fmt.Errorf("the FQDN for '%s' cannot be longer than 64 characters", result)
	}
	return result, nil
}
