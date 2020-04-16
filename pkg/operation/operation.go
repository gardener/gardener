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
	"net"
	"strings"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/etcdencryption"
	"github.com/gardener/gardener/pkg/operation/garden"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/operation/shoot"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/chart"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// New creates a new operation object with a Shoot resource object.
func New(shoot *gardencorev1beta1.Shoot, config *config.GardenletConfiguration, logger *logrus.Entry, k8sGardenClient kubernetes.Interface, k8sGardenCoreInformers gardencoreinformers.Interface, gardenerInfo *gardencorev1beta1.Gardener, secretsMap map[string]*corev1.Secret, imageVector imagevector.ImageVector) (*Operation, error) {
	return newOperation(config, logger, k8sGardenClient, k8sGardenCoreInformers, gardenerInfo, secretsMap, imageVector, shoot.Namespace, shoot.Spec.SeedName, shoot)
}

func newOperation(
	config *config.GardenletConfiguration,
	logger *logrus.Entry,
	k8sGardenClient kubernetes.Interface,
	k8sGardenCoreInformers gardencoreinformers.Interface,
	gardenerInfo *gardencorev1beta1.Gardener,
	secretsMap map[string]*corev1.Secret,
	imageVector imagevector.ImageVector,
	namespace string,
	seedName *string,
	shoot *gardencorev1beta1.Shoot,
) (*Operation, error) {

	secrets := make(map[string]*corev1.Secret)
	for k, v := range secretsMap {
		secrets[k] = v
	}

	gardenObj, err := garden.New(k8sGardenCoreInformers.Projects().Lister(), namespace, secrets)
	if err != nil {
		return nil, err
	}

	var (
		seedObj    *seed.Seed
		disableDNS bool
	)
	if seedName != nil {
		seedObj, err = seed.NewFromName(k8sGardenClient, k8sGardenCoreInformers, *seedName)
		if err != nil {
			return nil, err
		}
		disableDNS = gardencorev1beta1helper.TaintsHave(seedObj.Info.Spec.Taints, gardencorev1beta1.SeedTaintDisableDNS)
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
		Config:                 config,
		Logger:                 logger,
		GardenerInfo:           gardenerInfo,
		Secrets:                secrets,
		ImageVector:            imageVector,
		CheckSums:              make(map[string]string),
		Garden:                 gardenObj,
		Seed:                   seedObj,
		K8sGardenClient:        k8sGardenClient,
		K8sGardenCoreInformers: k8sGardenCoreInformers,
		ChartApplierGarden:     kubernetes.NewChartApplier(renderer, applier),
	}

	if shoot != nil {
		shootObj, err := shootpkg.New(k8sGardenClient, k8sGardenCoreInformers, shoot, gardenObj.Project.Name, disableDNS, gardenObj.InternalDomain, gardenObj.DefaultDomains)
		if err != nil {
			return nil, err
		}
		operation.Shoot = shootObj
		operation.Shoot.IgnoreAlerts = gardencorev1beta1helper.ShootIgnoresAlerts(shoot)
		operation.Shoot.WantsAlertmanager = shootWantsAlertmanager(shoot, secrets) && !operation.Shoot.IgnoreAlerts

		shootedSeed, err := gardencorev1beta1helper.ReadShootedSeed(shoot)
		if err != nil {
			logger.Warnf("Cannot use shoot %s/%s as shooted seed: %+v", shoot.Namespace, shoot.Name, err)
		} else {
			operation.ShootedSeed = shootedSeed
		}
	}

	return operation, nil
}

func shootWantsAlertmanager(shoot *gardencorev1beta1.Shoot, secrets map[string]*corev1.Secret) bool {
	if shoot.Spec.Monitoring != nil && shoot.Spec.Monitoring.Alerting != nil && len(shoot.Spec.Monitoring.Alerting.EmailReceivers) > 0 {
		return true
	}
	return false
}

