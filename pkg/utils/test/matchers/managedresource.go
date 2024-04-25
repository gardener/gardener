// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers

import (
	"context"
	"fmt"
	"strings"

	"github.com/onsi/gomega/format"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

type managedResourceObjectsMatcher struct {
	ctx                     context.Context
	cl                      client.Client
	expectedObjToSerialized map[client.Object]string
	extraObjectsCheck       bool

	extraObjects             []string
	missingObjects           []string
	mismatchExpectedToActual map[string]string
}

func (m *managedResourceObjectsMatcher) FailureMessage(actual interface{}) string {
	return m.createMessage(actual, "not to be")
}

func (m *managedResourceObjectsMatcher) NegatedFailureMessage(actual interface{}) string {
	return m.createMessage(actual, "to be")
}

func (m *managedResourceObjectsMatcher) createMessage(actual interface{}, addition string) string {
	managedResource, ok := actual.(*resourcesv1alpha1.ManagedResource)
	if !ok {
		return fmt.Sprintf("expected *resourcesv1alpha1.ManagedResource.  got:\n%s", format.Object(actual, 1))
	}

	var message string

	switch {
	case len(m.mismatchExpectedToActual) > 0:
		message = fmt.Sprintf("Expected for ManagedResource %s/%s the following object mismatches %s found:\n", managedResource.Namespace, managedResource.Name, addition)
		for expected, actual := range m.mismatchExpectedToActual {
			message += format.MessageWithDiff(actual, "to equal", expected)
		}
	case len(m.missingObjects) > 0:
		message = fmt.Sprintf("Expected for ManagedResource %s/%s the following missing elements %s found:\n", managedResource.Namespace, managedResource.Name, addition)
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

func (m *managedResourceObjectsMatcher) Match(actual interface{}) (bool, error) {
	if actual == nil {
		return false, nil
	}

	managedResource, ok := actual.(*resourcesv1alpha1.ManagedResource)
	if !ok {
		return false, fmt.Errorf("expected *resourcesv1alpha1.ManagedResource.  got:\n%s", format.Object(actual, 1))
	}

	// Retrieve managed resource secrets.
	var secrets = make([]*corev1.Secret, 0, len(managedResource.Spec.SecretRefs))
	for _, secretRef := range managedResource.Spec.SecretRefs {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretRef.Name,
				Namespace: managedResource.Namespace,
			},
		}

		if err := m.cl.Get(m.ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
			return false, fmt.Errorf("error when retrieving managed resource secret: %w", err)
		}
		secrets = append(secrets, secret)
	}

	dataKeyToExpected := make(map[string]string)

	// Compare expected and actual results.
	for expectedObj, expectedObjSerialized := range m.expectedObjToSerialized {
		gvk, err := apiutil.GVKForObject(expectedObj, m.cl.Scheme())
		if err != nil {
			return false, fmt.Errorf("error when extracting object kind: %w", err)
		}

		dataKey := fmt.Sprintf(
			"%s__%s__%s.yaml",
			strings.ToLower(gvk.Kind),
			expectedObj.GetNamespace(),
			strings.ReplaceAll(expectedObj.GetName(), ":", "_"),
		)
		if _, ok := dataKeyToExpected[dataKey]; ok {
			return false, fmt.Errorf("object is already specified for data key %s", dataKey)
		}
		dataKeyToExpected[dataKey] = expectedObjSerialized
	}

	dataKeys := sets.New(maps.Keys(dataKeyToExpected)...)

	// Use early returns for the following checks to not overwhelm Gomega output.
	m.mismatchExpectedToActual = findMismatchObjects(secrets, dataKeyToExpected)
	if len(m.mismatchExpectedToActual) > 0 {
		return false, nil
	}

	m.missingObjects = findMissingObjects(secrets, dataKeyToExpected)
	if len(m.missingObjects) > 0 {
		return false, nil
	}

	if m.extraObjectsCheck {
		m.extraObjects = findExtraObjects(secrets, dataKeys)
		if len(m.extraObjects) > 0 {
			return false, nil
		}
	}

	return true, nil
}

func findMismatchObjects(secrets []*corev1.Secret, keysToExpected map[string]string) map[string]string {
	mismatches := make(map[string]string)

	for _, secret := range secrets {
		for dataKey, expected := range keysToExpected {
			if actual, ok := secret.Data[dataKey]; ok && string(actual) != expected {
				mismatches[expected] = string(actual)
			}
		}
	}

	return mismatches
}

func findMissingObjects(secrets []*corev1.Secret, keysToExpected map[string]string) []string {
	var (
		keysToBeFound  = sets.New(maps.Keys(keysToExpected)...)
		missingObjects []string
	)

	for _, secret := range secrets {
		keysToBeFound = keysToBeFound.Difference(sets.New(maps.Keys(secret.Data)...))
	}

	for notFoundKey := range keysToBeFound {
		missingObjects = append(missingObjects, keysToExpected[notFoundKey])
	}

	return missingObjects
}

func findExtraObjects(secrets []*corev1.Secret, keys sets.Set[string]) []string {
	var extraObjects []string

	for _, secret := range secrets {
		extraKeys := sets.New(maps.Keys(secret.Data)...).Difference(keys)

		for extraKey := range extraKeys {
			extraObjects = append(extraObjects, string(secret.Data[extraKey]))
		}
	}

	return extraObjects
}
