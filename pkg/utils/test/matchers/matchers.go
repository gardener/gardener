// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers

import (
	"context"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	// if CharactersAroundMismatchToInclude is too small, then format.MessageWithDiff will be unable to output our
	// mismatch message
	// set the variable in init func, otherwise the race detector will complain when matchers are used concurrently in
	// multiple goroutines
	if format.CharactersAroundMismatchToInclude < 50 {
		format.CharactersAroundMismatchToInclude = 50
	}
}

// DeepEqual returns a Gomega matcher which checks whether the expected object is deeply equal with the object it is
// being compared against.
func DeepEqual(expected any) types.GomegaMatcher {
	return newDeepEqualMatcher(expected)
}

// DeepDerivativeEqual is similar to DeepEqual except that unset fields in actual are
// ignored (not compared). This allows us to focus on the fields that matter to
// the semantic comparison.
func DeepDerivativeEqual(expected any) types.GomegaMatcher {
	return newDeepDerivativeMatcher(expected)
}

// BeNotFoundError checks if error is NotFound.
func BeNotFoundError() types.GomegaMatcher {
	return MatchError(apierrors.IsNotFound, "NotFound")
}

// BeNotRegisteredError checks if error is NotRegistered.
func BeNotRegisteredError() types.GomegaMatcher {
	return MatchError(runtime.IsNotRegisteredError, "NotRegistered")
}

// BeAlreadyExistsError checks if error is AlreadyExists.
func BeAlreadyExistsError() types.GomegaMatcher {
	return MatchError(apierrors.IsAlreadyExists, "AlreadyExists")
}

// BeForbiddenError checks if error is Forbidden.
func BeForbiddenError() types.GomegaMatcher {
	return MatchError(apierrors.IsForbidden, "Forbidden")
}

// BeBadRequestError checks if error is BadRequest.
func BeBadRequestError() types.GomegaMatcher {
	return MatchError(apierrors.IsBadRequest, "BadRequest")
}

// BeNoMatchError checks if error is a NoMatchError.
func BeNoMatchError() types.GomegaMatcher {
	return MatchError(meta.IsNoMatchError, "NoMatch")
}

// BeMissingKindError checks if error is a MissingKindError.
func BeMissingKindError() types.GomegaMatcher {
	return MatchError(runtime.IsMissingKind, "Object 'Kind' is missing")
}

// BeInternalServerError checks if error is a InternalServerError.
func BeInternalServerError() types.GomegaMatcher {
	return MatchError(apierrors.IsInternalError, "")
}

// BeInvalidError checks if error is an InvalidError.
func BeInvalidError() types.GomegaMatcher {
	return MatchError(apierrors.IsInvalid, "Invalid")
}

// ShareSameReferenceAs checks if objects shares the same underlying reference as the passed object.
// This can be used to check if maps or slices have the same underlying data store.
// Only objects that work for 'reflect.ValueOf(x).Pointer' can be compared.
func ShareSameReferenceAs(expected any) types.GomegaMatcher {
	return &referenceMatcher{
		expected: expected,
	}
}

// NewManagedResourceContainsObjectsMatcher returns a function for a matcher that checks
// if the given objects are handled by the given managed resource.
// It is expected that the data keys of referenced secret(s) follow the semantics of `managedresources.Registry`.
// It allows to pass additional options to the underlying comparison, e.g. to ignore certain fields.
func NewManagedResourceContainsObjectsMatcher(c client.Client, compareOptions ...cmp.Option) func(...client.Object) types.GomegaMatcher {
	return func(objs ...client.Object) types.GomegaMatcher {
		return &managedResourceObjectsMatcher{
			ctx:             context.Background(),
			client:          c,
			decoder:         serializer.NewCodecFactory(c.Scheme()).UniversalDeserializer(),
			expectedObjects: expectedObjects(objs, c.Scheme()),
			compareOptions:  compareOptions,
		}
	}
}

// NewManagedResourceConsistOfObjectsMatcher returns a function for a matcher that checks
// if the exact list of given objects are handled by the given managed resource.
// Any extra objects found through the ManagedResource let the matcher fail.
// It is expected that the data keys of referenced secret(s) follow the semantics of `managedresources.Registry`.
// It allows to pass additional options to the underlying comparison, e.g. to ignore certain fields.
func NewManagedResourceConsistOfObjectsMatcher(c client.Client, compareOptions ...cmp.Option) func(...client.Object) types.GomegaMatcher {
	return func(objs ...client.Object) types.GomegaMatcher {
		return &managedResourceObjectsMatcher{
			ctx:               context.Background(),
			client:            c,
			decoder:           serializer.NewCodecFactory(c.Scheme()).UniversalDeserializer(),
			expectedObjects:   expectedObjects(objs, c.Scheme()),
			extraObjectsCheck: true,
			compareOptions:    compareOptions,
		}
	}
}

func expectedObjects(objs []client.Object, scheme *runtime.Scheme) map[string]client.Object {
	objects := make(map[string]client.Object)
	for _, o := range objs {
		obj := o

		// Fill GVK information with the help of the scheme.
		// Type meta is usually not explicitly configured when working with Kubernetes Go structs.
		if obj.GetObjectKind().GroupVersionKind().Empty() {
			gvk, _, err := scheme.ObjectKinds(obj)
			if len(gvk) < 1 || err != nil {
				continue
			}
			obj.GetObjectKind().SetGroupVersionKind(gvk[0])
		}

		objects[objectKey(obj, scheme)] = obj
	}

	return objects
}

// ContainAnyOf returns a Gomega matcher that checks if the actual slice contains at least of the wanted strings.
func ContainAnyOf(wanted ...string) types.GomegaMatcher {
	var matchers []types.GomegaMatcher

	for _, element := range wanted {
		matchers = append(matchers, ContainElement(element))
	}

	return Or(matchers...)
}
