// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package predicate

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// IsInGardenNamespacePredicate is a predicate which returns true when the provided object is in the 'garden' namespace.
var IsInGardenNamespacePredicate = predicate.NewPredicateFuncs(func(obj client.Object) bool {
	return obj != nil && obj.GetNamespace() == v1beta1constants.GardenNamespace
})

// ShootNotFailedPredicate returns a predicate which returns true when the Shoot's `.status.lastOperation.state` is not
// equals 'Failed'.
func ShootNotFailedPredicate(ctx context.Context, mgr manager.Manager) predicate.Predicate {
	return &shootNotFailedPredicate{
		ctx:    ctx,
		reader: mgr.GetClient(),
	}
}

type shootNotFailedPredicate struct {
	ctx    context.Context
	reader client.Reader
}

func (p *shootNotFailedPredicate) Create(e event.CreateEvent) bool {
	if e.Object == nil {
		return false
	}

	if !gardenerutils.IsShootNamespace(e.Object.GetNamespace()) {
		return true
	}

	cluster, err := extensionscontroller.GetCluster(p.ctx, p.reader, e.Object.GetNamespace())
	if err != nil {
		logger.Error(err, "Could not check if shoot is failed")
		return false
	}

	return !extensionscontroller.IsFailed(cluster)
}

func (p *shootNotFailedPredicate) Update(e event.UpdateEvent) bool {
	return p.Create(event.CreateEvent{Object: e.ObjectNew})
}

func (p *shootNotFailedPredicate) Delete(_ event.DeleteEvent) bool { return false }

func (p *shootNotFailedPredicate) Generic(_ event.GenericEvent) bool { return false }
