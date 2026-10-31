// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// EnsureNoGardenletOrOperator is a safety check that prevents operators from accidentally executing a command that
// deploys extensions and other components to a bootstrap cluster (e.g., `gardenadm bootstrap`) on a cluster that is
// already used as a runtime cluster with gardener-operator or as a seed cluster. Doing so would lead to conflicts when
// the command starts deploying components like provider extensions.
func EnsureNoGardenletOrOperator(ctx context.Context, c client.Reader, command string) error {
	for _, key := range []client.ObjectKey{
		{Namespace: v1beta1constants.GardenNamespace, Name: "gardener-operator"},
		{Namespace: v1beta1constants.GardenNamespace, Name: "gardenlet"},
	} {
		if err := c.Get(ctx, key, &appsv1.Deployment{}); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed checking if %q deployment exists: %w", key, err)
		}

		return fmt.Errorf("deployment %q exists on the targeted cluster. "+
			"`gardenadm %s` does not support targeting a cluster that is already used as a runtime cluster with gardener-operator or as a seed cluster. "+
			"Please consult the gardenadm documentation", key, command)
	}

	return nil
}