// InitializeSeedClients will use the Garden Kubernetes client to read the Seed Secret in the Garden
// cluster which contains a Kubeconfig that can be used to authenticate against the Seed cluster. With it,
// a Kubernetes client as well as a Chart renderer for the Seed cluster will be initialized and attached to
// the already existing Operation object.
func (o *Operation) InitializeSeedClients() error {
	if o.K8sSeedClient != nil && o.ChartApplierSeed != nil {
		return nil
	}

	k8sSeedClient, err := seed.GetSeedClient(context.TODO(), o.K8sGardenClient.Client(), o.Config.SeedClientConnection.ClientConnectionConfiguration, o.Config.SeedSelector == nil, o.Seed.Info.Name)
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

	secretName := v1beta1constants.SecretNameGardener
	// If the gardenlet runs in the same cluster like the API server of the shoot then use the internal kubeconfig
	// and communicate internally. Otherwise, fall back to the "external" kubeconfig and communicate via the
	// load balancer of the shoot API server.
	addr, err := net.LookupHost(o.Shoot.ComputeInClusterAPIServerAddress(false))
	if err != nil {
		o.Logger.Warnf("service DNS name lookup of kube-apiserver failed (%+v), falling back to external kubeconfig", err)
	} else if len(addr) > 0 {
		secretName = v1beta1constants.SecretNameGardenerInternal
	}

	k8sShootClient, err := kubernetes.NewClientFromSecret(o.K8sSeedClient, o.Shoot.SeedNamespace, secretName,
		kubernetes.WithClientConnectionOptions(o.Config.ShootClientConnection.ClientConnectionConfiguration),
		kubernetes.WithClientOptions(client.Options{
			Scheme: kubernetes.ShootScheme,
		}),
	)
	// TODO: This if-condition can be removed in a future version when all shoots were reconciled with Gardener v1.1 version.
	if secretName == v1beta1constants.SecretNameGardenerInternal && err != nil && apierrors.IsNotFound(err) {
		k8sShootClient, err = kubernetes.NewClientFromSecret(o.K8sSeedClient, o.Shoot.SeedNamespace, v1beta1constants.SecretNameGardener,
			kubernetes.WithClientConnectionOptions(o.Config.ShootClientConnection.ClientConnectionConfiguration),
			kubernetes.WithClientOptions(client.Options{
				Scheme: kubernetes.ShootScheme,
			}),
		)
	}
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
	replicaCount, err := common.CurrentReplicaCount(o.K8sSeedClient.Client(), o.Shoot.SeedNamespace, v1beta1constants.DeploymentNameKubeAPIServer)
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
	if err := o.K8sSeedClient.Client().Get(context.TODO(), kutil.Key(o.Shoot.SeedNamespace, common.PrometheusTLS), tlsSecret); err != nil {
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

	newShoot, err := kutil.TryUpdateShootStatus(o.K8sGardenClient.GardenCore(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
		func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
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

// CleanShootTaskError removes the error with taskID from the Shoot's status.LastErrors array.
// If the status.LastErrors array is empty then status.LastError is also removed.
func (o *Operation) CleanShootTaskError(ctx context.Context, taskID string) {
	var remainingErrors []gardencorev1beta1.LastError
	for _, lastErr := range o.Shoot.Info.Status.LastErrors {
		if lastErr.TaskID == nil || taskID != *lastErr.TaskID {
			remainingErrors = append(remainingErrors, lastErr)
		}
	}

	newShoot, err := kutil.TryUpdateShootStatus(o.K8sGardenClient.GardenCore(), retry.DefaultRetry, o.Shoot.Info.ObjectMeta,
		func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			shoot.Status.LastErrors = remainingErrors
			return shoot, nil
		})
	if err != nil {
		o.Logger.Errorf("Could not report shoot progress: %v", err)
		return
	}
	o.Shoot.Info = newShoot
}

// SeedVersion is a shorthand for the kubernetes version of the K8sSeedClient.
func (o *Operation) SeedVersion() string {
	return o.K8sSeedClient.Version()
}

// ShootVersion is a shorthand for the desired kubernetes version of the operation's shoot.
func (o *Operation) ShootVersion() string {
	return o.Shoot.Info.Spec.Kubernetes.Version
}

// InjectSeedSeedImages injects images that shall run on the Seed and target the Seed's Kubernetes version.
func (o *Operation) InjectSeedSeedImages(values map[string]interface{}, names ...string) (map[string]interface{}, error) {
	return chart.InjectImages(values, o.ImageVector, names, imagevector.RuntimeVersion(o.SeedVersion()), imagevector.TargetVersion(o.SeedVersion()))
}

// InjectSeedShootImages injects images that shall run on the Seed but target the Shoot's Kubernetes version.
func (o *Operation) InjectSeedShootImages(values map[string]interface{}, names ...string) (map[string]interface{}, error) {
	return chart.InjectImages(values, o.ImageVector, names, imagevector.RuntimeVersion(o.SeedVersion()), imagevector.TargetVersion(o.ShootVersion()))
}

// InjectShootShootImages injects images that shall run on the Shoot and target the Shoot's Kubernetes version.
func (o *Operation) InjectShootShootImages(values map[string]interface{}, names ...string) (map[string]interface{}, error) {
	return chart.InjectImages(values, o.ImageVector, names, imagevector.RuntimeVersion(o.ShootVersion()), imagevector.TargetVersion(o.ShootVersion()))
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
		APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
		Kind:       "CloudProfile",
	}
	seedObj.TypeMeta = metav1.TypeMeta{
		APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
		Kind:       "Seed",
	}
	shootObj.TypeMeta = metav1.TypeMeta{
		APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
		Kind:       "Shoot",
	}

	_, err := controllerutil.CreateOrUpdate(ctx, o.K8sSeedClient.Client(), cluster, func() error {
		cluster.Spec.CloudProfile = runtime.RawExtension{Object: cloudProfileObj}
		cluster.Spec.Seed = runtime.RawExtension{Object: seedObj}
		cluster.Spec.Shoot = runtime.RawExtension{Object: shootObj}
		return nil
	})
	return err
}

// EnsureShootStateExists creates the ShootState resource for the corresponding shoot and sets its ownerReferences to the Shoot.
func (o *Operation) EnsureShootStateExists(ctx context.Context) error {
	shootState := &gardencorev1alpha1.ShootState{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Shoot.Info.Name,
			Namespace: o.Shoot.Info.Namespace,
		},
	}
	ownerReference := metav1.NewControllerRef(o.Shoot.Info, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot"))
	blockOwnerDeletion := false
	ownerReference.BlockOwnerDeletion = &blockOwnerDeletion

	_, err := controllerutil.CreateOrUpdate(ctx, o.K8sGardenClient.Client(), shootState, func() error {
		shootState.OwnerReferences = []metav1.OwnerReference{*ownerReference}
		return nil
	})
	if err != nil {
		return err
	}

	o.ShootState = shootState
	gardenerResourceList := gardencorev1alpha1helper.GardenerResourceDataList(shootState.Spec.Gardener)
	o.Shoot.ETCDEncryption, err = etcdencryption.GetEncryptionConfig(gardenerResourceList)
	return err
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
		o.ComputeGrafanaOperatorsHostDeprecated(),
		o.ComputeGrafanaUsersHostDeprecated(),
		o.ComputeGrafanaOperatorsHost(),
		o.ComputeGrafanaUsersHost(),
	}
}

// ComputeKibanaHosts computes the hosts for kibana.
func (o *Operation) ComputeKibanaHosts() []string {
	return []string{
		o.ComputeKibanaHostDeprecated(),
		o.ComputeKibanaHost(),
	}
}

// ComputePrometheusHosts computes the hosts for prometheus.
func (o *Operation) ComputePrometheusHosts() []string {
	return []string{
		o.ComputePrometheusHostDeprecated(),
		o.ComputePrometheusHost(),
	}
}

// ComputeAlertManagerHosts computes the host for alert manager.
func (o *Operation) ComputeAlertManagerHosts() []string {
	return []string{
		o.ComputeAlertManagerHostDeprecated(),
		o.ComputeAlertManagerHost(),
	}
}

// ComputeGrafanaOperatorsHostDeprecated computes the host for users Grafana.
// TODO: timuthy - remove in the future. Old Grafana host is retained for migration reasons.
func (o *Operation) ComputeGrafanaOperatorsHostDeprecated() string {
	return o.ComputeIngressHostDeprecated(common.GrafanaOperatorsPrefix)
}

// ComputeGrafanaUsersHostDeprecated computes the host for operators Grafana.
// TODO: timuthy - remove in the future. Old Grafana host is retained for migration reasons.
func (o *Operation) ComputeGrafanaUsersHostDeprecated() string {
	return o.ComputeIngressHostDeprecated(common.GrafanaUsersPrefix)
}

// ComputeGrafanaOperatorsHost computes the host for users Grafana.
func (o *Operation) ComputeGrafanaOperatorsHost() string {
	return o.ComputeIngressHost(common.GrafanaOperatorsPrefix)
}

// ComputeGrafanaUsersHost computes the host for operators Grafana.
func (o *Operation) ComputeGrafanaUsersHost() string {
	return o.ComputeIngressHost(common.GrafanaUsersPrefix)
}

// ComputeAlertManagerHostDeprecated computes the host for alert manager.
// TODO: timuthy - remove in the future. Old AlertManager host is retained for migration reasons.
func (o *Operation) ComputeAlertManagerHostDeprecated() string {
	return o.ComputeIngressHostDeprecated(common.AlertManagerPrefix)
}

// ComputeAlertManagerHost computes the host for alert manager.
func (o *Operation) ComputeAlertManagerHost() string {
	return o.ComputeIngressHost(common.AlertManagerPrefix)
}

// ComputePrometheusHostDeprecated computes the host for prometheus.
// TODO: timuthy - remove in the future. Old Prometheus host is retained for migration reasons.
func (o *Operation) ComputePrometheusHostDeprecated() string {
	return o.ComputeIngressHostDeprecated(common.PrometheusPrefix)
}

// ComputePrometheusHost computes the host for prometheus.
func (o *Operation) ComputePrometheusHost() string {
	return o.ComputeIngressHost(common.PrometheusPrefix)
}

// ComputeKibanaHostDeprecated computes the host for kibana.
// TODO: timuthy - remove in the future. Old Kibana host is retained for migration reasons.
func (o *Operation) ComputeKibanaHostDeprecated() string {
	return o.ComputeIngressHostDeprecated(common.KibanaPrefix)
}

// ComputeKibanaHost computes the host for kibana.
func (o *Operation) ComputeKibanaHost() string {
	return o.ComputeIngressHost(common.KibanaPrefix)
}

// ComputeIngressHostDeprecated computes the host for a given prefix.
// TODO: timuthy - remove in the future. Only retained for migration reasons.
func (o *Operation) ComputeIngressHostDeprecated(prefix string) string {
	return o.Seed.GetIngressFQDNDeprecated(prefix, o.Shoot.Info.Name, o.Garden.Project.Name)
}

// ComputeIngressHost computes the host for a given prefix.
func (o *Operation) ComputeIngressHost(prefix string) string {
	shortID := strings.Replace(o.Shoot.Info.Status.TechnicalID, shoot.TechnicalIDPrefix, "", 1)
	return fmt.Sprintf("%s-%s.%s", prefix, shortID, o.Seed.Info.Spec.DNS.IngressDomain)
}
