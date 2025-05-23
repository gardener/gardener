// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerruntime

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
)

// ClusterToContainerResourceMapper returns a mapper that returns requests for Container resource whose referenced
// clusters have been modified.
func ClusterToContainerResourceMapper(reader client.Reader, predicates ...predicate.Predicate) handler.MapFunc {
	return mapper.ClusterToObjectMapper(reader, func() client.ObjectList { return &extensionsv1alpha1.ContainerRuntimeList{} }, predicates)
}
