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

package operation

import (
	"fmt"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"k8s.io/client-go/util/retry"
	"path/filepath"
	"strings"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	machineclientset "github.com/gardener/gardener/pkg/client/machine/clientset/versioned"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// New creates a new operation object with a Shoot resource object.
func New(shoot *gardenv1beta1.Shoot, logger *logrus.Entry, k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.Interface, gardenerInfo *gardenv1beta1.Gardener, secretsMap map[string]*corev1.Secret, imageVector imagevector.ImageVector) (*Operation, error) {
	return newOperation(logger, k8sGardenClient, k8sGardenInformers, gardenerInfo, secretsMap, imageVector, shoot.Namespace, *(shoot.Spec.Cloud.Seed), shoot, nil)
}

// NewWithBackupInfrastructure creates a new operation object without a Shoot resource object but the BackupInfrastructure resource.
func NewWithBackupInfrastructure(backupInfrastructure *gardenv1beta1.BackupInfrastructure, logger *logrus.Entry, k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.Interface, gardenerInfo *gardenv1beta1.Gardener, secretsMap map[string]*corev1.Secret, imageVector imagevector.ImageVector) (*Operation, error) {
	return newOperation(logger, k8sGardenClient, k8sGardenInformers, gardenerInfo, secretsMap, imageVector, backupInfrastructure.Namespace, backupInfrastructure.Spec.Seed, nil, backupInfrastructure)
}

func newOperation(
	logger *logrus.Entry,
	k8sGardenClient kubernetes.Client,
	k8sGardenInformers gardeninformers.Interface,
	gardenerInfo *gardenv1beta1.Gardener,
	secretsMap map[string]*corev1.Secret,
	imageVector imagevector.ImageVector,
	namespace,
	seedName string,
	shoot *gardenv1beta1.Shoot,
	backupInfrastructure *gardenv1beta1.BackupInfrastructure,
) (*Operation, error) {

	secrets := make(map[string]*corev1.Secret)
	for k, v := range secretsMap {
		secrets[k] = v
	}

	gardenObj, err := garden.New(k8sGardenInformers.Projects().Lister(), namespace)
	if err != nil {
		return nil, err
	}
	seedObj, err := seed.NewFromName(k8sGardenClient, k8sGardenInformers, seedName)
	if err != nil {
		return nil, err
	}

	chartRenderer, err := chartrenderer.New(k8sGardenClient)
	if err != nil {
		return nil, err
	}

	operation := &Operation{
		Logger:               logger,
		GardenerInfo:         gardenerInfo,
		Secrets:              secrets,
		ImageVector:          imageVector,
		CheckSums:            make(map[string]string),
		Garden:               gardenObj,
		Seed:                 seedObj,
		K8sGardenClient:      k8sGardenClient,
		K8sGardenInformers:   k8sGardenInformers,
		ChartGardenRenderer:  chartRenderer,
		BackupInfrastructure: backupInfrastructure,
		MachineDeployments:   MachineDeployments{},
	}

	if shoot != nil {
		internalDomain := constructInternalDomain(shoot.Name, gardenObj.Project.Name, secretsMap[common.GardenRoleInternalDomain].Annotations[common.DNSDomain])
		shootObj, err := shootpkg.New(k8sGardenClient, k8sGardenInformers, shoot, gardenObj.Project.Name, internalDomain)
		if err != nil {
			return nil, err
		}
		operation.Shoot = shootObj

		shootedSeed, err := helper.ReadShootedSeed(shoot)
		if err != nil {
			logger.Warnf("Cannot use shoot %s/%s as shooted seed: %+v", shoot.Namespace, shoot.Name, err)
		} else {
			operation.ShootedSeed = shootedSeed
		}
	}

	return operation, nil
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
	// Create a MachineV1alpha1Client and the respective API group scheme for the Machine API group.
	machineClientset, err := machineclientset.NewForConfig(k8sSeedClient.GetConfig())
	if err != nil {
		return err
	}
	k8sSeedClient.SetMachineClientset(machineClientset)

	o.K8sSeedClient = k8sSeedClient
	o.ChartSeedRenderer, err = chartrenderer.New(k8sSeedClient)
	if err != nil {
		return err
	}
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

	o.K8sShootClient = k8sShootClient
	o.ChartShootRenderer, err = chartrenderer.New(k8sShootClient)
	if err != nil {
		return err
	}
	return nil
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

func makeDescription(stats *flow.Stats) string {
	if stats.ProgressPercent() == 100 {
		return "Execution finished"
	}
	return strings.Join(stats.Running.StringList(), ", ")
}

// ReportShootProgress will update the last operation object in the Shoot manifest `status` section
// by the current progress of the Flow execution.
func (o *Operation) ReportShootProgress(stats *flow.Stats) {
	var (
		description    = makeDescription(stats)
		progress       = stats.ProgressPercent()
		lastUpdateTime = metav1.Now()
	)

	newShoot, err := kutil.TryUpdateShootStatus(o.K8sGardenClient.GardenClientset(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
		func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
			if shoot.Status.LastOperation == nil {
				return nil, fmt.Errorf("last operation of Shoot %s/%s is unset", shoot.Namespace, shoot.Name)
			}
			if shoot.Status.LastOperation.LastUpdateTime.After(lastUpdateTime.Time) {
				return nil, fmt.Errorf("last operation of Shoot %s/%s was updated mid-air", shoot.Namespace, shoot.Name)
			}
			shoot.Status.LastOperation.Description = description
			shoot.Status.LastOperation.Progress = progress
			shoot.Status.LastOperation.LastUpdateTime = lastUpdateTime
			return shoot, nil
		})
	if err != nil {
		o.Logger.Errorf("Could not report shoot progress: %v", err)
		return
	}

	o.Shoot.Info = newShoot
}

// ReportBackupInfrastructureProgress will update the phase and error in the BackupInfrastructure manifest `status` section
// by the current progress of the Flow execution.
func (o *Operation) ReportBackupInfrastructureProgress(stats *flow.Stats) {
	o.BackupInfrastructure.Status.LastOperation.Description = makeDescription(stats)
	o.BackupInfrastructure.Status.LastOperation.Progress = stats.ProgressPercent()
	o.BackupInfrastructure.Status.LastOperation.LastUpdateTime = metav1.Now()

	if newBackupInfrastructure, err := o.K8sGardenClient.GardenClientset().GardenV1beta1().BackupInfrastructures(o.BackupInfrastructure.Namespace).UpdateStatus(o.BackupInfrastructure); err == nil {
		o.BackupInfrastructure = newBackupInfrastructure
	}
}

// InjectImages injects images from the image vector into the provided <values> map.
func (o *Operation) InjectImages(values map[string]interface{}, k8sVersionRuntime, k8sVersionTarget string, images ...string) (map[string]interface{}, error) {
	var (
		copy = make(map[string]interface{})
		i    = make(map[string]interface{})
	)

	for k, v := range values {
		copy[k] = v
	}

	for _, imageName := range images {
		image, err := o.ImageVector.FindImage(imageName, k8sVersionRuntime, k8sVersionTarget)
		if err != nil {
			return nil, err
		}
		i[imageName] = image.String()
	}

	copy["images"] = i
	return copy, nil
}

// ComputeDownloaderCloudConfig computes the downloader cloud config which is injected as user data while
// creating machines/VMs. It needs the name of the worker group it is used for (<workerName>) and returns
// the rendered chart.
func (o *Operation) ComputeDownloaderCloudConfig(workerName string) (*chartrenderer.RenderedChart, error) {
	config := map[string]interface{}{
		"kubeconfig": string(o.Secrets["cloud-config-downloader"].Data["kubeconfig"]),
		"secretName": o.Shoot.ComputeCloudConfigSecretName(workerName),
	}

	values, err := o.InjectImages(config, o.ShootVersion(), o.ShootVersion(), common.RubyImageName)
	if err != nil {
		return nil, err
	}

	return o.ChartShootRenderer.Render(filepath.Join(common.ChartPath, "shoot-cloud-config", "charts", "downloader"), "shoot-cloud-config-downloader", metav1.NamespaceSystem, values)
}

// ComputeOriginalCloudConfig computes the original cloud config which is downloaded by the cloud config
// downloader process running on machines/VMs. It will regularly check for new versions and restart all
// units once it finds a newer state.
func (o *Operation) ComputeOriginalCloudConfig(config map[string]interface{}) (*chartrenderer.RenderedChart, error) {
	return o.ChartShootRenderer.Render(filepath.Join(common.ChartPath, "shoot-cloud-config", "charts", "original"), "shoot-cloud-config-original", metav1.NamespaceSystem, config)
}

// SeedVersion is a shorthand for the kubernetes version of the K8sSeedClient.
func (o *Operation) SeedVersion() string {
	return o.K8sSeedClient.Version()
}

// ShootVersion is a shorthand for the desired kubernetes version of the operation's shoot.
func (o *Operation) ShootVersion() string {
	return o.Shoot.Info.Spec.Kubernetes.Version
}

// constructInternalDomain constructs the domain pointing to the kube-apiserver of a Shoot cluster
// which is only used for internal purposes (all kubeconfigs except the one which is received by the
// user will only talk with the kube-apiserver via this domain). In case the given <internalDomain>
// already contains "internal", the result is constructed as "api.<shootName>.<shootProject>.<internalDomain>."
// In case it does not, the word "internal" will be appended, resulting in
// "api.<shootName>.<shootProject>.internal.<internalDomain>".
func constructInternalDomain(shootName, shootProject, internalDomain string) string {
	if strings.Contains(internalDomain, common.InternalDomainKey) {
		return fmt.Sprintf("api.%s.%s.%s", shootName, shootProject, internalDomain)
	}
	return fmt.Sprintf("api.%s.%s.%s.%s", shootName, shootProject, common.InternalDomainKey, internalDomain)
}

// ContainsName checks whether the <name> is part of the <machineDeployments>
// list, i.e. whether there is an entry whose 'Name' attribute matches <name>. It returns true or false.
func (m MachineDeployments) ContainsName(name string) bool {
	for _, deployment := range m {
		if name == deployment.Name {
			return true
		}
	}
	return false
}

// ContainsClass checks whether the <className> is part of the <machineDeployments>
// list, i.e. whether there is an entry whose 'ClassName' attribute matches <name>. It returns true or false.
func (m MachineDeployments) ContainsClass(className string) bool {
	for _, deployment := range m {
		if className == deployment.ClassName {
			return true
		}
	}
	return false
}
