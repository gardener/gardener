// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// SeedIsGarden returns 'true' if the cluster is registered as a Garden cluster.
func SeedIsGarden(ctx context.Context, seedClient client.Reader) (bool, error) {
	seedIsGarden, err := kubernetesutils.ResourcesExist(ctx, seedClient, &operatorv1alpha1.GardenList{}, operatorclient.RuntimeScheme)
	if err != nil {
		if !meta.IsNoMatchError(err) {
			return false, err
		}
		seedIsGarden = false
	}
	return seedIsGarden, nil
}
