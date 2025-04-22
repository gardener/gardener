// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpa

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	vpaconstants "github.com/gardener/gardener/pkg/component/autoscaling/vpa/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/seed"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/shoot"
	monitoringutils "github.com/gardener/gardener/pkg/component/observability/monitoring/utils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// ManagedResourceControlName is the name of the vpa managed resource for seeds.
	ManagedResourceControlName = "vpa"
	shootManagedResourceName   = "shoot-core-" + ManagedResourceControlName
)

// Interface contains functions for a VPA deployer.
type Interface interface {
	component.DeployWaiter

	// GetValues returns the current configuration values of the deployer.
	GetValues() Values
}

// New creates a new instance of DeployWaiter for the Kubernetes Vertical Pod Autoscaler.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) Interface {
	return &vpa{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type vpa struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values

	caSecretName                     string
	caBundle                         []byte
	serverSecretName                 string
	genericTokenKubeconfigSecretName *string
}

// Values is a set of configuration values for the VPA components.
type Values struct {
	// ClusterType specifies the type of the cluster to which VPA is being deployed.
	// For seeds, all resources are being deployed as part of a ManagedResource (except for the CRDs - those must be
	// deployed separately because the VPA components themselves create VPA resources, hence the CRD must exist
	// beforehand).
	// For shoots, the VPA runs in the shoot namespace in the seed as part of the control plane. Hence, only the runtime
	// resources (like Deployment, Service, etc.) are being deployed directly (with the client). All other application-
	// related resources (like RBAC roles, CRD, etc.) are deployed as part of a ManagedResource.
	ClusterType component.ClusterType
	// IsGardenCluster specifies if the VPA is being deployed in a cluster registered as a Garden.
	IsGardenCluster bool
	// Enabled specifies if VPA is enabled.
	Enabled bool
	// SecretNameServerCA is the name of the server CA secret.
	SecretNameServerCA string
	// RuntimeKubernetesVersion is the Kubernetes version of the runtime cluster.
	RuntimeKubernetesVersion *semver.Version

	// AdmissionController is a set of configuration values for the vpa-admission-controller.
	AdmissionController ValuesAdmissionController
	// Recommender is a set of configuration values for the vpa-recommender.
	Recommender ValuesRecommender
	// Updater is a set of configuration values for the vpa-updater.
	Updater ValuesUpdater
}

func (v *vpa) Deploy(ctx context.Context) error {
	caSecret, found := v.secretsManager.Get(v.values.SecretNameServerCA)
	if !found {
		return fmt.Errorf("secret %q not found", v.values.SecretNameServerCA)
	}
	v.caSecretName = caSecret.Name
	v.caBundle = caSecret.Data[secretsutils.DataKeyCertificateBundle]

	serverSecret, err := v.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                        "vpa-admission-controller-server",
		CommonName:                  fmt.Sprintf("%s.%s.svc", vpaconstants.AdmissionControllerServiceName, v.namespace),
		DNSNames:                    kubernetesutils.DNSNamesForService(vpaconstants.AdmissionControllerServiceName, v.namespace),
		CertType:                    secretsutils.ServerCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v.values.SecretNameServerCA, secretsmanager.UseCurrentCA), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return err
	}
	v.serverSecretName = serverSecret.Name

	var allResources component.ResourceConfigs
	if v.values.Enabled {
		allResources = component.MergeResourceConfigs(
			v.admissionControllerResourceConfigs(),
			v.recommenderResourceConfigs(),
			v.updaterResourceConfigs(),
			v.generalResourceConfigs(),
		)
	}

	var (
		registry    *managedresources.Registry
		crdDeployer component.DeployWaiter
	)
	if v.values.ClusterType == component.ClusterTypeSeed {
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	} else {
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
	}

	if v.values.ClusterType == component.ClusterTypeShoot {
		genericTokenKubeconfigSecret, found := v.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
		if !found {
			return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
		}
		v.genericTokenKubeconfigSecretName = &genericTokenKubeconfigSecret.Name

		for _, name := range []string{
			v1beta1constants.DeploymentNameVPAAdmissionController,
			v1beta1constants.DeploymentNameVPARecommender,
			v1beta1constants.DeploymentNameVPAUpdater,
		} {
			if err := gardenerutils.NewShootAccessSecret(name, v.namespace).Reconcile(ctx, v.client); err != nil {
				return err
			}
		}

		crdDeployer, err = NewCRD(v.client, nil, registry)
		if err != nil {
			return fmt.Errorf("failed to create CRDDeployer: %w", err)
		}
		if err := crdDeployer.Deploy(ctx); err != nil {
			return err
		}
	}

	return component.DeployResourceConfigs(ctx, v.client, v.namespace, v.values.ClusterType, v.managedResourceName(), nil, registry, allResources)
}

