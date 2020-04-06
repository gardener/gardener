// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package util

import (
	"context"

	"github.com/gardener/gardener/pkg/chartrenderer"
	gardenerkubernetes "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Secrets represents a set of secrets that can be deployed and deleted.
type Secrets interface {
	// Deploy generates and deploys the secrets into the given namespace, taking into account existing secrets.
	Deploy(context.Context, kubernetes.Interface, gardenerkubernetes.Interface, string) (map[string]*corev1.Secret, error)
	// Delete deletes the secrets from the given namespace.
	Delete(kubernetes.Interface, string) error
}

// Chart represents a Helm chart that can be applied and deleted.
type Chart interface {
	// Apply applies this chart in the given namespace using the given ChartApplier. Before applying the chart,
	// it collects its values, injecting images and merging the given values as needed.
	Apply(context.Context, gardenerkubernetes.ChartApplier, string, imagevector.ImageVector, string, string, map[string]interface{}) error
	// Render renders this chart in the given namespace using the given chartRenderer. Before rendering the chart,
	// it collects its values, injecting images and merging the given values as needed.
	Render(chartrenderer.Interface, string, imagevector.ImageVector, string, string, map[string]interface{}) (string, []byte, error)
	// Delete deletes this chart's objects from the given namespace.
	Delete(context.Context, client.Client, string) error
}
