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
	"context"
	"crypto/x509"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
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
	"github.com/gardener/gardener/pkg/operation/terraformer"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/chart"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	utilretry "github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/secrets"

	prometheusapi "github.com/prometheus/client_golang/api"
	prometheusclient "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// New creates a new operation object with a Shoot resource object.
func New(shoot *gardenv1beta1.Shoot, config *config.ControllerManagerConfiguration, logger *logrus.Entry, k8sGardenClient kubernetes.Interface, k8sGardenInformers gardeninformers.Interface, gardenerInfo *gardenv1beta1.Gardener, secretsMap map[string]*corev1.Secret, imageVector imagevector.ImageVector, shootBackup *config.ShootBackup) (*Operation, error) {
	return newOperation(config, logger, k8sGardenClient, k8sGardenInformers, gardenerInfo, secretsMap, imageVector, shoot.Namespace, shoot.Spec.Cloud.Seed, shoot, nil, shootBackup)
}

// NewWithBackupInfrastructure creates a new operation object without a Shoot resource object but the BackupInfrastructure resource.
func NewWithBackupInfrastructure(backupInfrastructure *gardenv1beta1.BackupInfrastructure, config *config.ControllerManagerConfiguration, logger *logrus.Entry, k8sGardenClient kubernetes.Interface, k8sGardenInformers gardeninformers.Interface, gardenerInfo *gardenv1beta1.Gardener, secretsMap map[string]*corev1.Secret, imageVector imagevector.ImageVector) (*Operation, error) {
	return newOperation(config, logger, k8sGardenClient, k8sGardenInformers, gardenerInfo, secretsMap, imageVector, backupInfrastructure.Namespace, &backupInfrastructure.Spec.Seed, nil, backupInfrastructure, nil)
}

