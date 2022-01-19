// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package predicate

import (
	"context"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	contextutil "github.com/gardener/gardener/pkg/utils/context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// IsInGardenNamespacePredicate is a predicate which returns true when the provided object is in the 'garden' namespace.
var IsInGardenNamespacePredicate = predicate.NewPredicateFuncs(func(obj client.Object) bool {
	return obj != nil && obj.GetNamespace() == v1beta1constants.GardenNamespace
})

// ShootNotFailedPredicate returns a predicate which returns true when the Shoot's `.status.lastOperation.state` is not
// equals 'Failed'.
func ShootNotFailedPredicate() predicate.Predicate {
	return &shootNotFailedPredicate{}
}

type shootNotFailedPredicate struct {
	ctx    context.Context
	reader client.Reader
}

func (p *shootNotFailedPredicate) InjectStopChannel(stopChan <-chan struct{}) error {
	p.ctx = contextutil.FromStopChannel(stopChan)
	return nil
}

func (p *shootNotFailedPredicate) InjectClient(client client.Client) error {
	p.reader = client
	return nil
}

func (p *shootNotFailedPredicate) Create(e event.CreateEvent) bool {
	if e.Object == nil {
		return false
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
