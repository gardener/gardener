// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubecontrollermanager

import (
	"context"
	"fmt"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (k *kubeControllerManager) Wait(_ context.Context) error        { return nil }
func (k *kubeControllerManager) WaitCleanup(_ context.Context) error { return nil }

func (k *kubeControllerManager) WaitForControllerToBeActive(ctx context.Context) error {
	const (
		pollInterval = 5 * time.Second
		pollTimeout  = 90 * time.Second
	)

	// Check whether the kube-controller-manager deployment exists
	if err := k.seedClient.Get(ctx, kutil.Key(k.namespace, v1beta1constants.DeploymentNameKubeControllerManager), &appsv1.Deployment{}); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("kube controller manager deployment not found: %v", err)
		}
		return err
	}

	return retry.UntilTimeout(ctx, pollInterval, pollTimeout, func(ctx context.Context) (done bool, err error) {
		podList := &corev1.PodList{}
		err = k.seedClient.List(ctx, podList,
			client.InNamespace(k.namespace),
			client.MatchingLabels(map[string]string{
				v1beta1constants.LabelApp:  v1beta1constants.LabelKubernetes,
				v1beta1constants.LabelRole: LabelRole,
			}))
		if err != nil {
			return retry.SevereError(fmt.Errorf("could not check whether controller %s is active: %w", v1beta1constants.DeploymentNameKubeControllerManager, err))
		}

		// Check that one replica of the controller exists.
		if len(podList.Items) != 1 {
			k.log.Infof("Waiting for %s to have exactly one replica", v1beta1constants.DeploymentNameKubeControllerManager)
			return retry.MinorError(fmt.Errorf("controller %s is not active", v1beta1constants.DeploymentNameKubeControllerManager))
		}

		// Check that the existing replica is not getting deleted.
		if podList.Items[0].DeletionTimestamp != nil {
			k.log.Infof("Waiting for a new replica of %s", v1beta1constants.DeploymentNameKubeControllerManager)
			return retry.MinorError(fmt.Errorf("controller %s is not active", v1beta1constants.DeploymentNameKubeControllerManager))
		}

		// Check if the controller is active by reading its leader election record.
		lock := resourcelock.EndpointsResourceLock
		if versionConstraintK8sGreaterEqual120.Check(k.version) {
			lock = resourcelock.LeasesResourceLock
		}

		leaderElectionRecord, err := kutil.ReadLeaderElectionRecord(ctx, k.shootClient, lock, metav1.NamespaceSystem, v1beta1constants.DeploymentNameKubeControllerManager)
		if err != nil {
			return retry.SevereError(fmt.Errorf("could not check whether controller %s is active: %w", v1beta1constants.DeploymentNameKubeControllerManager, err))
		}

		lastRenew := metav1.Now().UTC().Sub(leaderElectionRecord.RenewTime.Time.UTC())
		defaultRenewalTime := 2 * time.Second
		if lastRenew <= defaultRenewalTime*2 {
			return retry.Ok()
		}

		k.log.Infof("Waiting for %s to be active", v1beta1constants.DeploymentNameKubeControllerManager)
		return retry.MinorError(fmt.Errorf("controller %s is not active", v1beta1constants.DeploymentNameKubeControllerManager))
	})
}