func newOperation(
	config *config.ControllerManagerConfiguration,
	logger *logrus.Entry,
	k8sGardenClient kubernetes.Interface,
	k8sGardenInformers gardeninformers.Interface,
	gardenerInfo *gardenv1beta1.Gardener,
	secretsMap map[string]*corev1.Secret,
	imageVector imagevector.ImageVector,
	namespace string,
	seedName *string,
	shoot *gardenv1beta1.Shoot,
	backupInfrastructure *gardenv1beta1.BackupInfrastructure,
	shootBackup *config.ShootBackup,
) (*Operation, error) {

	secrets := make(map[string]*corev1.Secret)
	for k, v := range secretsMap {
		secrets[k] = v
	}

	gardenObj, err := garden.New(k8sGardenInformers.Projects().Lister(), namespace, secrets)
	if err != nil {
		return nil, err
	}

	var seedObj *seed.Seed
	if seedName != nil {
		seedObj, err = seed.NewFromName(k8sGardenClient, k8sGardenInformers, *seedName)
		if err != nil {
			return nil, err
		}
	}

	renderer, err := chartrenderer.NewForConfig(k8sGardenClient.RESTConfig())
	if err != nil {
		return nil, err
	}
	applier, err := kubernetes.NewApplierForConfig(k8sGardenClient.RESTConfig())
	if err != nil {
		return nil, err
	}

	operation := &Operation{
		Config:               config,
		Logger:               logger,
		GardenerInfo:         gardenerInfo,
		Secrets:              secrets,
		ImageVector:          imageVector,
		CheckSums:            make(map[string]string),
		Garden:               gardenObj,
		Seed:                 seedObj,
		K8sGardenClient:      k8sGardenClient,
		K8sGardenInformers:   k8sGardenInformers,
		ChartApplierGarden:   kubernetes.NewChartApplier(renderer, applier),
		BackupInfrastructure: backupInfrastructure,
		ShootBackup:          shootBackup,
	}

	if shoot != nil {
		shootObj, err := shootpkg.New(k8sGardenClient, k8sGardenInformers, shoot, gardenObj.Project.Name, gardenObj.InternalDomain.Domain, gardenObj.DefaultDomains)
		if err != nil {
			return nil, err
		}
		operation.Shoot = shootObj
		operation.Shoot.IgnoreAlerts = helper.ShootIgnoreAlerts(shoot)
		operation.Shoot.WantsAlertmanager = helper.ShootWantsAlertmanager(shoot, secrets) && !operation.Shoot.IgnoreAlerts

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
	if o.K8sSeedClient != nil && o.ChartApplierSeed != nil {
		return nil
	}

	k8sSeedClient, err := kubernetes.NewClientFromSecretObject(o.Seed.Secret,
		kubernetes.WithClientConnectionOptions(o.Config.ClientConnection),
		kubernetes.WithClientOptions(
			client.Options{
				Scheme: kubernetes.SeedScheme,
			}),
	)
	if err != nil {
		return err
	}

	o.K8sSeedClient = k8sSeedClient

	renderer, err := chartrenderer.NewForConfig(k8sSeedClient.RESTConfig())
	if err != nil {
		return err
	}
	applier, err := kubernetes.NewApplierForConfig(k8sSeedClient.RESTConfig())
	if err != nil {
		return err
	}

	o.ChartApplierSeed = kubernetes.NewChartApplier(renderer, applier)
	return nil
}

// InitializeShootClients will use the Seed Kubernetes client to read the gardener Secret in the Seed
// cluster which contains a Kubeconfig that can be used to authenticate against the Shoot cluster. With it,
// a Kubernetes client as well as a Chart renderer for the Shoot cluster will be initialized and attached to
// the already existing Operation object.
func (o *Operation) InitializeShootClients() error {
	if o.K8sShootClient != nil && o.ChartApplierShoot != nil {
		return nil
	}

	if o.Shoot.HibernationEnabled {
		controlPlaneHibernated, err := o.controlPlaneHibernated()
		if err != nil {
			return err
		}
		// Do not initialize Shoot clients for already hibernated shoots.
		if controlPlaneHibernated {
			return nil
		}
	}

	k8sShootClient, err := kubernetes.NewClientFromSecret(o.K8sSeedClient, o.Shoot.SeedNamespace, gardenv1beta1.GardenerName,
		kubernetes.WithClientConnectionOptions(o.Config.ClientConnection),
		kubernetes.WithClientOptions(
			client.Options{
				Scheme: kubernetes.ShootScheme,
			}),
	)
	if err != nil {
		return err
	}
	o.K8sShootClient = k8sShootClient

	renderer, err := chartrenderer.NewForConfig(k8sShootClient.RESTConfig())
	if err != nil {
		return err
	}
	applier, err := kubernetes.NewApplierForConfig(k8sShootClient.RESTConfig())
	if err != nil {
		return err
	}

	o.ChartApplierShoot = kubernetes.NewChartApplier(renderer, applier)
	return nil
}

func (o *Operation) controlPlaneHibernated() (bool, error) {
	replicaCount, err := common.CurrentReplicaCount(o.K8sSeedClient.Client(), o.Shoot.SeedNamespace, gardencorev1alpha1.DeploymentNameKubeAPIServer)
	if err != nil {
		return false, err
	}
	if replicaCount > 0 {
		return false, nil
	}
	return true, nil
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
	tlsSecret := &corev1.Secret{}
	if err := o.K8sSeedClient.Client().Get(context.TODO(), kutil.Key(o.Shoot.SeedNamespace, "prometheus-tls"), tlsSecret); err != nil {
		return err
	}

	ca := x509.NewCertPool()
	ca.AppendCertsFromPEM(tlsSecret.Data[secrets.DataKeyCertificateCA])

	// Read the basic auth credentials.
	credentials := &corev1.Secret{}
	if err := o.K8sSeedClient.Client().Get(context.TODO(), kutil.Key(o.Shoot.SeedNamespace, "monitoring-ingress-credentials"), credentials); err != nil {
		return err
	}

	config := prometheusapi.Config{
		Address: fmt.Sprintf("https://%s", o.ComputeIngressHost("p")),
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
func (o *Operation) ApplyChartGarden(chartPath, namespace, name string, defaultValues, additionalValues map[string]interface{}) error {
	return o.ChartApplierGarden.ApplyChart(context.TODO(), chartPath, namespace, name, defaultValues, additionalValues)
}

// ApplyChartSeed takes a path to a chart <chartPath>, name of the release <name>, release's namespace <namespace>
// and two maps <defaultValues>, <additionalValues>, and renders the template based on the merged result of both value maps.
// The resulting manifest will be applied to the Seed cluster.
func (o *Operation) ApplyChartSeed(chartPath, namespace, name string, defaultValues, additionalValues map[string]interface{}) error {
	return o.ChartApplierSeed.ApplyChart(context.TODO(), chartPath, namespace, name, defaultValues, additionalValues)
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
func (o *Operation) ReportShootProgress(ctx context.Context, stats *flow.Stats) {
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
func (o *Operation) ReportBackupInfrastructureProgress(ctx context.Context, stats *flow.Stats) {
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

func (o *Operation) injectImages(values map[string]interface{}, names []string, opts ...imagevector.FindOptionFunc) (map[string]interface{}, error) {
	return chart.InjectImages(values, o.ImageVector, names, opts...)
}

// InjectSeedSeedImages injects images that shall run on the Seed and target the Seed's Kubernetes version.
func (o *Operation) InjectSeedSeedImages(values map[string]interface{}, names ...string) (map[string]interface{}, error) {
	return o.injectImages(values, names, imagevector.RuntimeVersion(o.SeedVersion()), imagevector.TargetVersion(o.SeedVersion()))
}

// InjectSeedShootImages injects images that shall run on the Seed but target the Shoot's Kubernetes version.
func (o *Operation) InjectSeedShootImages(values map[string]interface{}, names ...string) (map[string]interface{}, error) {
	return o.injectImages(values, names, imagevector.RuntimeVersion(o.SeedVersion()), imagevector.TargetVersion(o.ShootVersion()))
}

// InjectShootShootImages injects images that shall run on the Shoot and target the Shoot's Kubernetes version.
func (o *Operation) InjectShootShootImages(values map[string]interface{}, names ...string) (map[string]interface{}, error) {
	return o.injectImages(values, names, imagevector.RuntimeVersion(o.ShootVersion()), imagevector.TargetVersion(o.ShootVersion()))
}

func (o *Operation) newTerraformer(purpose, namespace, name string) (*terraformer.Terraformer, error) {
	image, err := o.ImageVector.FindImage(common.TerraformerImageName, imagevector.RuntimeVersion(o.K8sSeedClient.Version()), imagevector.TargetVersion(o.K8sSeedClient.Version()))
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
		chartRenderer, err := chartrenderer.NewForConfig(o.K8sSeedClient.RESTConfig())
		if err != nil {
			return err
		}
		applier, err := kubernetes.NewApplierForConfig(o.K8sSeedClient.RESTConfig())
		if err != nil {
			return err
		}
		chartApplier := kubernetes.NewChartApplier(chartRenderer, applier)

		values["names"] = map[string]interface{}{
			"configuration": config.ConfigurationName,
			"variables":     config.VariablesName,
			"state":         config.StateName,
		}
		values["initializeEmptyState"] = config.InitializeState

		return utilretry.UntilTimeout(context.TODO(), 5*time.Second, 30*time.Second, func(ctx context.Context) (done bool, err error) {
			if err := chartApplier.ApplyChart(ctx, filepath.Join(common.TerraformerChartPath, chartName), config.Namespace, chartName, nil, values); err != nil {
				return utilretry.MinorError(err)
			}
			return utilretry.Ok()
		})
	}
}

// SyncClusterResourceToSeed creates or updates the `Cluster` extension resource for the shoot in the seed cluster.
// It contains the shoot, seed, and cloudprofile specification.
func (o *Operation) SyncClusterResourceToSeed(ctx context.Context) error {
	if err := o.InitializeSeedClients(); err != nil {
		o.Logger.Errorf("Could not initialize a new Kubernetes client for the seed cluster: %s", err.Error())
		return err
	}

	var (
		cluster = &extensionsv1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: o.Shoot.SeedNamespace,
			},
		}

		cloudProfileObj = o.Shoot.CloudProfile.DeepCopy()
		seedObj         = o.Seed.Info.DeepCopy()
		shootObj        = o.Shoot.Info.DeepCopy()
	)

	cloudProfileObj.TypeMeta = metav1.TypeMeta{
		APIVersion: gardenv1beta1.SchemeGroupVersion.String(),
		Kind:       "CloudProfile",
	}
	seedObj.TypeMeta = metav1.TypeMeta{
		APIVersion: gardenv1beta1.SchemeGroupVersion.String(),
		Kind:       "Seed",
	}
	shootObj.TypeMeta = metav1.TypeMeta{
		APIVersion: gardenv1beta1.SchemeGroupVersion.String(),
		Kind:       "Shoot",
	}

	return kutil.CreateOrUpdate(ctx, o.K8sSeedClient.Client(), cluster, func() error {
		cluster.Spec.CloudProfile = runtime.RawExtension{Object: cloudProfileObj}
		cluster.Spec.Seed = runtime.RawExtension{Object: seedObj}
		cluster.Spec.Shoot = runtime.RawExtension{Object: shootObj}
		return nil
	})
}

// DeleteClusterResourceFromSeed deletes the `Cluster` extension resource for the shoot in the seed cluster.
func (o *Operation) DeleteClusterResourceFromSeed(ctx context.Context) error {
	if err := o.InitializeSeedClients(); err != nil {
		o.Logger.Errorf("Could not initialize a new Kubernetes client for the seed cluster: %s", err.Error())
		return err
	}

	if err := o.K8sSeedClient.Client().Delete(ctx, &extensionsv1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: o.Shoot.SeedNamespace}}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

// ComputeGrafanaHosts computes the host for both grafanas.
func (o *Operation) ComputeGrafanaHosts() []string {
	return []string{
		o.ComputeGrafanaOperatorsHost(),
		o.ComputeGrafanaUsersHost(),
	}
}

// ComputeGrafanaOperatorsHost computes the host for users Grafana.
func (o *Operation) ComputeGrafanaOperatorsHost() string {
	return o.ComputeIngressHost(common.GrafanaOperatorsPrefix)
}

// ComputeGrafanaUsersHost computes the host for operators Grafana.
func (o *Operation) ComputeGrafanaUsersHost() string {
	return o.ComputeIngressHost(common.GrafanaUsersPrefix)
}

// ComputeAlertManagerHost computes the host for alert manager.
func (o *Operation) ComputeAlertManagerHost() string {
	return o.ComputeIngressHost("a")
}

// ComputePrometheusHost computes the host for prometheus.
func (o *Operation) ComputePrometheusHost() string {
	return o.ComputeIngressHost("p")
}

// ComputeKibanaHost computes the host for kibana.
func (o *Operation) ComputeKibanaHost() string {
	return o.ComputeIngressHost("k")
}

// ComputeIngressHost computes the host for a given prefix.
func (o *Operation) ComputeIngressHost(prefix string) string {
	return o.Seed.GetIngressFQDN(prefix, o.Shoot.Info.Name, o.Garden.Project.Name)
}
