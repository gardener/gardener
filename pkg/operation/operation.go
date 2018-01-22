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

package operation

import (
	"fmt"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// New creates a new operation object.
func New(shoot *gardenv1beta1.Shoot, shootLogger *logrus.Entry, k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.Interface, gardenerInfo *gardenv1beta1.Gardener, secretsMap map[string]*corev1.Secret, imageVector imagevector.ImageVector) (*Operation, error) {
	secrets := make(map[string]*corev1.Secret)
	for k, v := range secretsMap {
		secrets[k] = v
	}

	gardenObj := garden.New(shoot)
	seedObj, err := seed.NewFromName(k8sGardenClient, k8sGardenInformers, *(shoot.Spec.Cloud.Seed))
	if err != nil {
		return nil, err
	}
	shootObj, err := shootpkg.New(k8sGardenClient, k8sGardenInformers, shoot, fmt.Sprintf("api.internal.%s.%s.%s", shoot.Name, gardenObj.ProjectName, secrets[common.GardenRoleInternalDomain].Annotations[common.DNSDomain]))
	if err != nil {
		return nil, err
	}

	return &Operation{
		Logger:              shootLogger,
		GardenerInfo:        gardenerInfo,
		Secrets:             secrets,
		ImageVector:         imageVector,
		CheckSums:           make(map[string]string),
		Garden:              gardenObj,
		Seed:                seedObj,
		Shoot:               shootObj,
		K8sGardenClient:     k8sGardenClient,
		K8sGardenInformers:  k8sGardenInformers,
		ChartGardenRenderer: chartrenderer.New(k8sGardenClient),
	}, nil
}

// InitializeSeedClients will use the Garden Kubernetes client to read the Seed Secret in the Garden
// cluster which contains a Kubeconfig that can be used to authenticate against the Seed cluster. With it,
// a Kubernetes client as well as a Chart renderer for the Seed cluster will be initialized and attached to
// the already existing Operation object.
func (o *Operation) InitializeSeedClients() error {
	if o.K8sSeedClient != nil && o.ChartSeedRenderer != nil {
		return nil
	}

	k8sSeedClient, err := kubernetes.NewClientFromSecretObject(o.Seed.Secret)
	if err != nil {
		return err
	}
	chartSeedRenderer := chartrenderer.New(k8sSeedClient)

	o.K8sSeedClient = k8sSeedClient
	o.ChartSeedRenderer = chartSeedRenderer
	return nil
}

// InitializeShootClients will use the Seed Kubernetes client to read the gardener Secret in the Seed
// cluster which contains a Kubeconfig that can be used to authenticate against the Shoot cluster. With it,
// a Kubernetes client as well as a Chart renderer for the Shoot cluster will be initialized and attached to
// the already existing Operation object.
func (o *Operation) InitializeShootClients() error {
	if o.K8sShootClient != nil && o.ChartShootRenderer != nil {
		return nil
	}

	k8sShootClient, err := kubernetes.NewClientFromSecret(o.K8sSeedClient, o.Shoot.SeedNamespace, gardenv1beta1.GardenerName)
	if err != nil {
		return err
	}
	chartShootRenderer := chartrenderer.New(k8sShootClient)

	o.K8sShootClient = k8sShootClient
	o.ChartShootRenderer = chartShootRenderer
	return nil
}

// EnsureImagePullSecretsGarden ensures that the image pull secrets do exist in the Garden cluster
// namespace in which the Shoot resource has been created, and that the default service account in
// that namespace contains the respective .imagePullSecrets[] field.
func (o *Operation) EnsureImagePullSecretsGarden() error {
	return common.EnsureImagePullSecrets(o.K8sGardenClient, o.Shoot.Info.Namespace, o.Secrets, true, o.Logger)
}

// EnsureImagePullSecretsSeed ensures that the image pull secrets do exist in the Seed cluster's
// Shoot namespace and that the default service account in that namespace contains the respective
// .imagePullSecrets[] field.
func (o *Operation) EnsureImagePullSecretsSeed() error {
	return common.EnsureImagePullSecrets(o.K8sSeedClient, o.Shoot.SeedNamespace, o.Secrets, true, o.Logger)
}

// EnsureImagePullSecretsShoot ensures that the image pull secrets do exist in the Shoot cluster's
// kube-system namespace and that the default service account in that namespace contains the
// respective .imagePullSecrets[] field.
func (o *Operation) EnsureImagePullSecretsShoot() error {
	return common.EnsureImagePullSecrets(o.K8sShootClient, metav1.NamespaceSystem, o.Secrets, true, o.Logger)
}

// ApplyChartGarden takes a path to a chart <chartPath>, name of the release <name>, release's namespace <namespace>
// and two maps <defaultValues>, <additionalValues>, and renders the template based on the merged result of both value maps.
// The resulting manifest will be applied to the Garden cluster.
func (o *Operation) ApplyChartGarden(chartPath, name, namespace string, defaultValues, additionalValues map[string]interface{}) error {
	return common.ApplyChart(o.K8sGardenClient, o.ChartGardenRenderer, chartPath, name, namespace, defaultValues, additionalValues)
}

// ApplyChartSeed takes a path to a chart <chartPath>, name of the release <name>, release's namespace <namespace>
// and two maps <defaultValues>, <additionalValues>, and renders the template based on the merged result of both value maps.
// The resulting manifest will be applied to the Seed cluster.
func (o *Operation) ApplyChartSeed(chartPath, name, namespace string, defaultValues, additionalValues map[string]interface{}) error {
	return common.ApplyChart(o.K8sSeedClient, o.ChartSeedRenderer, chartPath, name, namespace, defaultValues, additionalValues)
}

// ApplyChartShoot takes a path to a chart <chartPath>, name of the release <name>, release's namespace <namespace>
// and two maps <defaultValues>, <additionalValues>, and renders the template based on the merged result of both value maps.
// The resulting manifest will be applied to the Shoot cluster.
func (o *Operation) ApplyChartShoot(chartPath, name, namespace string, defaultValues, additionalValues map[string]interface{}) error {
	return common.ApplyChart(o.K8sShootClient, o.ChartShootRenderer, chartPath, name, namespace, defaultValues, additionalValues)
}

// GetSecretKeysOfRole returns a list of keys which are present in the Garden Secrets map and which
// are prefixed with <kind>.
func (o *Operation) GetSecretKeysOfRole(kind string) []string {
	return common.GetSecretKeysWithPrefix(kind, o.Secrets)
}

// GetImagePullSecretsMap returns all known image pull secrets as map whereas the key is "name" and
// the value is the respective name of the image pull secret. The map can be used to specify a list
// of image pull secrets on a Kubernetes PodTemplateSpec object.
func (o *Operation) GetImagePullSecretsMap() []map[string]interface{} {
	imagePullSecrets := []map[string]interface{}{}
	for _, key := range o.GetSecretKeysOfRole(common.GardenRoleImagePull) {
		imagePullSecrets = append(imagePullSecrets, map[string]interface{}{
			"name": o.Secrets[key].Name,
		})
	}
	return imagePullSecrets
}

// ReportShootProgress will update the last operation object in the Shoot manifest `status` section
// by the current progress of the Flow execution.
func (o *Operation) ReportShootProgress(progress int, currentFunctions string) {
	o.Shoot.Info.Status.LastOperation.Description = "Currently executing " + currentFunctions
	o.Shoot.Info.Status.LastOperation.Progress = progress
	o.Shoot.Info.Status.LastOperation.LastUpdateTime = metav1.Now()

	newShoot, err := o.K8sGardenClient.UpdateShootStatus(o.Shoot.Info)
	if err == nil {
		o.Shoot.Info = newShoot
	}
}

// InjectImages injects images from the image vector into the provided <values> map.
func (o *Operation) InjectImages(values map[string]interface{}, version string, imageMap map[string]string) (map[string]interface{}, error) {
	if values == nil {
		return nil, nil
	}

	copy := make(map[string]interface{})
	for k, v := range values {
		copy[k] = v
	}

	i := make(map[string]interface{})
	for keyInChart, imageName := range imageMap {
		image, err := o.ImageVector.FindImage(imageName, version)
		if err != nil {
			return nil, err
		}
		i[keyInChart] = image.String()
	}

	copy["images"] = i
	return copy, nil
}
