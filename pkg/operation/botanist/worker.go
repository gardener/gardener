// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver"
	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/executor"
	"github.com/gardener/gardener/pkg/component/extensions/worker"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
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
			Namespace:           b.Shoot.SeedNamespace,
			Name:                b.Shoot.GetInfo().Name,
			Type:                b.Shoot.GetInfo().Spec.Provider.Type,
			Region:              b.Shoot.GetInfo().Spec.Region,
			Workers:             b.Shoot.GetInfo().Spec.Provider.Workers,
			KubernetesVersion:   b.Shoot.KubernetesVersion,
			MachineTypes:        b.Shoot.CloudProfile.Spec.MachineTypes,
			NodeLocalDNSEnabled: v1beta1helper.IsNodeLocalDNSEnabled(b.Shoot.GetInfo().Spec.SystemComponents, b.Shoot.GetInfo().Annotations),
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
	b.Shoot.Components.Extensions.Worker.SetWorkerNameToOperatingSystemConfigsMap(b.Shoot.Components.Extensions.OperatingSystemConfig.WorkerNameToOperatingSystemConfigsMap())

	if b.isRestorePhase() {
		return b.Shoot.Components.Extensions.Worker.Restore(ctx, b.GetShootState())
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

// WorkerPoolToCloudConfigSecretMetaMap lists all the cloud-config secrets with the given client in the shoot cluster.
// It returns a map whose key is the name of a worker pool and whose values are the corresponding metadata of the
// cloud-config script stored inside the secret's data.
func WorkerPoolToCloudConfigSecretMetaMap(ctx context.Context, shootClient client.Client) (map[string]metav1.ObjectMeta, error) {
	secretList := &corev1.SecretList{}
	if err := shootClient.List(ctx, secretList, client.InNamespace(metav1.NamespaceSystem), client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleCloudConfig}); err != nil {
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

// CloudConfigUpdatedForAllWorkerPools checks if all the nodes for all the provided worker pools have successfully
// applied the desired version of their cloud-config user data.
func CloudConfigUpdatedForAllWorkerPools(
	workers []gardencorev1beta1.Worker,
	workerPoolToNodes map[string][]corev1.Node,
	workerPoolToCloudConfigSecretMeta map[string]metav1.ObjectMeta,
) error {
	var result error

	for _, worker := range workers {
		secretMeta, ok := workerPoolToCloudConfigSecretMeta[worker.Name]
		if !ok {
			result = multierror.Append(result, fmt.Errorf("missing cloud config secret metadata for worker pool %q", worker.Name))
			continue
		}

		var (
			secretOSCKey   = secretMeta.Name
			secretChecksum = secretMeta.Annotations[downloader.AnnotationKeyChecksum]
		)

		for _, node := range workerPoolToNodes[worker.Name] {
			nodeWillBeDeleted, err := nodeToBeDeleted(node, secretOSCKey)
			if err != nil {
				result = multierror.Append(result, fmt.Errorf("failed checking whether node %q will be deleted: %w", node.Name, err))
				continue
			}

			if nodeWillBeDeleted {
				continue
			}

			if nodeChecksum, ok := node.Annotations[executor.AnnotationKeyChecksum]; ok && nodeChecksum != secretChecksum {
				result = multierror.Append(result, fmt.Errorf("the last successfully applied cloud config on node %q is outdated (current: %s, desired: %s)", node.Name, nodeChecksum, secretChecksum))
			}
		}
	}

	return result
}

func nodeToBeDeleted(node corev1.Node, secretOSCKey string) (bool, error) {
	if nodeTaintedForNoSchedule(node) {
		return true, nil
	}
	return nodeOSCKeyDifferentFromSecretOSCKey(node, secretOSCKey)
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

func nodeOSCKeyDifferentFromSecretOSCKey(node corev1.Node, secretOSCKey string) (bool, error) {
	kubernetesVersion, err := semver.NewVersion(node.Labels[v1beta1constants.LabelWorkerKubernetesVersion])
	if err != nil {
		return false, fmt.Errorf("failed parsing Kubernetes version to semver for node %q: %w", node.Name, err)
	}

	var criConfig *gardencorev1beta1.CRI
	if v, ok := node.Labels[extensionsv1alpha1.CRINameWorkerLabel]; ok {
		criConfig = &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRIName(v)}
	}

	return operatingsystemconfig.Key(node.Labels[v1beta1constants.LabelWorkerPool], kubernetesVersion, criConfig) != secretOSCKey, nil
}

// exposed for testing
var (
	// IntervalWaitCloudConfigUpdated is the interval when waiting until the cloud config was updated for all worker pools.
	IntervalWaitCloudConfigUpdated = 5 * time.Second
	// GetTimeoutWaitCloudConfigUpdated retrieves the timeout when waiting until the cloud config was updated for all worker pools.
	GetTimeoutWaitCloudConfigUpdated = getTimeoutWaitCloudConfigUpdated
)

func getTimeoutWaitCloudConfigUpdated(shoot *shootpkg.Shoot) time.Duration {
	return downloader.UnitRestartSeconds*time.Second*2 + time.Duration(shoot.CloudConfigExecutionMaxDelaySeconds)*time.Second
}

// WaitUntilCloudConfigUpdatedForAllWorkerPools waits for a maximum of 6 minutes until all the nodes for all the worker
// pools in the Shoot have successfully applied the desired version of their cloud-config user data.
func (b *Botanist) WaitUntilCloudConfigUpdatedForAllWorkerPools(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, GetTimeoutWaitCloudConfigUpdated(b.Shoot))
	defer cancel()

	if err := managedresources.WaitUntilHealthy(timeoutCtx, b.SeedClientSet.Client(), b.Shoot.SeedNamespace, CloudConfigExecutionManagedResourceName); err != nil {
		return fmt.Errorf("the cloud-config user data scripts for the worker nodes were not populated yet: %w", err)
	}

	timeoutCtx2, cancel2 := context.WithTimeout(ctx, GetTimeoutWaitCloudConfigUpdated(b.Shoot))
	defer cancel2()

	return retry.Until(timeoutCtx2, IntervalWaitCloudConfigUpdated, func(ctx context.Context) (done bool, err error) {
		workerPoolToNodes, err := WorkerPoolToNodesMap(ctx, b.ShootClientSet.Client())
		if err != nil {
			return retry.SevereError(err)
		}

		workerPoolToCloudConfigSecretMeta, err := WorkerPoolToCloudConfigSecretMetaMap(ctx, b.ShootClientSet.Client())
		if err != nil {
			return retry.SevereError(err)
		}

		if err := CloudConfigUpdatedForAllWorkerPools(b.Shoot.GetInfo().Spec.Provider.Workers, workerPoolToNodes, workerPoolToCloudConfigSecretMeta); err != nil {
			return retry.MinorError(err)
		}

		return retry.Ok()
	})
}