func (v *vpa) Destroy(ctx context.Context) error {
	return component.DestroyResourceConfigs(ctx, v.client, v.namespace, v.values.ClusterType, v.managedResourceName(),
		v.admissionControllerResourceConfigs(),
		v.recommenderResourceConfigs(),
		v.updaterResourceConfigs(),
		v.generalResourceConfigs(),
	)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (v *vpa) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, v.client, v.namespace, v.managedResourceName())
}

func (v *vpa) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, v.client, v.namespace, v.managedResourceName())
}

func (v *vpa) GetValues() Values {
	return v.values
}

func (v *vpa) managedResourceName() string {
	if v.values.ClusterType == component.ClusterTypeSeed {
		return ManagedResourceControlName
	}
	return shootManagedResourceName
}

func (v *vpa) emptyService(name string) *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: v.namespace}}
}

func (v *vpa) emptyServiceAccount(name string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: v.namespace}}
}

func (v *vpa) emptyClusterRole(name string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: v.rbacNamePrefix() + name}}
}

func (v *vpa) emptyClusterRoleBinding(name string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: v.rbacNamePrefix() + name}}
}

func (v *vpa) emptyRole(name string) *rbacv1.Role {
	return &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: v.rbacNamePrefix() + name, Namespace: v.namespaceForApplicationClassResource()}}
}

func (v *vpa) emptyRoleBinding(name string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: v.rbacNamePrefix() + name, Namespace: v.namespaceForApplicationClassResource()}}
}

func (v *vpa) emptyDeployment(name string) *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: v.namespace}}
}

func (v *vpa) emptyPodDisruptionBudget(name string) *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: v.namespace}}
}

func (v *vpa) emptyServiceMonitor(name string) *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{ObjectMeta: monitoringutils.ConfigObjectMeta(name, v.namespace, v.getPrometheusLabel())}
}

func (v *vpa) emptyVerticalPodAutoscaler(name string) *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: v.namespace}}
}

func (v *vpa) emptyMutatingWebhookConfiguration() *admissionregistrationv1.MutatingWebhookConfiguration {
	suffix := "source"
	if v.values.ClusterType == component.ClusterTypeShoot {
		suffix = "target"
	}

	// The order in which MutatingWebhooks are called is determined alphabetically. This webhook's name intentionally
	// starts with 'zzz', such that it is called after all other webhooks which inject containers. All containers
	// injected by webhooks that are called _after_ the vpa webhook will not be under control of vpa.
	// See also the `gardener.cloud/description` annotation on the MutatingWebhook.
	return &admissionregistrationv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "zzz-vpa-webhook-config-" + suffix}}
}

func (v *vpa) rbacNamePrefix() string {
	prefix := "gardener.cloud:vpa:"

	if v.values.ClusterType == component.ClusterTypeSeed {
		return prefix + "source:"
	}

	return prefix + "target:"
}

func (v *vpa) namespaceForApplicationClassResource() string {
	if v.values.ClusterType == component.ClusterTypeSeed {
		return v.namespace
	}
	return metav1.NamespaceSystem
}

func getAppLabel(appValue string) map[string]string {
	return map[string]string{v1beta1constants.LabelApp: appValue}
}

func getRoleLabel() map[string]string {
	return map[string]string{v1beta1constants.GardenRole: "vpa"}
}

func getAllLabels(appValue string) map[string]string {
	return utils.MergeStringMaps(getAppLabel(appValue), getRoleLabel())
}

func (v *vpa) getDeploymentLabels(appValue string) map[string]string {
	if v.values.ClusterType == component.ClusterTypeSeed {
		return utils.MergeStringMaps(getAppLabel(appValue), getRoleLabel())
	}

	return utils.MergeStringMaps(getAppLabel(appValue), map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
	})
}

func (v *vpa) injectAPIServerConnectionSpec(deployment *appsv1.Deployment, name string, serviceAccountName *string) {
	if serviceAccountName != nil {
		deployment.Spec.Template.Spec.ServiceAccountName = *serviceAccountName

		if name != recommender {
			deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
				Name: "NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			})
		}
	} else {
		deployment.Spec.Template.Spec.AutomountServiceAccountToken = ptr.To(false)

		utilruntime.Must(gardenerutils.InjectGenericKubeconfig(deployment, *v.genericTokenKubeconfigSecretName, gardenerutils.SecretNamePrefixShootAccess+deployment.Name))
	}
}

func (v *vpa) getPrometheusLabel() string {
	if v.values.ClusterType == component.ClusterTypeSeed {
		if v.values.IsGardenCluster {
			return garden.Label
		}
		return seed.Label
	}
	return shoot.Label
}
