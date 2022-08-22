// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/executor"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/worker"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultWorker creates the default deployer for the Worker custom resource.
func (b *Botanist) DefaultWorker() worker.Interface {
	return worker.New(
		b.Logger,
		b.K8sSeedClient.Client(),
		&worker.Values{
			Namespace:           b.Shoot.SeedNamespace,
			Name:                b.Shoot.GetInfo().Name,
			Type:                b.Shoot.GetInfo().Spec.Provider.Type,
			Region:              b.Shoot.GetInfo().Spec.Region,
			Workers:             b.Shoot.GetInfo().Spec.Provider.Workers,
			KubernetesVersion:   b.Shoot.KubernetesVersion,
			MachineTypes:        b.Shoot.CloudProfile.Spec.MachineTypes,
			NodeLocalDNSENabled: v1beta1helper.IsNodeLocalDNSEnabled(b.Shoot.GetInfo().Spec.SystemComponents, b.Shoot.GetInfo().Annotations),
		},
		worker.DefaultInterval,
		worker.DefaultSevereThreshold,
		worker.DefaultTimeout,
	)
}

// DeployWorker deploys the Worker custom resource and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration
func (b *Botanist) DeployWorker(ctx context.Context) error {
	sshKeypairSecret, found := b.SecretsManager.Get(v1beta1constants.SecretNameSSHKeyPair)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameSSHKeyPair)
	}
	b.Shoot.Components.Extensions.Worker.SetSSHPublicKey(sshKeypairSecret.Data[secrets.DataKeySSHAuthorizedKeys])
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
	if err := shootClient.List(ctx, secretList, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleCloudConfig}); err != nil {
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
			secretChecksum = secretMeta.Annotations[downloader.AnnotationKeyChecksum]
		)

		for _, node := range workerPoolToNodes[worker.Name] {
			if nodeToBeDeleted(node) {
				continue
			}

			if nodeChecksum, ok := node.Annotations[executor.AnnotationKeyChecksum]; ok && nodeChecksum != secretChecksum {
				result = multierror.Append(result, fmt.Errorf("the last successfully applied cloud config on node %q is outdated (current: %s, desired: %s)", node.Name, nodeChecksum, secretChecksum))
			}
		}
	}

	return result
}

const (
	// MCMPreferNoScheduleKey is used to identify machineSet nodes on which PreferNoSchedule taint is added on
	// older machineSets during a rolling update
	MCMPreferNoScheduleKey = "deployment.machine.sapcloud.io/prefer-no-schedule"
)

// nodeToBeDeleted checks if the MCM has set the node to be deleted.
func nodeToBeDeleted(node corev1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == MCMPreferNoScheduleKey && taint.Effect == corev1.TaintEffectPreferNoSchedule {
			return true
		}
	}

	return false
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

	if err := managedresources.WaitUntilHealthy(timeoutCtx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace, CloudConfigExecutionManagedResourceName); err != nil {
		return fmt.Errorf("the cloud-config user data scripts for the worker nodes were not populated yet: %w", err)
	}

	timeoutCtx2, cancel2 := context.WithTimeout(ctx, GetTimeoutWaitCloudConfigUpdated(b.Shoot))
	defer cancel2()

	return retry.Until(timeoutCtx2, IntervalWaitCloudConfigUpdated, func(ctx context.Context) (done bool, err error) {
		workerPoolToNodes, err := WorkerPoolToNodesMap(ctx, b.K8sShootClient.Client())
		if err != nil {
			return retry.SevereError(err)
		}

		workerPoolToCloudConfigSecretMeta, err := WorkerPoolToCloudConfigSecretMetaMap(ctx, b.K8sShootClient.Client())
		if err != nil {
			return retry.SevereError(err)
		}

		if err := CloudConfigUpdatedForAllWorkerPools(b.Shoot.GetInfo().Spec.Provider.Workers, workerPoolToNodes, workerPoolToCloudConfigSecretMeta); err != nil {
			return retry.MinorError(err)
		}

		return retry.Ok()
	})
}
