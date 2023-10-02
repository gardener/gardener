// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package operator

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
