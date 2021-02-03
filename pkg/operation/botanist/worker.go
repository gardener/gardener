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
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/worker"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/secrets"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultWorker creates the default deployer for the Worker custom resource.
func (b *Botanist) DefaultWorker(seedClient client.Client) worker.Interface {
	return worker.New(
		b.Logger,
		seedClient,
		&worker.Values{
			Namespace:         b.Shoot.SeedNamespace,
			Name:              b.Shoot.Info.Name,
			Type:              b.Shoot.Info.Spec.Provider.Type,
			Region:            b.Shoot.Info.Spec.Region,
			Workers:           b.Shoot.Info.Spec.Provider.Workers,
			KubernetesVersion: b.Shoot.KubernetesVersion,
		},
		worker.DefaultInterval,
		worker.DefaultSevereThreshold,
		worker.DefaultTimeout,
	)
}

// DeployWorker deploys the Worker custom resource and triggers the restore operation in case
// the Shoot is in the restore phase of the control plane migration
func (b *Botanist) DeployWorker(ctx context.Context) error {
	b.Shoot.Components.Extensions.Worker.SetSSHPublicKey(b.Secrets[v1beta1constants.SecretNameSSHKeyPair].Data[secrets.DataKeySSHAuthorizedKeys])
	b.Shoot.Components.Extensions.Worker.SetInfrastructureProviderStatus(&runtime.RawExtension{Raw: b.Shoot.InfrastructureStatus})
	b.Shoot.Components.Extensions.Worker.SetWorkerNameToOperatingSystemConfigsMap(b.Shoot.Components.Extensions.OperatingSystemConfig.WorkerNameToOperatingSystemConfigsMap())

	if b.isRestorePhase() {
		return b.Shoot.Components.Extensions.Worker.Restore(ctx, b.ShootState)
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

// WorkerPoolToCloudConfigSecretChecksumMap lists all the cloud-config secrets with the given client in the shoot
// cluster. It returns a map whose key is the name of a worker pool and whose values are the corresponding checksums of
// the cloud-config script stored inside the secret's data.
func WorkerPoolToCloudConfigSecretChecksumMap(ctx context.Context, shootClient client.Client) (map[string]string, error) {
	secretList := &corev1.SecretList{}
	if err := shootClient.List(ctx, secretList, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleCloudConfig}); err != nil {
		return nil, err
	}

	workerPoolToCloudConfigSecretChecksum := make(map[string]string, len(secretList.Items))
	for _, secret := range secretList.Items {
		var (
			poolName, ok1 = secret.Labels[v1beta1constants.LabelWorkerPool]
			checksum, ok2 = secret.Annotations[common.CloudConfigChecksumSecretAnnotation]
		)

		if ok1 && ok2 {
			workerPoolToCloudConfigSecretChecksum[poolName] = checksum
		}
	}

	return workerPoolToCloudConfigSecretChecksum, nil
}

// CloudConfigUpdatedForAllWorkerPools checks if all the nodes for all the provided worker pools have successfully
// applied the desired version of their cloud-config user data.
func CloudConfigUpdatedForAllWorkerPools(workers []gardencorev1beta1.Worker, workerPoolToNodes map[string][]corev1.Node, workerPoolToCloudConfigSecretChecksum map[string]string) error {
	var result error

	for _, worker := range workers {
		secretChecksum, ok := workerPoolToCloudConfigSecretChecksum[worker.Name]
		if !ok {
			// This is to ensure backwards-compatibility to not break existing clusters which don't have a secret
			// checksum label yet.
			continue
		}

		for _, node := range workerPoolToNodes[worker.Name] {
			if nodeChecksum, ok := node.Annotations[common.CloudConfigChecksumNodeAnnotation]; ok && nodeChecksum != secretChecksum {
				result = multierror.Append(result, fmt.Errorf("the last successfully applied cloud config on node %q is outdated (current: %s, desired: %s)", node.Name, nodeChecksum, secretChecksum))
			}
		}
	}

	return result
}

// exposed for testing
var (
	// IntervalWaitCloudConfigUpdated is the interval when waiting until the cloud config was updated for all worker pools.
	IntervalWaitCloudConfigUpdated = 5 * time.Second
	// TimeoutWaitCloudConfigUpdated is the timeout when waiting until the cloud config was updated for all worker pools.
	TimeoutWaitCloudConfigUpdated = 2 * time.Minute
)

// WaitUntilCloudConfigUpdatedForAllWorkerPools waits for a maximum 2 minutes until all the nodes for all the worker
// pools in the Shoot have successfully applied the desired version of their cloud-config user data.
func (b *Botanist) WaitUntilCloudConfigUpdatedForAllWorkerPools(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitCloudConfigUpdated)
	defer cancel()

	if err := managedresources.WaitUntilManagedResourceHealthy(timeoutCtx, b.K8sSeedClient.Client(), b.Shoot.SeedNamespace, CloudConfigExecutionManagedResourceName); err != nil {
		return errors.Wrapf(err, "the cloud-config user data scripts for the worker nodes were not populated yet")
	}

	timeoutCtx2, cancel2 := context.WithTimeout(ctx, TimeoutWaitCloudConfigUpdated)
	defer cancel2()

	return retry.Until(timeoutCtx2, IntervalWaitCloudConfigUpdated, func(ctx context.Context) (done bool, err error) {
		workerPoolToNodes, err := WorkerPoolToNodesMap(ctx, b.K8sShootClient.Client())
		if err != nil {
			return retry.SevereError(err)
		}

		workerPoolToCloudConfigSecretChecksum, err := WorkerPoolToCloudConfigSecretChecksumMap(ctx, b.K8sShootClient.Client())
		if err != nil {
			return retry.SevereError(err)
		}

		if err := CloudConfigUpdatedForAllWorkerPools(b.Shoot.Info.Spec.Provider.Workers, workerPoolToNodes, workerPoolToCloudConfigSecretChecksum); err != nil {
			return retry.MinorError(err)
		}

		return retry.Ok()
	})
}
