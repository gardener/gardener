// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
)

// ClusterToWorkerMapper returns a mapper that returns requests for Worker whose
// referenced clusters have been modified.
func ClusterToWorkerMapper(reader client.Reader, predicates []predicate.Predicate) handler.MapFunc {
	return mapper.ClusterToObjectMapper(reader, func() client.ObjectList { return &extensionsv1alpha1.WorkerList{} }, predicates)
}
