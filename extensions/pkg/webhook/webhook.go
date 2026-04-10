// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"fmt"
	"slices"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
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
	// Action defines whether this is a mutating or validating webhook (ActionMutating or ActionValidating).
	Action string
	// Name is the name of the webhook.
	Name string
	// Path is the endpoint webhook path.
	Path string
	// Target specifies where the webhook is to be installed (TargetSeed or TargetShoot).
	// For the garden use case, TargetSeed corresponds to a webhook in the runtime garden, whereas TargetShoot
	// is a webhook configured in the virtual garden cluster.
	Target string
	// Types contains the Kubernetes object types and subresources the webhook acts upon.
	Types []Type
	// Webhook is the controller-runtime webhook handler with panic recovery enabled.
	Webhook *admission.Webhook
	// NamespaceSelector selects namespaces for which the webhook is active.
	// If nil, the webhook is active for all namespaces.
	NamespaceSelector *metav1.LabelSelector
	// ObjectSelector selects objects for which the webhook is active based on labels.
	// If nil, the webhook is active for all objects matching the type.
	ObjectSelector *metav1.LabelSelector
	// FailurePolicy defines how unrecognized errors and timeout errors are handled.
	// If nil, defaults to the Kubernetes default failure policy.
	FailurePolicy *admissionregistrationv1.FailurePolicyType
	// TimeoutSeconds specifies the timeout for the webhook call.
	// If nil, defaults to the Kubernetes default timeout.
	TimeoutSeconds *int32
}

// Validator validates objects.
type Validator interface {
	Validate(ctx context.Context, new, old client.Object) error
}

// Mutator validates and if needed mutates objects.
type Mutator interface {
	// Mutate validates and if needed mutates the given object.
	// "old" is optional, and it must always be checked for nil.
	Mutate(ctx context.Context, new, old client.Object) error
}

// Type contains information about the Kubernetes object types and subresources the webhook acts upon.
type Type struct {
	Obj         client.Object
	Subresource *string
}

// Args contains Webhook creation arguments.
type Args struct {
	// Name is the name of the webhook.
	Name string
	// Path is the endpoint webhook path.
	Path string
	// Target specifies where the webhook is to be installed (TargetSeed or TargetShoot).
	// For the garden use case, TargetSeed corresponds to a webhook in the runtime garden, whereas TargetShoot
	// is a webhook configured in the virtual garden cluster.
	Target string
	// NamespaceSelector selects namespaces for which the webhook is active.
	// If nil, the webhook is active for all namespaces.
	NamespaceSelector *metav1.LabelSelector
	// ObjectSelector selects objects for which the webhook is active based on labels.
	// If nil, the webhook is active for all objects matching the type.
	ObjectSelector *metav1.LabelSelector
	// Predicates are additional filters for objects the webhook should process.
	Predicates []predicate.Predicate
	// Validators maps Validator implementations to the object types they validate.
	// Validators cannot be mixed with Mutators in the same webhook.
	Validators map[Validator][]Type
	// Mutators maps Mutator implementations to the object types they mutate.
	// Mutators cannot be mixed with Validators in the same webhook.
	Mutators map[Mutator][]Type
}

// New creates a new Webhook with the given args.
func New(mgr manager.Manager, args Args) (*Webhook, error) {
	var (
		objTypes []Type

		logger  = log.Log.WithName(args.Name)
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
		Action:            actionType,
		NamespaceSelector: args.NamespaceSelector,
		ObjectSelector:    args.ObjectSelector,
		Path:              args.Path,
		Target:            args.Target,
		Webhook:           &admission.Webhook{Handler: handler, RecoverPanic: ptr.To(true)},
		Types:             objTypes,
	}, nil
}

// BuildExtensionTypeNamespaceSelector returns a label selector which matches the 'extensions.gardener.cloud/<extension-type>' label.
// If the webhook is responsible for the 'garden' or 'seed' extension class, the 'garden' namespace is selected.
func BuildExtensionTypeNamespaceSelector(extensionType string, extensionClasses []extensionsv1alpha1.ExtensionClass) *metav1.LabelSelector {
	labelSelector := &metav1.LabelSelector{}

	if slices.Contains(extensionClasses, extensionsv1alpha1.ExtensionClassGarden) ||
		slices.Contains(extensionClasses, extensionsv1alpha1.ExtensionClassSeed) {
		labelSelector.MatchExpressions = append(labelSelector.MatchExpressions, metav1.LabelSelectorRequirement{
			Key:      corev1.LabelMetadataName,
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{v1beta1constants.GardenNamespace},
		})
	}

	if len(extensionClasses) == 0 || slices.Contains(extensionClasses, extensionsv1alpha1.ExtensionClassShoot) {
		labelSelector.MatchExpressions = append(labelSelector.MatchExpressions, metav1.LabelSelectorRequirement{
			Key:      v1beta1constants.LabelExtensionPrefix + extensionType,
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{"true"},
		})
	}

	return labelSelector
}
