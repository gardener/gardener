// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers

import (
	"context"
	"fmt"

	"github.com/onsi/gomega/format"
	"golang.org/x/exp/maps"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

type managedResourceObjectsMatcher struct {
	ctx               context.Context
	client            client.Client
	decoder           runtime.Decoder
	expectedObjects   map[string]client.Object
	extraObjectsCheck bool

	extraObjects             []string
	missingObjects           []string
	mismatchExpectedToActual map[client.Object]client.Object
}

func (m *managedResourceObjectsMatcher) FailureMessage(actual any) string {
	return m.createMessage(actual, "not to be")
}

func (m *managedResourceObjectsMatcher) NegatedFailureMessage(actual any) string {
	return m.createMessage(actual, "to be")
}

func (m *managedResourceObjectsMatcher) createMessage(actual any, addition string) string {
	managedResource, ok := actual.(*resourcesv1alpha1.ManagedResource)
	if !ok {
		return fmt.Sprintf("expected *resourcesv1alpha1.ManagedResource.  got:\n%s", format.Object(actual, 1))
	}

	var message string

	switch {
	case len(m.mismatchExpectedToActual) > 0:
		message = fmt.Sprintf("Expected for ManagedResource %s/%s the following object mismatches %s found:\n", managedResource.Namespace, managedResource.Name, addition)
		for expected, actual := range m.mismatchExpectedToActual {
			message += format.Message(actual, "to equal", expected)
		}
	case len(m.missingObjects) > 0:
		message = fmt.Sprintf("Expected for ManagedResource %s/%s the following elements %s absent:\n", managedResource.Namespace, managedResource.Name, addition)
		for _, missingObject := range m.missingObjects {
			message += format.IndentString(missingObject, 2)
		}
	case len(m.extraObjects) > 0:
		message = fmt.Sprintf("Expected for ManagedResource %s/%s the following extra and unexpected elements %s found:\n", managedResource.Namespace, managedResource.Name, addition)
		for _, extraObject := range m.extraObjects {
			message += format.IndentString(extraObject, 2)
		}
	}

	return message
}

func (m *managedResourceObjectsMatcher) Match(actual any) (bool, error) {
	if actual == nil {
		return false, nil
	}

	managedResource, ok := actual.(*resourcesv1alpha1.ManagedResource)
	if !ok {
		return false, fmt.Errorf("expected *resourcesv1alpha1.ManagedResource.  got:\n%s", format.Object(actual, 1))
	}

	// Retrieve managed resource secrets and extract objects.
	availableObjects := make(map[string]client.Object)

	objectFromManagedResource, err := managedresources.GetObjects(m.ctx, m.client, managedResource.Namespace, managedResource.Name)
	if err != nil {
		return false, err
	}

	for _, obj := range objectFromManagedResource {
		availableObjects[objectKey(obj, m.client.Scheme())] = obj
	}

	// Use early returns for the following checks to not overwhelm Gomega output.
	m.mismatchExpectedToActual = findMismatchObjects(availableObjects, m.expectedObjects)
	if len(m.mismatchExpectedToActual) > 0 {
		return false, nil
	}

	m.missingObjects = findMissingObjects(availableObjects, m.expectedObjects)
	if len(m.missingObjects) > 0 {
		return false, nil
	}

	if m.extraObjectsCheck {
		m.extraObjects = findExtraObjects(availableObjects, m.expectedObjects)
		if len(m.extraObjects) > 0 {
			return false, nil
		}
	}

	return true, nil
}

func findMismatchObjects(availableObjects map[string]client.Object, expectedObjects map[string]client.Object) map[client.Object]client.Object {
	mismatches := make(map[client.Object]client.Object)

	for expectedObjKey, expectedObj := range expectedObjects {
		actualObject, ok := availableObjects[expectedObjKey]
		if ok && !apiequality.Semantic.DeepEqual(actualObject, expectedObj) {
			mismatches[expectedObj] = actualObject
		}
	}

	return mismatches
}

func findMissingObjects(availableObjects map[string]client.Object, expectedObjects map[string]client.Object) []string {
	return sets.New(maps.Keys(expectedObjects)...).Difference(sets.New(maps.Keys(availableObjects)...)).UnsortedList()
}

func findExtraObjects(availableObjects map[string]client.Object, expectedObjects map[string]client.Object) []string {
	return sets.New(maps.Keys(availableObjects)...).Difference(sets.New(maps.Keys(expectedObjects)...)).UnsortedList()
}

func objectKey(obj client.Object, scheme *runtime.Scheme) string {
	gvkStr := "unknown"
	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err == nil {
		gvkStr = gvk.String()
	}

	return fmt.Sprintf("%s__%s__%s", gvkStr, obj.GetNamespace(), obj.GetName())
}
