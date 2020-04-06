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

package webhook

import (
	"net/http"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// TargetSeed defines that the webhook is to be installed in the seed.
	TargetSeed = "seed"
	// TargetShoot defines that the webhook is to be installed in the shoot.
	TargetShoot = "shoot"
)

// Webhook is the specification of a webhook.
type Webhook struct {
	Name     string
	Kind     string
	Provider string
	Path     string
	Target   string
	Types    []runtime.Object
	Webhook  *admission.Webhook
	Handler  http.Handler
	Selector *metav1.LabelSelector
}

// FactoryAggregator aggregates various Factory functions.
type FactoryAggregator []func(manager.Manager) (*Webhook, error)

// NewFactoryAggregator creates a new FactoryAggregator and registers the given functions.
func NewFactoryAggregator(m []func(manager.Manager) (*Webhook, error)) FactoryAggregator {
	builder := FactoryAggregator{}

	for _, f := range m {
		builder.Register(f)
	}

	return builder
}

// Register registers the given functions in this builder.
func (a *FactoryAggregator) Register(f func(manager.Manager) (*Webhook, error)) {
	*a = append(*a, f)
}

// Webhooks calls all factories with the given managers and returns all created webhooks.
// As soon as there is an error creating a webhook, the error is returned immediately.
func (a *FactoryAggregator) Webhooks(mgr manager.Manager) ([]*Webhook, error) {
	webhooks := make([]*Webhook, 0, len(*a))

	for _, f := range *a {
		wh, err := f(mgr)
		if err != nil {
			return nil, err
		}

		webhooks = append(webhooks, wh)
	}

	return webhooks, nil
}

// buildTypesMap builds a map of the given types keyed by their GroupVersionKind, using the scheme from the given Manager.
func buildTypesMap(mgr manager.Manager, types []runtime.Object) (map[metav1.GroupVersionKind]runtime.Object, error) {
	typesMap := make(map[metav1.GroupVersionKind]runtime.Object)
	for _, t := range types {
		// Get GVK from the type
		gvk, err := apiutil.GVKForObject(t, mgr.GetScheme())
		if err != nil {
			return nil, errors.Wrapf(err, "could not get GroupVersionKind from object %v", t)
		}

		// Add the type to the types map
		typesMap[metav1.GroupVersionKind(gvk)] = t
	}
	return typesMap, nil
}
