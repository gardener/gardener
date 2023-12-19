// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package dependencywatchdog

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// DefaultProbeInterval is the default value of interval between two probes by DWD prober
	DefaultProbeInterval = 30 * time.Second
	// DefaultWatchDuration is the default value of the total duration for which a DWD Weeder watches for any dependant Pod to transition to CrashLoopBackoff after the target service has recovered.
	DefaultWatchDuration = 5 * time.Minute
	// ExternalProbeSecretName is the name of the kubecfg secret with internal DNS for external access.
	ExternalProbeSecretName = gardenerutils.SecretNamePrefixShootAccess + "dependency-watchdog-external-probe"
	// InternalProbeSecretName is the name of the kubecfg secret with cluster IP access.
	InternalProbeSecretName = gardenerutils.SecretNamePrefixShootAccess + "dependency-watchdog-internal-probe"
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
	// ServerOutOfCluster is the out-of-cluster address of a kube-apiserver.
	ServerOutOfCluster string
	// ServerInCluster is the in-cluster address of a kube-apiserver.
	ServerInCluster string
}

func (d *dependencyWatchdogAccess) Deploy(ctx context.Context) error {
	caSecret, found := d.secretsManager.Get(v1beta1constants.SecretNameCACluster)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCACluster)
	}

	for name, server := range map[string]string{
		InternalProbeSecretName: d.values.ServerInCluster,
		ExternalProbeSecretName: d.values.ServerOutOfCluster,
	} {
		var (
			shootAccessSecret = gardenerutils.NewShootAccessSecret(name, d.namespace).WithNameOverride(name)
			kubeconfig        = kubernetesutils.NewKubeconfig(
				d.namespace,
				clientcmdv1.Cluster{Server: server, CertificateAuthorityData: caSecret.Data[secretsutils.DataKeyCertificateBundle]},
				clientcmdv1.AuthInfo{Token: ""},
			)
		)

		if err := shootAccessSecret.WithKubeconfig(kubeconfig).Reconcile(ctx, d.client); err != nil {
			return err
		}
	}

	return nil
}

func (d *dependencyWatchdogAccess) Destroy(ctx context.Context) error {
	return kubernetesutils.DeleteObjects(ctx, d.client,
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: InternalProbeSecretName, Namespace: d.namespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ExternalProbeSecretName, Namespace: d.namespace}},
	)
}
