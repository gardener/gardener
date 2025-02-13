// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component/extensions/worker"
	"github.com/gardener/gardener/pkg/controllerutils"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/secrets"
)

// DefaultWorker creates the default deployer for the Worker custom resource.
func (b *Botanist) DefaultWorker() worker.Interface {
	return worker.New(
		b.Logger,
		b.SeedClientSet.Client(),
		&worker.Values{
			Namespace:           b.Shoot.ControlPlaneNamespace,
			Name:                b.Shoot.GetInfo().Name,
			Type:                b.Shoot.GetInfo().Spec.Provider.Type,
			Region:              b.Shoot.GetInfo().Spec.Region,
			Workers:             b.Shoot.GetInfo().Spec.Provider.Workers,
			KubernetesVersion:   b.Shoot.KubernetesVersion,
			MachineTypes:        b.Shoot.CloudProfile.Spec.MachineTypes,
			NodeLocalDNSEnabled: v1beta1helper.IsNodeLocalDNSEnabled(b.Shoot.GetInfo().Spec.SystemComponents),
		},
		worker.DefaultInterval,
		worker.DefaultSevereThreshold,
		worker.DefaultTimeout,
	)
}

// DeployWorker deploys the Worker custom resource and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration
func (b *Botanist) DeployWorker(ctx context.Context) error {
	if v1beta1helper.ShootEnablesSSHAccess(b.Shoot.GetInfo()) {
		sshKeypairSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameSSHKeyPair)
		if !found {
			return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameSSHKeyPair)
		}
		b.Shoot.Components.Extensions.Worker.SetSSHPublicKey(sshKeypairSecret.Data[secrets.DataKeySSHAuthorizedKeys])
	}

	b.Shoot.Components.Extensions.Worker.SetInfrastructureProviderStatus(b.Shoot.Components.Extensions.Infrastructure.ProviderStatus())
	b.Shoot.Components.Extensions.Worker.SetWorkerPoolNameToOperatingSystemConfigsMap(b.Shoot.Components.Extensions.OperatingSystemConfig.WorkerPoolNameToOperatingSystemConfigsMap())

	if b.IsRestorePhase() {
		return b.Shoot.Components.Extensions.Worker.Restore(ctx, b.Shoot.GetShootState())
	}

	return b.Shoot.Components.Extensions.Worker.Deploy(ctx)
}

// WorkerPoolToNodesMap lists all the nodes with the given client in the shoot cluster. It returns a map whose key is
// the name of a worker pool and whose values are the corresponding nodes.
func WorkerPoolToNodesMap(ctx context.Context, shootClient client.Client) (map[string][]corev1.Node, error) {
	nodeList := &corev1.NodeList{}
	if err := shootClient.List(ctx, nodeList); err != nil {
		return nil, err
	}

	workerPoolToNodes := make(map[string][]corev1.Node)
	for _, node := range nodeList.Items {
		if pool, ok := node.Labels[v1beta1constants.LabelWorkerPool]; ok {
			workerPoolToNodes[pool] = append(workerPoolToNodes[pool], node)
		}
	}

	return workerPoolToNodes, nil
}

// WorkerPoolToOperatingSystemConfigSecretMetaMap lists all the cloud-config secrets with the given client in the shoot cluster.
// It returns a map whose key is the name of a worker pool and whose values are the corresponding metadata of the
// cloud-config script stored inside the secret's data.
func WorkerPoolToOperatingSystemConfigSecretMetaMap(ctx context.Context, shootClient client.Client, roleValue string) (map[string]metav1.ObjectMeta, error) {
	secretList := &corev1.SecretList{}
	if err := shootClient.List(ctx, secretList, client.InNamespace(metav1.NamespaceSystem), client.MatchingLabels{v1beta1constants.GardenRole: roleValue}); err != nil {
		return nil, err
	}

	workerPoolToCloudConfigSecretMeta := make(map[string]metav1.ObjectMeta, len(secretList.Items))
	for _, secret := range secretList.Items {
		if poolName, ok := secret.Labels[v1beta1constants.LabelWorkerPool]; ok {
			workerPoolToCloudConfigSecretMeta[poolName] = secret.ObjectMeta
		}
	}

	return workerPoolToCloudConfigSecretMeta, nil
}

