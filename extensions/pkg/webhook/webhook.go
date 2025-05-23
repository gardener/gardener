// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"fmt"
	"net/http"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// ActionMutating defines the webhook as a mutating webhook.
	ActionMutating = "mutating"
	// ActionValidating defines the webhook as a validating webhook.
	ActionValidating = "validating"
	// TargetSeed defines that the webhook is to be installed in the seed.
	TargetSeed = "seed"
	// TargetShoot defines that the webhook is to be installed in the shoot.
	TargetShoot = "shoot"
)

// Webhook is the specification of a webhook.
type Webhook struct {
	Action            string
	Name              string
	Provider          string
	Path              string
	Target            string
	Types             []Type
	Webhook           *admission.Webhook
	Handler           http.Handler
	NamespaceSelector *metav1.LabelSelector
	ObjectSelector    *metav1.LabelSelector
	FailurePolicy     *admissionregistrationv1.FailurePolicyType
	TimeoutSeconds    *int32
}

// Type contains information about the Kubernetes object types and subresources the webhook acts upon.
type Type struct {
	Obj         client.Object
	Subresource *string
}

// Args contains Webhook creation arguments.
type Args struct {
	Provider          string
	Name              string
	Path              string
	Target            string
	NamespaceSelector *metav1.LabelSelector
	ObjectSelector    *metav1.LabelSelector
	Predicates        []predicate.Predicate
	Validators        map[Validator][]Type
	Mutators          map[Mutator][]Type
}

// New creates a new Webhook with the given args.
func New(mgr manager.Manager, args Args) (*Webhook, error) {
	var (
		objTypes []Type

		logger  = log.Log.WithName(args.Name).WithValues("provider", args.Provider)
		builder = NewBuilder(mgr, logger)
	)

	var actionType string
	if len(args.Mutators) > 0 {
		actionType = ActionMutating
	}
	if len(args.Validators) > 0 {
		// Mutators and validators must not be configured at the same time because mutators are supposed to be placed in
		// a 'MutatingWebhookConfiguration' while validators should reside in a 'ValidatingWebhookConfiguration'.
		if actionType == ActionMutating {
			return nil, fmt.Errorf("failed to create webhook because a mixture of mutating and validating functions is not permitted")
		}
		actionType = ActionValidating
	}

	for mut, objs := range args.Mutators {
		builder.WithMutator(mut, objs...)
		objTypes = append(objTypes, objs...)
	}

	for val, objs := range args.Validators {
		builder.WithValidator(val, objs...)
		objTypes = append(objTypes, objs...)
	}

	builder.WithPredicates(args.Predicates...)

	handler, err := builder.Build()
	if err != nil {
		return nil, err
	}

	// Create webhook
	logger.Info("Creating webhook")

	return &Webhook{
		Name:              args.Name,
		Provider:          args.Provider,
		Action:            actionType,
		NamespaceSelector: args.NamespaceSelector,
		ObjectSelector:    args.ObjectSelector,
		Path:              args.Path,
		Target:            args.Target,
		Webhook:           &admission.Webhook{Handler: handler, RecoverPanic: ptr.To(true)},
		Types:             objTypes,
	}, nil
}
