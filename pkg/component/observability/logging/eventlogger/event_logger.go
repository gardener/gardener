// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package eventlogger

import (
	"context"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	kubeapiserverconstants "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	name                = "event-logger"
	vpaName             = "event-logger-vpa"
	managedResourceName = "shoot-event-logger"
	roleName            = "gardener.cloud:logging:event-logger"
)

// Values are the values for the event-logger.
type Values struct {
	// Image of the event logger.
	Image string
	// Replicas is the number of pod replicas.
	Replicas int32
}
type eventLogger struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

// New creates a new instance of event-logger for the shoot event logging.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) (
	component.Deployer,
	error,
) {
	if client == nil {
		return nil, errors.New("client cannot be nil")
	}

	if len(namespace) == 0 {
		return nil, errors.New("namespace cannot be empty")
	}

	if len(values.Image) == 0 {
		return nil, errors.New("image cannot be empty")
	}

	if secretsManager == nil {
		return nil, errors.New("secret manager cannot be nil")
	}

	return &eventLogger{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}, nil
}

func (l *eventLogger) Deploy(ctx context.Context) error {
	if err := l.reconcileRBACForShoot(ctx); err != nil {
		return err
	}

	if err := l.reconcileRBACForSeed(ctx); err != nil {
		return err
	}

	if err := l.reconcileDeployment(ctx); err != nil {
		return err
	}

	return l.reconcileVPA(ctx)
}

func (l *eventLogger) Destroy(ctx context.Context) error {
	if err := l.deleteRBACForShoot(ctx); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(
		ctx,
		l.client,
		l.emptyServiceAccount(),
		l.emptyRole(),
		l.emptyRoleBinding(),
		l.emptyVPA(),
		l.emptyDeployment(),
	)
}

func (l *eventLogger) reconcileRBACForShoot(ctx context.Context) error {
	eventLoggerShootAccessSecret := l.newShootAccessSecret()
	if err := eventLoggerShootAccessSecret.Reconcile(ctx, l.client); err != nil {
		return err
	}

	var (
		eventLoggerClusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   roleName,
				Labels: getLabels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      eventLoggerShootAccessSecret.ServiceAccountName,
				Namespace: metav1.NamespaceSystem,
			}},
		}

		eventLoggerClusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: getLabels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{
						"",
					},
					Resources: []string{
						"events",
					},
					Verbs: []string{
						"get",
						"list",
						"watch",
					},
				},
			},
		}

		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
	)

	resources, err := registry.AddAllAndSerialize(
		eventLoggerClusterRole,
		eventLoggerClusterRoleBinding,
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, l.client, l.namespace, managedResourceName, managedresources.LabelValueGardener, false, resources)
}

func (l *eventLogger) reconcileRBACForSeed(ctx context.Context) error {
	var (
		serviceAccount = l.emptyServiceAccount()
		role           = l.emptyRole()
		roleBinding    = l.emptyRoleBinding()
	)

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, l.client, serviceAccount, func() error {
		serviceAccount.AutomountServiceAccountToken = ptr.To(false)
		serviceAccount.Labels = getLabels()
		return nil
	}); err != nil {
		return err
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, l.client, role, func() error {
		role.Labels = getLabels()
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"get", "list", "watch"},
			},
		}
		return nil
	}); err != nil {
		return err
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, l.client, roleBinding, func() error {
		roleBinding.Labels = getLabels()
		roleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     role.Name,
		}
		roleBinding.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccount.Name,
			Namespace: l.namespace,
		}}
		return nil
	})
	return err
}

func (l *eventLogger) reconcileDeployment(ctx context.Context) error {
	genericTokenKubeconfigSecret, found := l.secretsManager.Get(v1beta1constants.SecretNameGenericTokenKubeconfig)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameGenericTokenKubeconfig)
	}

	deployment := l.emptyDeployment()
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, l.client, deployment, func() error {
		deployment.Labels = getLabels()
		deployment.Spec = appsv1.DeploymentSpec{
			RevisionHistoryLimit: ptr.To[int32](1),
			Replicas:             ptr.To(l.values.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: getLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: utils.MergeStringMaps(getLabels(), map[string]string{
						v1beta1constants.LabelNetworkPolicyToDNS:                                                                    v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:                                                       v1beta1constants.LabelNetworkPolicyAllowed,
						gardenerutils.NetworkPolicyLabel(v1beta1constants.DeploymentNameKubeAPIServer, kubeapiserverconstants.Port): v1beta1constants.LabelNetworkPolicyAllowed,
					}),
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: name,
					PriorityClassName:  v1beta1constants.PriorityClassNameShootControlPlane100,
					Containers: []corev1.Container{
						{
							Name:            name,
							Image:           l.values.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         l.computeCommand(),
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("12m"),
									corev1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
							},
						},
					},
				},
			},
		}

		utilruntime.Must(gardenerutils.InjectGenericKubeconfig(deployment, genericTokenKubeconfigSecret.Name, gardenerutils.SecretNamePrefixShootAccess+name))

		return nil
	})
	return err
}

func (l *eventLogger) reconcileVPA(ctx context.Context) error {
	var (
		vpa              = l.emptyVPA()
		vpaUpdateMode    = vpaautoscalingv1.UpdateModeAuto
		controlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
	)

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, l.client, vpa, func() error {
		vpa.Spec = vpaautoscalingv1.VerticalPodAutoscalerSpec{
			TargetRef: &autoscalingv1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
				Name:       v1beta1constants.DeploymentNameEventLogger,
			},
			UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: &vpaUpdateMode,
			},
			ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
				ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
					{
						ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
						MinAllowed: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("20Mi"),
						},
						ControlledValues: &controlledValues,
					},
				},
			},
		}
		return nil
	})

	return err
}

func (l *eventLogger) deleteRBACForShoot(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, l.client, l.namespace, managedResourceName); err != nil {
		return err
	}

	return kubernetesutils.DeleteObjects(ctx, l.client, l.newShootAccessSecret().Secret)
}

func (l *eventLogger) newShootAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(name, l.namespace)
}

func (l *eventLogger) emptyRole() *rbacv1.Role {
	return &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: l.namespace}}
}

func (l *eventLogger) emptyRoleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: l.namespace}}
}

func (l *eventLogger) emptyServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: l.namespace}}
}

func (l *eventLogger) emptyDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: l.namespace}}
}

func (l *eventLogger) emptyVPA() *vpaautoscalingv1.VerticalPodAutoscaler {
	return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: l.namespace}}
}

func (l *eventLogger) computeCommand() []string {
	return []string{
		"./event-logger",
		"--seed-event-namespaces=" + l.namespace,
		"--shoot-kubeconfig=" + gardenerutils.PathGenericKubeconfig,
		"--shoot-event-namespaces=" + metav1.NamespaceSystem + "," + metav1.NamespaceDefault,
	}
}

func getLabels() map[string]string {
	return map[string]string{
		v1beta1constants.LabelApp:   name,
		v1beta1constants.LabelRole:  v1beta1constants.LabelLogging,
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleLogging,
	}
}
