// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dependencywatchdog

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// DefaultProbeInterval is the default value of interval between two probes by DWD prober
	DefaultProbeInterval = 30 * time.Second
	// DefaultWatchDuration is the default value of the total duration for which a DWD Weeder watches for any dependant Pod to transition to CrashLoopBackoff after the target service has recovered.
	DefaultWatchDuration = 5 * time.Minute
	// KubeConfigSecretName is the name of the kubecfg secret with internal DNS for external access.
	KubeConfigSecretName = gardenerutils.SecretNamePrefixShootAccess + "dependency-watchdog-probe"
	// managedResourceName is the name of the managed resource created for DWD.
	managedResourceName = "shoot-core-dependency-watchdog"
)

// NewAccess creates a new instance of the deployer for shoot cluster access for the dependency-watchdog.
func NewAccess(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values AccessValues,
) component.Deployer {
	return &dependencyWatchdogAccess{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type dependencyWatchdogAccess struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         AccessValues
}

// AccessValues contains configurations for the component.
type AccessValues struct {
	// ServerInCluster is the in-cluster address of a kube-apiserver.
	ServerInCluster string
}

func (d *dependencyWatchdogAccess) Deploy(ctx context.Context) error {
	caSecret, found := d.secretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	if err := d.createShootAccessSecret(ctx, caSecret); err != nil {
		return err
	}

	return d.createManagedResource(ctx)
}

func (d *dependencyWatchdogAccess) createShootAccessSecret(ctx context.Context, caSecret *corev1.Secret) error {
	var (
		shootAccessSecret = gardenerutils.NewShootAccessSecret(KubeConfigSecretName, d.namespace).WithNameOverride(KubeConfigSecretName)
		kubeconfig        = kubernetesutils.NewKubeconfig(
			d.namespace,
			clientcmdv1.Cluster{Server: d.values.ServerInCluster, CertificateAuthorityData: caSecret.Data[secretsutils.DataKeyCertificateBundle]},
			clientcmdv1.AuthInfo{Token: ""},
		)
	)

	return shootAccessSecret.WithKubeconfig(kubeconfig).Reconcile(ctx, d.client)
}

func (d *dependencyWatchdogAccess) createManagedResource(ctx context.Context) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:target:" + prefixDependencyWatchdog,
				Namespace: corev1.NamespaceNodeLease,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{"leases"},
					Verbs:     []string{"get", "list"},
				},
			},
		}
		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardener.cloud:target:" + prefixDependencyWatchdog,
				Namespace: corev1.NamespaceNodeLease,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     role.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      strings.TrimPrefix(KubeConfigSecretName, gardenerutils.SecretNamePrefixShootAccess),
				Namespace: metav1.NamespaceSystem,
			}},
		}
		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:" + prefixDependencyWatchdog,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"nodes"},
					Verbs:     []string{"get", "list"},
				},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gardener.cloud:target:" + prefixDependencyWatchdog,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRole.Name,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      strings.TrimPrefix(KubeConfigSecretName, gardenerutils.SecretNamePrefixShootAccess),
				Namespace: metav1.NamespaceSystem,
			}},
		}
	)

	resources, err := registry.AddAllAndSerialize(
		role,
		roleBinding,
		clusterRole,
		clusterRoleBinding,
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, d.client, d.namespace, managedResourceName, managedresources.LabelValueGardener, false, resources)
}

func (d *dependencyWatchdogAccess) Destroy(ctx context.Context) error {
	if err := managedresources.DeleteForShoot(ctx, d.client, d.namespace, managedResourceName); err != nil {
		return err
	}
	return kubernetesutils.DeleteObjects(ctx, d.client,
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: KubeConfigSecretName, Namespace: d.namespace}},
	)
}