// OperatingSystemConfigUpdatedForAllWorkerPools checks if all the nodes for all the provided worker pools have successfully
// applied the desired version of their cloud-config user data.
func OperatingSystemConfigUpdatedForAllWorkerPools(
	workers []gardencorev1beta1.Worker,
	workerPoolToNodes map[string][]corev1.Node,
	workerPoolToOperatingSystemConfigSecretMeta map[string]metav1.ObjectMeta,
) error {
	var result error

	for _, worker := range workers {
		secretMeta, ok := workerPoolToOperatingSystemConfigSecretMeta[worker.Name]
		if !ok {
			result = multierror.Append(result, fmt.Errorf("missing operating system config secret metadata for worker pool %q", worker.Name))
			continue
		}

		var (
			gardenerNodeAgentSecretName = secretMeta.Name
			secretChecksum              = secretMeta.Annotations[nodeagentconfigv1alpha1.AnnotationKeyChecksumDownloadedOperatingSystemConfig]
		)

		for _, node := range workerPoolToNodes[worker.Name] {
			if nodeToBeDeleted(node, gardenerNodeAgentSecretName) {
				continue
			}

			if nodeChecksum, ok := node.Annotations[nodeagentconfigv1alpha1.AnnotationKeyChecksumAppliedOperatingSystemConfig]; nodeChecksum != secretChecksum {
				if !ok {
					result = multierror.Append(result, fmt.Errorf("the last successfully applied operating system config on node %q hasn't been reported yet", node.Name))
				} else {
					result = multierror.Append(result, fmt.Errorf("the last successfully applied operating system config on node %q is outdated (current: %s, desired: %s)", node.Name, nodeChecksum, secretChecksum))
				}
			}
		}
	}

	return result
}

func nodeToBeDeleted(node corev1.Node, gardenerNodeAgentSecretName string) bool {
	if nodeTaintedForNoSchedule(node) {
		return true
	}

	if val, ok := node.Labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName]; ok {
		return val != gardenerNodeAgentSecretName
	}

	return false
}

func nodeTaintedForNoSchedule(node corev1.Node) bool {
	// mcmPreferNoScheduleKey is used to identify machineSet nodes on which PreferNoSchedule taint is added on
	// older machineSets during a rolling update
	const mcmPreferNoScheduleKey = "deployment.machine.sapcloud.io/prefer-no-schedule"

	for _, taint := range node.Spec.Taints {
		if taint.Key == mcmPreferNoScheduleKey && taint.Effect == corev1.TaintEffectPreferNoSchedule {
			return true
		}
	}

	return false
}

// exposed for testing
var (
	// IntervalWaitOperatingSystemConfigUpdated is the interval when waiting until the operating system config was
	// updated for all worker pools.
	IntervalWaitOperatingSystemConfigUpdated = 5 * time.Second
	// GetTimeoutWaitOperatingSystemConfigUpdated retrieves the timeout when waiting until the operating system config
	// was updated for all worker pools.
	GetTimeoutWaitOperatingSystemConfigUpdated = getTimeoutWaitOperatingSystemConfigUpdated
)

func getTimeoutWaitOperatingSystemConfigUpdated(shoot *shootpkg.Shoot) time.Duration {
	return shoot.OSCSyncJitterPeriod.Duration + controllerutils.DefaultReconciliationTimeout
}

// WaitUntilOperatingSystemConfigUpdatedForAllWorkerPools waits for a maximum of 6 minutes until all the nodes for all
// the worker pools in the Shoot have successfully applied the desired version of their operating system config.
func (b *Botanist) WaitUntilOperatingSystemConfigUpdatedForAllWorkerPools(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, GetTimeoutWaitOperatingSystemConfigUpdated(b.Shoot))
	defer cancel()

	if err := managedresources.WaitUntilHealthy(timeoutCtx, b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, GardenerNodeAgentManagedResourceName); err != nil {
		return fmt.Errorf("the operating system configs for the worker nodes were not populated yet: %w", err)
	}

	timeoutCtx2, cancel2 := context.WithTimeout(ctx, GetTimeoutWaitOperatingSystemConfigUpdated(b.Shoot))
	defer cancel2()

	return retry.Until(timeoutCtx2, IntervalWaitOperatingSystemConfigUpdated, func(ctx context.Context) (done bool, err error) {
		workerPoolToNodes, err := WorkerPoolToNodesMap(ctx, b.ShootClientSet.Client())
		if err != nil {
			return retry.SevereError(err)
		}

		workerPoolToOperatingSystemConfigSecretMeta, err := WorkerPoolToOperatingSystemConfigSecretMetaMap(ctx, b.ShootClientSet.Client(), v1beta1constants.GardenRoleOperatingSystemConfig)
		if err != nil {
			return retry.SevereError(err)
		}

		if err := OperatingSystemConfigUpdatedForAllWorkerPools(b.Shoot.GetInfo().Spec.Provider.Workers, workerPoolToNodes, workerPoolToOperatingSystemConfigSecretMeta); err != nil {
			return retry.MinorError(err)
		}

		return retry.Ok()
	})
}
