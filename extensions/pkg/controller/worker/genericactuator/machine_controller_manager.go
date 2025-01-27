// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package genericactuator

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func scaleMachineControllerManager(ctx context.Context, logger logr.Logger, cl client.Client, worker *extensionsv1alpha1.Worker, replicas int32) error {
	logger.Info("Scaling machine-controller-manager", "replicas", replicas)
	return client.IgnoreNotFound(kubernetesutils.ScaleDeployment(ctx, cl, client.ObjectKey{Namespace: worker.Namespace, Name: v1beta1constants.DeploymentNameMachineControllerManager}, replicas))
}
