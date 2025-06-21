// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers

import (
	"context"
	"fmt"

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
	return &kubernetesErrors{
		checkFunc: apierrors.IsNotFound,
		message:   "NotFound",
	}
}

// BeNotRegisteredError checks if error is NotRegistered.
func BeNotRegisteredError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: runtime.IsNotRegisteredError,
		message:   "NotRegistered",
	}
}

// BeAlreadyExistsError checks if error is AlreadyExists.
func BeAlreadyExistsError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: apierrors.IsAlreadyExists,
		message:   "AlreadyExists",
	}
}

// BeForbiddenError checks if error is Forbidden.
func BeForbiddenError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: apierrors.IsForbidden,
		message:   "Forbidden",
	}
}

// BeBadRequestError checks if error is BadRequest.
func BeBadRequestError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: apierrors.IsBadRequest,
		message:   "BadRequest",
	}
}

// BeNoMatchError checks if error is a NoMatchError.
func BeNoMatchError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: meta.IsNoMatchError,
		message:   "NoMatch",
	}
}

// BeMissingKindError checks if error is a MissingKindError.
func BeMissingKindError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: runtime.IsMissingKind,
		message:   "Object 'Kind' is missing",
	}
}

// BeInternalServerError checks if error is a InternalServerError.
func BeInternalServerError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: apierrors.IsInternalError,
		message:   "",
	}
}

// BeInvalidError checks if error is an InvalidError.
func BeInvalidError() types.GomegaMatcher {
	return &kubernetesErrors{
		checkFunc: apierrors.IsInvalid,
		message:   "Invalid",
	}
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
func NewManagedResourceContainsObjectsMatcher(c client.Client) func(...client.Object) types.GomegaMatcher {
	return func(objs ...client.Object) types.GomegaMatcher {
		return &managedResourceObjectsMatcher{
			ctx:             context.Background(),
			client:          c,
			decoder:         serializer.NewCodecFactory(c.Scheme()).UniversalDeserializer(),
			expectedObjects: expectedObjects(objs, c.Scheme()),
		}
	}
}

// NewManagedResourceConsistOfObjectsMatcher returns a function for a matcher that checks
// if the exact list of given objects are handled by the given managed resource.
// Any extra objects found through the ManagedResource let the matcher fail.
// It is expected that the data keys of referenced secret(s) follow the semantics of `managedresources.Registry`.
func NewManagedResourceConsistOfObjectsMatcher(c client.Client) func(...client.Object) types.GomegaMatcher {
	return func(objs ...client.Object) types.GomegaMatcher {
		return &managedResourceObjectsMatcher{
			ctx:               context.Background(),
			client:            c,
			decoder:           serializer.NewCodecFactory(c.Scheme()).UniversalDeserializer(),
			expectedObjects:   expectedObjects(objs, c.Scheme()),
			extraObjectsCheck: true,
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

// ContainAnyOf returns a Gomega matcher that checks if the actual slice does contain at least of the wanted strings.
func ContainAnyOf(wanted ...string) types.GomegaMatcher {
	return &containAnyMatcher{wanted: wanted}
}

type containAnyMatcher struct {
	wanted []string
}

// Match checks if the actual slice does not contain any of the forbidden strings.
func (matcher *containAnyMatcher) Match(actual interface{}) (bool, error) {
	actualSlice, ok := actual.([]string)
	if !ok {
		return false, fmt.Errorf("ContainAnyOf expects a []string")
	}
	for _, wants := range matcher.wanted {
		for _, val := range actualSlice {
			if val == wants {
				return true, nil
			}
		}
	}
	return false, nil
}

// FailureMessage returns a formatted failure message if the actual slice contains none of the wanted strings.
func (matcher *containAnyMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected\n\t%#v\nto contain any of\n\t%#v\n", actual, matcher.wanted)
}

// NegatedFailureMessage returns a message indicating that there is at least one of the strings in the actual slice.
func (matcher *containAnyMatcher) NegatedFailureMessage(actual interface{}) string {
	actualSlice, ok := actual.([]string)
	if !ok {
		return fmt.Sprintf("ContainAnyOf expects a []string, but got %#v", actual)
	}
	foundStrings := []string{}
	for _, wants := range matcher.wanted {
		for _, val := range actualSlice {
			if val == wants {
				foundStrings = append(foundStrings, wants)
			}
		}
	}
	return fmt.Sprintf("Expected\n\t%#v\nto not contain any of\n\t%#v, but found \n\t%#v\n", actualSlice, matcher.wanted, foundStrings)
}
