// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"k8s.io/apimachinery/pkg/util/sets"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

type filter func(e Extension) bool

func all(_ Extension) bool {
	return true
}

func deployBeforeKubeAPIServer(e Extension) bool {
	if e.Lifecycle == nil || e.Lifecycle.Reconcile == nil {
		return false
	}
	return *e.Lifecycle.Reconcile == gardencorev1beta1.BeforeKubeAPIServer
}

func deployAfterKubeAPIServer(e Extension) bool {
	if e.Lifecycle == nil || e.Lifecycle.Reconcile == nil {
		return true
	}
	return *e.Lifecycle.Reconcile == gardencorev1beta1.AfterKubeAPIServer
}

func deployAfterWorker(e Extension) bool {
	if e.Lifecycle == nil || e.Lifecycle.Reconcile == nil {
		return false
	}
	return *e.Lifecycle.Reconcile == gardencorev1beta1.AfterWorker
}

func deleteBeforeKubeAPIServer(e Extension) bool {
	if e.Lifecycle == nil || e.Lifecycle.Delete == nil {
		return true
	}
	return *e.Lifecycle.Delete == gardencorev1beta1.BeforeKubeAPIServer
}

func deleteAfterKubeAPIServer(e Extension) bool {
	if e.Lifecycle == nil || e.Lifecycle.Delete == nil {
		return false
	}
	return *e.Lifecycle.Delete == gardencorev1beta1.AfterKubeAPIServer
}

func migrateBeforeKubeAPIServer(e Extension) bool {
	if e.Lifecycle == nil || e.Lifecycle.Migrate == nil {
		return true
	}
	return *e.Lifecycle.Migrate == gardencorev1beta1.BeforeKubeAPIServer
}

func migrateAfterKubeAPIServer(e Extension) bool {
	if e.Lifecycle == nil || e.Lifecycle.Migrate == nil {
		return false
	}
	return *e.Lifecycle.Migrate == gardencorev1beta1.AfterKubeAPIServer
}

func (e *extension) filterExtensions(f filter) sets.Set[string] {
	extensions := sets.New[string]()
	for _, ext := range e.values.Extensions {
		if f(ext) {
			extensions.Insert(ext.Spec.Type)
		}
	}
	return extensions
}
