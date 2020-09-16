// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerruntime

import (
	extensionshandler "github.com/gardener/gardener/extensions/pkg/handler"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ClusterToContainerResourceMapper returns a mapper that returns requests for Container resource whose
// referenced clusters have been modified.
func ClusterToContainerResourceMapper(predicates ...predicate.Predicate) handler.Mapper {
	return extensionshandler.ClusterToObjectMapper(func() runtime.Object { return &extensionsv1alpha1.ContainerRuntimeList{} }, predicates)
}
