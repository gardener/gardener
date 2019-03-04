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
	"crypto/x509"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/operation/terraformer"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/chartrenderer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"

	prometheusapi "github.com/prometheus/client_golang/api"
	prometheusclient "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// New creates a new operation object with a Shoot resource object.
func New(shoot *gardenv1beta1.Shoot, logger *logrus.Entry, k8sGardenClient kubernetes.Interface, k8sGardenInformers gardeninformers.Interface, gardenerInfo *gardenv1beta1.Gardener, secretsMap map[string]*corev1.Secret, imageVector imagevector.ImageVector, shootBackup *config.ShootBackup) (*Operation, error) {
	return newOperation(logger, k8sGardenClient, k8sGardenInformers, gardenerInfo, secretsMap, imageVector, shoot.Namespace, *(shoot.Spec.Cloud.Seed), shoot, nil, shootBackup)
}

// NewWithBackupInfrastructure creates a new operation object without a Shoot resource object but the BackupInfrastructure resource.
func NewWithBackupInfrastructure(backupInfrastructure *gardenv1beta1.BackupInfrastructure, logger *logrus.Entry, k8sGardenClient kubernetes.Interface, k8sGardenInformers gardeninformers.Interface, gardenerInfo *gardenv1beta1.Gardener, secretsMap map[string]*corev1.Secret, imageVector imagevector.ImageVector) (*Operation, error) {
	return newOperation(logger, k8sGardenClient, k8sGardenInformers, gardenerInfo, secretsMap, imageVector, backupInfrastructure.Namespace, backupInfrastructure.Spec.Seed, nil, backupInfrastructure, nil)
}

func newOperation(
	logger *logrus.Entry,
	k8sGardenClient kubernetes.Interface,
	k8sGardenInformers gardeninformers.Interface,
	gardenerInfo *gardenv1beta1.Gardener,
	secretsMap map[string]*corev1.Secret,
	imageVector imagevector.ImageVector,
	namespace,
	seedName string,
	shoot *gardenv1beta1.Shoot,
	backupInfrastructure *gardenv1beta1.BackupInfrastructure,
	shootBackup *config.ShootBackup,
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

	chartRenderer, err := chartrenderer.New(k8sGardenClient.Kubernetes())
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
		ShootBackup:          shootBackup,
		MachineDeployments:   MachineDeployments{},
	}

	if shoot != nil {
		internalDomain := constructInternalDomain(shoot.Name, gardenObj.Project.Name, secretsMap[common.GardenRoleInternalDomain].Annotations[common.DNSDomain])
		shootObj, err := shootpkg.New(k8sGardenClient, k8sGardenInformers, shoot, gardenObj.Project.Name, internalDomain)
		if err != nil {
			return nil, err
		}
		operation.Shoot = shootObj
		operation.Shoot.WantsAlertmanager = helper.ShootWantsAlertmanager(shoot, secrets)

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

	k8sSeedClient, err := kubernetes.NewClientFromSecretObject(o.Seed.Secret, client.Options{
		Scheme: kubernetes.SeedScheme,
	})
	if err != nil {
		return err
	}

	o.K8sSeedClient = k8sSeedClient
	o.ChartSeedRenderer, err = chartrenderer.New(k8sSeedClient.Kubernetes())
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

	k8sShootClient, err := kubernetes.NewClientFromSecret(o.K8sSeedClient, o.Shoot.SeedNamespace, gardenv1beta1.GardenerName, client.Options{
		Scheme: kubernetes.ShootScheme,
	})
	if err != nil {
		return err
	}

	o.K8sShootClient = k8sShootClient
	o.ChartShootRenderer, err = chartrenderer.New(k8sShootClient.Kubernetes())
	if err != nil {
		return err
	}
	return nil
}

//ComputePrometheusIngressFQDN computes full qualified domain name for prometheus ingress sub-resource and returns it.
func (o *Operation) ComputePrometheusIngressFQDN() string {
	return o.Seed.GetIngressFQDN("p", o.Shoot.Info.Name, o.Garden.Project.Name)
}

