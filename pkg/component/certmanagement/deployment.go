// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certmanagement

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// deploymentManagedResourceName is the name of the managed resource for the resources.
	deploymentManagedResourceName = "cert-management-deployment"

	resourceName  = "cert-controller-manager"
	containerName = "cert-management"

	serverPort        = 8080
	rsaPrivateKeySize = 3072
)

// newCertManagementDeployment creates a new instance of DeployWaiter for the CertManagement deployment.
func newCertManagementDeployment(cl client.Client, values Values) component.DeployWaiter {
	return &certManagementDeployment{
		client:    cl,
		namespace: values.Namespace,
		image:     values.Image,
		config:    values.DeployConfig,
	}
}

type certManagementDeployment struct {
	client    client.Client
	namespace string
	image     string
	config    *operatorv1alpha1.CertManagementConfig
}

func (d *certManagementDeployment) Deploy(ctx context.Context) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: d.namespace,
			},
		}
		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: resourceName,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"services"},
					Verbs:     []string{"get", "list", "update", "watch"},
				},
				{
					APIGroups: []string{"networking.k8s.io"},
					Resources: []string{"ingresses"},
					Verbs:     []string{"get", "list", "update", "watch"},
				},
				{
					APIGroups: []string{"gateway.networking.k8s.io"},
					Resources: []string{"gateways", "httproutes"},
					Verbs:     []string{"get", "list", "update", "watch"},
				},
				{
					APIGroups: []string{"networking.istio.io"},
					Resources: []string{"gateways", "virtualservices"},
					Verbs:     []string{"get", "list", "update", "watch"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"secrets"},
					Verbs:     []string{"get", "list", "update", "watch", "create", "delete"},
				},
				{
					APIGroups: []string{"cert.gardener.cloud"},
					Resources: []string{
						"issuers", "issuers/status",
						"certificates", "certificates/status",
						"certificaterevocations", "certificaterevocations/status",
					},
					Verbs: []string{"get", "list", "update", "watch", "create", "delete"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"events"},
					Verbs:     []string{"create", "patch"},
				},
				{
					APIGroups: []string{"apiextensions.k8s.io"},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     []string{"get", "list", "update", "create", "watch"},
				},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRole.Name,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccount.Name,
				Namespace: serviceAccount.Namespace,
			}},
		}
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: d.namespace,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups:     []string{"coordination.k8s.io"},
					Resources:     []string{"leases"},
					ResourceNames: []string{"cert-controller-manager-controllers"},
					Verbs:         []string{"get", "watch", "update"},
				},
				{
					APIGroups: []string{"dns.gardener.cloud"},
					Resources: []string{"dnsentries"},
					Verbs:     []string{"get", "list", "update", "watch", "create", "delete"},
				},
			},
		}
		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      role.Name,
				Namespace: d.namespace,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      serviceAccount.Name,
					Namespace: serviceAccount.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     role.Name,
			},
		}

		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: d.namespace,
				Labels: gardenerutils.MergeStringMaps(getDeploymentLabels(), map[string]string{
					resourcesv1alpha1.HighAvailabilityConfigType: resourcesv1alpha1.HighAvailabilityConfigTypeController,
				}),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             ptr.To[int32](1),
				RevisionHistoryLimit: ptr.To[int32](5),
				Selector:             &metav1.LabelSelector{MatchLabels: getDeploymentLabels()},
				Strategy:             appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: gardenerutils.MergeStringMaps(getDeploymentLabels(), map[string]string{
							v1beta1constants.LabelNetworkPolicyToDNS:              v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyToPublicNetworks:   v1beta1constants.LabelNetworkPolicyAllowed,
							v1beta1constants.LabelNetworkPolicyToPrivateNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
						}),
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: serviceAccount.Name,
						Containers: []corev1.Container{{
							Name:            containerName,
							Image:           d.image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--name=cert-controller-manager",
								fmt.Sprintf("--dns-namespace=%s", d.namespace),
								"--use-dnsrecords",
								fmt.Sprintf("--issuer.issuer-namespace=%s", d.namespace),
								fmt.Sprintf("--server-port-http=%d", serverPort),
								fmt.Sprintf("--default-rsa-private-key-size=%d", rsaPrivateKeySize),
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/healthz",
										Port:   intstr.FromInt32(serverPort),
										Scheme: "HTTP",
									},
								},
								InitialDelaySeconds: 30,
								TimeoutSeconds:      5,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							Ports: []corev1.ContainerPort{{
								ContainerPort: serverPort,
								Protocol:      corev1.ProtocolTCP,
							}},
						}},
					},
				},
			},
		}
	)

	objects := []client.Object{
		serviceAccount,
		clusterRole,
		clusterRoleBinding,
		role,
		roleBinding,
		deployment,
	}

	if d.config != nil && d.config.CACertificatesSecretRef != nil {
		caCertSecret := &corev1.Secret{}
		if err := d.client.Get(ctx, getObjectKeyLocalObjectRef(*d.config.CACertificatesSecretRef), caCertSecret); err != nil {
			return err
		}
		caCertSecret.ObjectMeta = metav1.ObjectMeta{
			Name:      caCertSecret.Name,
			Namespace: d.namespace,
		}
		utilruntime.Must(kubernetesutils.MakeUnique(caCertSecret))
		objects = append(objects, caCertSecret)
		container := &deployment.Spec.Template.Spec.Containers[0]
		container.Env = append(container.Env,
			corev1.EnvVar{
				Name:  "LEGO_CA_SYSTEM_CERT_POOL",
				Value: "true",
			},
			corev1.EnvVar{
				Name:  "LEGO_CA_CERTIFICATES",
				Value: "/var/run/cert-manager/certs/bundle.crt",
			},
		)
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "ca-certificates",
			ReadOnly:  true,
			MountPath: "/var/run/cert-manager/certs",
		})
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "ca-certificates",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: caCertSecret.Name,
				},
			},
		})
		utilruntime.Must(references.InjectAnnotations(deployment))
	}

	resources, err := registry.AddAllAndSerialize(objects...)
	if err != nil {
		return err
	}

	return managedresources.CreateForSeed(ctx, d.client, v1beta1constants.GardenNamespace, deploymentManagedResourceName, false, resources)
}

func (d *certManagementDeployment) deleteHelmRelease(ctx context.Context) error {
	if err := d.client.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(d.namespace), client.MatchingLabels{
		"name":  "cert-management",
		"owner": "helm",
	}); err != nil {
		return fmt.Errorf("deleting secrets for old Helm releases failed: %w", err)
	}
	return nil
}

func (d *certManagementDeployment) Destroy(ctx context.Context) error {
	return managedresources.DeleteForSeed(ctx, d.client, v1beta1constants.GardenNamespace, deploymentManagedResourceName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (d *certManagementDeployment) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, d.client, v1beta1constants.GardenNamespace, deploymentManagedResourceName)
}

func (d *certManagementDeployment) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, d.client, v1beta1constants.GardenNamespace, deploymentManagedResourceName)
}

func getDeploymentLabels() map[string]string {
	return map[string]string{
		appName:     componentName,
		appInstance: componentName,
	}
}

func getObjectKeyLocalObjectRef(ref corev1.LocalObjectReference) client.ObjectKey {
	return client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: ref.Name}
}
