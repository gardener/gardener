// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package vpa

import (
	"context"
	"fmt"
	"strconv"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ManagedResourceControlName is the name of the vpa managed resource for seeds.
	ManagedResourceControlName = "vpa"
	shootManagedResourceName   = "shoot-core-" + ManagedResourceControlName
)

// Interface contains functions for a VPA deployer.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
}

// New creates a new instance of DeployWaiter for the Kubernetes Vertical Pod Autoscaler.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) Interface {
	v := &vpa{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}

	if values.ClusterType == component.ClusterTypeSeed {
		v.registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
	} else {
		v.registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
		v.crdDeployer = NewCRD(nil, v.registry)
	}

	return v
}

type vpa struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values

	registry    *managedresources.Registry
	crdDeployer component.Deployer

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
	// Enabled specifies if VPA is enabled. If VPA is not enabled and the cluster type is "seed", only vpa-exporter
	// is deployed.
	Enabled bool
	// SecretNameServerCA is the name of the server CA secret.
	SecretNameServerCA string

	// AdmissionController is a set of configuration values for the vpa-admission-controller.
	AdmissionController ValuesAdmissionController
	// Exporter is a set of configuration values for the vpa-exporter.
	Exporter ValuesExporter
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
	v.caBundle = caSecret.Data[secretutils.DataKeyCertificateBundle]

	serverSecret, err := v.secretsManager.Generate(ctx, &secretutils.CertificateSecretConfig{
		Name:                        "vpa-admission-controller-server",
		CommonName:                  fmt.Sprintf("%s.%s.svc", admissionControllerServiceName, v.namespace),
		DNSNames:                    kutil.DNSNamesForService(admissionControllerServiceName, v.namespace),
		CertType:                    secretutils.ServerCert,
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

	if v.values.ClusterType == component.ClusterTypeSeed {
		allResources = component.MergeResourceConfigs(allResources, v.exporterResourceConfigs())
	} else {
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
			if err := gutil.NewShootAccessSecret(name, v.namespace).Reconcile(ctx, v.client); err != nil {
				return err
			}
		}

		if err := v.crdDeployer.Deploy(ctx); err != nil {
			return err
		}
	}

	if err := component.DeployResourceConfigs(ctx, v.client, v.namespace, v.values.ClusterType, v.managedResourceName(), v.registry, allResources); err != nil {
		return err
	}

	if v.values.ClusterType == component.ClusterTypeSeed {
		// TODO(rfranzke): Remove in a future release.
		return kutil.DeleteObjects(ctx, v.client,
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:exporter"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:exporter"}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:evictioner"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:evictioner"}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:metrics-reader"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:metrics-reader"}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:checkpoint-actor"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:checkpoint-actor"}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:admission-controller"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:admission-controller"}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vpa-tls-certs", Namespace: v.namespace}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:actor"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:actor"}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:target-reader"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:target-reader"}},
			&admissionregistrationv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "vpa-webhook-config-seed"}},
		)
	}

	// TODO(rfranzke): Remove in a future release.
	return kutil.DeleteObjects(ctx, v.client,
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vpa-tls-certs", Namespace: v.namespace}},
	)
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

func (v *vpa) emptyDeployment(name string) *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: v.namespace}}
}

func (v *vpa) emptyVerticalPodAutoscaler(name string) *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: v.namespace}}
}

func (v *vpa) emptyNetworkPolicy(name string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: v.namespace}}
}

func (v *vpa) emptyMutatingWebhookConfiguration() *admissionregistrationv1.MutatingWebhookConfiguration {
	suffix := "source"
	if v.values.ClusterType == component.ClusterTypeShoot {
		suffix = "target"
	}

	return &admissionregistrationv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "vpa-webhook-config-" + suffix}}
}

func (v *vpa) rbacNamePrefix() string {
	prefix := "gardener.cloud:vpa:"

	if v.values.ClusterType == component.ClusterTypeSeed {
		return prefix + "source:"
	}

	return prefix + "target:"
}

func (v *vpa) serviceAccountNamespace() string {
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
		deployment.Spec.Template.Spec.AutomountServiceAccountToken = pointer.Bool(false)

		// TODO(shafeeqes): Adapt admission-controller to use kubeconfig too, after https://github.com/kubernetes/autoscaler/issues/4844 is fixed.
		if name != admissionController {
			utilruntime.Must(gutil.InjectGenericKubeconfig(deployment, *v.genericTokenKubeconfigSecretName, gutil.SecretNamePrefixShootAccess+deployment.Name))
		} else {
			deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env,
				corev1.EnvVar{
					Name:  "KUBERNETES_SERVICE_HOST",
					Value: v1beta1constants.DeploymentNameKubeAPIServer,
				},
				corev1.EnvVar{
					Name:  "KUBERNETES_SERVICE_PORT",
					Value: strconv.Itoa(kubeapiserver.Port),
				},
			)
			deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: "shoot-access",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						DefaultMode: pointer.Int32(420),
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: v.caSecretName,
									},
									Items: []corev1.KeyToPath{{
										Key:  secretutils.DataKeyCertificateBundle,
										Path: "ca.crt",
									}},
								},
							},
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: gutil.SecretNamePrefixShootAccess + name,
									},
									Items: []corev1.KeyToPath{{
										Key:  resourcesv1alpha1.DataKeyToken,
										Path: "token",
									}},
									Optional: pointer.Bool(false),
								},
							},
						},
					},
				},
			})
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
				Name:      "shoot-access",
				MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
				ReadOnly:  true,
			})
		}
	}
}

func durationDeref(ptr *metav1.Duration, def metav1.Duration) metav1.Duration {
	if ptr != nil {
		return *ptr
	}
	return def
}