// InitializeMonitoringClient will read the Prometheus ingress auth and tls
// secrets from the Seed cluster, which are containing the cert to secure
// the connection and the credentials authenticate against the Shoot Prometheus.
// With those certs and credentials, a Prometheus client API will be created
// and attached to the existing Operation object.
func (o *Operation) InitializeMonitoringClient() error {
	if o.MonitoringClient != nil {
		return nil
	}

	// Read the CA.
	tlsSecret, err := o.K8sSeedClient.GetSecret(o.Shoot.SeedNamespace, "prometheus-tls")
	if err != nil {
		return err
	}

	ca := x509.NewCertPool()
	ca.AppendCertsFromPEM(tlsSecret.Data[secrets.DataKeyCertificateCA])

	// Read the basic auth credentials.
	credentials, err := o.K8sSeedClient.GetSecret(o.Shoot.SeedNamespace, "monitoring-ingress-credentials")
	if err != nil {
		return err
	}

	config := prometheusapi.Config{
		Address: fmt.Sprintf("https://%s", o.ComputePrometheusIngressFQDN()),
		RoundTripper: &prometheusRoundTripper{
			authHeader: fmt.Sprintf("Basic %s", utils.EncodeBase64([]byte(fmt.Sprintf("%s:%s", credentials.Data[secrets.DataKeyUserName], credentials.Data[secrets.DataKeyPassword])))),
			ca:         ca,
		},
	}
	client, err := prometheusapi.NewClient(config)
	if err != nil {
		return err
	}
	o.MonitoringClient = prometheusclient.NewAPI(client)
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

	newShoot, err := kutil.TryUpdateShootStatus(o.K8sGardenClient.Garden(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
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

	if newBackupInfrastructure, err := o.K8sGardenClient.Garden().GardenV1beta1().BackupInfrastructures(o.BackupInfrastructure.Namespace).UpdateStatus(o.BackupInfrastructure); err == nil {
		o.BackupInfrastructure = newBackupInfrastructure
	}
}

// SeedVersion is a shorthand for the kubernetes version of the K8sSeedClient.
func (o *Operation) SeedVersion() string {
	return o.K8sSeedClient.Version()
}

// ShootVersion is a shorthand for the desired kubernetes version of the operation's shoot.
func (o *Operation) ShootVersion() string {
	return o.Shoot.Info.Spec.Kubernetes.Version
}

func (o *Operation) newTerraformer(purpose, namespace, name string) (*terraformer.Terraformer, error) {
	image, err := o.ImageVector.FindImage(common.TerraformerImageName, o.K8sSeedClient.Version(), o.K8sSeedClient.Version())
	if err != nil {
		return nil, err
	}

	return terraformer.NewForConfig(o.Logger, o.K8sSeedClient.RESTConfig(), purpose, namespace, name, image.String())
}

// NewBackupInfrastructureTerraformer creates a new Terraformer for the matching BackupInfrastructure.
func (o *Operation) NewBackupInfrastructureTerraformer() (*terraformer.Terraformer, error) {
	var backupInfrastructureName string
	if o.Shoot != nil {
		backupInfrastructureName = common.GenerateBackupInfrastructureName(o.Shoot.SeedNamespace, o.Shoot.Info.Status.UID)
	} else {
		backupInfrastructureName = o.BackupInfrastructure.Name
	}

	return o.newTerraformer(common.TerraformerPurposeBackup, common.GenerateBackupNamespaceName(backupInfrastructureName), backupInfrastructureName)
}

// NewShootTerraformer creates a new Terraformer for the current shoot with the given purpose.
func (o *Operation) NewShootTerraformer(purpose string) (*terraformer.Terraformer, error) {
	return o.newTerraformer(purpose, o.Shoot.SeedNamespace, o.Shoot.Info.Name)
}

// ChartInitializer initializes a terraformer based on the given chart and values.
func (o *Operation) ChartInitializer(chartName string, values map[string]interface{}) terraformer.Initializer {
	return func(config *terraformer.InitializerConfig) error {
		chartRenderer, err := chartrenderer.New(o.K8sSeedClient.Kubernetes())
		if err != nil {
			return err
		}

		values["names"] = map[string]interface{}{
			"configuration": config.ConfigurationName,
			"variables":     config.VariablesName,
			"state":         config.StateName,
		}
		values["initializeEmptyState"] = config.InitializeState

		return utils.Retry(5*time.Second, 30*time.Second, func() (bool, bool, error) {
			if err := common.ApplyChart(o.K8sSeedClient, chartRenderer, filepath.Join(common.TerraformerChartPath, chartName), chartName, config.Namespace, nil, values); err != nil {
				return false, false, nil
			}
			return true, false, nil
		})
	}
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
