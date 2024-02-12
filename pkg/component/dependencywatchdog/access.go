// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dependencywatchdog

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
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
	// managedResourceTargetName is the name of the managed resource created for DWD.
	managedResourceTargetName = "shoot-core-dependency-watchdog"
	// DefaultKCMNodeMonitorGraceDuration is the default value for the NodeMonitorGraceDuration parameter of KCM.
	DefaultKCMNodeMonitorGraceDuration = 40 * time.Second

	// ExternalProbeSecretName is the name of the kubecfg secret with internal DNS for external access.
	ExternalProbeSecretName = gardenerutils.SecretNamePrefixShootAccess + "dependency-watchdog-external-probe"
	// InternalProbeSecretName is the name of the kubecfg secret with cluster IP access.
	InternalProbeSecretName = gardenerutils.SecretNamePrefixShootAccess + "dependency-watchdog-internal-probe"
	// TempGRMConfigMapName is the name is the configMap that is temporarily deployed for DWD prober
	TempGRMConfigMapName = "gardener-resource-manager-dwd"
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

// NewDWDAccess creates a new instance of dependencyWatchdogAccess for shoot cluster access for the dependency-watchdog.
// TODO(aaronfern): remove this when g/g v1.91 is released
func NewDWDAccess(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values AccessValues,
) *dependencyWatchdogAccess {
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
	// Create shoot access secret
	caSecret, found := d.secretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	var (
		shootAccessSecret = gardenerutils.NewShootAccessSecret(KubeConfigSecretName, d.namespace).WithNameOverride(KubeConfigSecretName)
		kubeconfig        = kubernetesutils.NewKubeconfig(
			d.namespace,
			clientcmdv1.Cluster{Server: d.values.ServerInCluster, CertificateAuthorityData: caSecret.Data[secretsutils.DataKeyCertificateBundle]},
			clientcmdv1.AuthInfo{Token: ""},
		)
	)

	if err := shootAccessSecret.WithKubeconfig(kubeconfig).Reconcile(ctx, d.client); err != nil {
		return err
	}

	// Delete old Secrets and CM with shoot reconcile
	if err := kubernetesutils.DeleteObjects(ctx, d.client,
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: InternalProbeSecretName, Namespace: d.namespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ExternalProbeSecretName, Namespace: d.namespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: TempGRMConfigMapName, Namespace: d.namespace}},
	); err != nil {
		return err
	}

	var registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

	role := &rbacv1.Role{
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
	roleBinding := &rbacv1.RoleBinding{
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

	resources, err := registry.AddAllAndSerialize(
		role,
		roleBinding,
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, d.client, d.namespace, managedResourceTargetName, managedresources.LabelValueGardener, false, resources)
}

// TODO(aaronfern): remove this when g/g v1.91 is released
func (d *dependencyWatchdogAccess) DeployMigrate(ctx context.Context) error {
	// Create shoot access secret
	caSecret := corev1.Secret{}
	err := d.client.Get(ctx, types.NamespacedName{Namespace: d.namespace, Name: v1beta1constants.SecretNameCACluster}, &caSecret)
	if err != nil {
		return fmt.Errorf("error in fetching secret %s. err: %v", v1beta1constants.SecretNameCACluster, err)
	}

	var (
		shootAccessSecret = gardenerutils.NewShootAccessSecret(KubeConfigSecretName, d.namespace).WithNameOverride(KubeConfigSecretName)
		kubeconfig        = kubernetesutils.NewKubeconfig(
			d.namespace,
			clientcmdv1.Cluster{Server: d.values.ServerInCluster, CertificateAuthorityData: caSecret.Data[secretsutils.DataKeyCertificateBundle]},
			clientcmdv1.AuthInfo{Token: ""},
		)
	)

	if err := shootAccessSecret.WithKubeconfig(kubeconfig).Reconcile(ctx, d.client); err != nil {
		return err
	}

	var registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

	role := &rbacv1.Role{
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
	roleBinding := &rbacv1.RoleBinding{
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

	resources, err := registry.AddAllAndSerialize(
		role,
		roleBinding,
	)
	if err != nil {
		return err
	}

	return managedresources.CreateForShoot(ctx, d.client, d.namespace, managedResourceTargetName, managedresources.LabelValueGardener, false, resources)
}

func (d *dependencyWatchdogAccess) Destroy(ctx context.Context) error {
	return kubernetesutils.DeleteObjects(ctx, d.client,
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: KubeConfigSecretName, Namespace: d.namespace}},
		&resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceTargetName, Namespace: d.namespace}},
	)
}
