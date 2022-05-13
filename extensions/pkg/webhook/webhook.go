// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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
	Name           string
	Provider       string
	Path           string
	Target         string
	Types          []Type
	Webhook        *admission.Webhook
	Handler        http.Handler
	Selector       *metav1.LabelSelector
	ObjectSelector *metav1.LabelSelector
	FailurePolicy  *admissionregistrationv1.FailurePolicyType
	TimeoutSeconds *int32
}

// Type contains information about the Kubernetes object types and subresources the webhook acts upon.
type Type struct {
	Obj         client.Object
	Subresource *string
}

// Args contains Webhook creation arguments.
type Args struct {
	Provider   string
	Name       string
	Path       string
	Predicates []predicate.Predicate
	Validators map[Validator][]Type
	Mutators   map[Mutator][]Type
}

// New creates a new Webhook with the given args.
func New(mgr manager.Manager, args Args) (*Webhook, error) {
	logger := log.Log.WithName(args.Name).WithValues("provider", args.Provider)

	// Create handler
	builder := NewBuilder(mgr, logger)

	for val, objs := range args.Validators {
		builder.WithValidator(val, objs...)
	}

	for mut, objs := range args.Mutators {
		builder.WithMutator(mut, objs...)
	}

	builder.WithPredicates(args.Predicates...)

	handler, err := builder.Build()
	if err != nil {
		return nil, err
	}

	// Create webhook
	logger.Info("Creating webhook")

	return &Webhook{
		Path:    args.Path,
		Webhook: &admission.Webhook{Handler: handler},
	}, nil
}
