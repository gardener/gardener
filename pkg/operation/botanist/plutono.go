// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/plutono"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/operation/common"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DefaultPlutono returns a deployer for Plutono.
func (b *Botanist) DefaultPlutono() (plutono.Interface, error) {
	return shared.NewPlutono(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		b.SecretsManager,
		component.ClusterTypeShoot,
		b.Shoot.GetReplicas(1),
		"",
		b.ComputePlutonoHost(),
		v1beta1constants.PriorityClassNameShootControlPlane100,
		b.ShootUsesDNS(),
		b.Shoot.IsWorkerless,
		false,
		b.Shoot.NodeLocalDNSEnabled,
		b.Shoot.VPNHighAvailabilityEnabled,
		b.Shoot.WantsVerticalPodAutoscaler,
		nil,
	)
}

// DeployPlutono deploys the plutono in the Seed cluster.
func (b *Botanist) DeployPlutono(ctx context.Context) error {
	if b.ControlPlaneWildcardCert != nil {
		b.Operation.Shoot.Components.ControlPlane.Plutono.SetWildcardCertName(pointer.String(b.ControlPlaneWildcardCert.GetName()))
	}
	// disable monitoring if shoot has purpose testing or monitoring and vali is disabled
	if !b.Operation.WantsPlutono() {
		if err := b.Shoot.Components.ControlPlane.Plutono.Destroy(ctx); err != nil {
			return err
		}

		secretName := gardenerutils.ComputeShootProjectSecretName(b.Shoot.GetInfo().Name, gardenerutils.ShootProjectSecretSuffixMonitoring)
		return kubernetesutils.DeleteObject(ctx, b.GardenClient, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: b.Shoot.GetInfo().Namespace}})
	}

	// TODO(rickardsjp, istvanballok): Remove in release v1.77 once the Grafana to Plutono migration is complete.
	if err := common.DeleteGrafana(ctx, b.SeedClientSet, b.Shoot.SeedNamespace); err != nil {
		return err
	}

	if err := b.Shoot.Components.ControlPlane.Plutono.Deploy(ctx); err != nil {
		return err
	}

	credentialsSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameObservabilityIngressUsers)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameObservabilityIngressUsers)
	}

	return b.syncShootCredentialToGarden(
		ctx,
		gardenerutils.ShootProjectSecretSuffixMonitoring,
		map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleMonitoring},
		map[string]string{"url": "https://" + b.ComputePlutonoHost()},
		credentialsSecret.Data,
	)
}
