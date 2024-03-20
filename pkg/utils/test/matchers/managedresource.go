// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package matchers

import (
	"context"
	"fmt"
	"strings"

	"github.com/onsi/gomega/format"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

type managedResourceDataMatcher struct {
	ctx         context.Context
	cl          client.Client
	expectedObj client.Object

	expectedObjectSerialized string
	actualObjectSerialized   string
	secretNames              []string
}

func (m *managedResourceDataMatcher) FailureMessage(_ interface{}) string {
	if m.actualObjectSerialized != "" {
		return format.Message(m.actualObjectSerialized, "to equal", m.expectedObjectSerialized)
	}
	return format.Message(m.expectedObjectSerialized, fmt.Sprintf("to be found in managed resource secrets %v", m.secretNames))
}

func (m *managedResourceDataMatcher) NegatedFailureMessage(_ interface{}) string {
	return format.Message(m.expectedObjectSerialized, fmt.Sprintf("not to be found in managed resource secrets %v", m.secretNames))
}

func (m *managedResourceDataMatcher) Match(actual interface{}) (bool, error) {
	if actual == nil {
		return false, nil
	}

	managedResource, ok := actual.(*resourcesv1alpha1.ManagedResource)
	if !ok {
		return false, fmt.Errorf("expected *resourcesv1alpha1.ManagedResource.  got:\n%s", format.Object(actual, 1))
	}

	gvk, err := apiutil.GVKForObject(m.expectedObj, m.cl.Scheme())
	if err != nil {
		return false, fmt.Errorf("error when extracting object kind: %w", err)
	}

	dataKey := fmt.Sprintf(
		"%s__%s__%s.yaml",
		strings.ToLower(gvk.Kind),
		m.expectedObj.GetNamespace(),
		strings.ReplaceAll(m.expectedObj.GetName(), ":", "_"),
	)

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

		m.secretNames = append(m.secretNames, secretRef.Name)

		objData, ok := secret.Data[dataKey]
		if !ok {
			continue
		}

		if string(objData) == m.expectedObjectSerialized {
			return true, nil
		} else {
			m.actualObjectSerialized = string(objData)
			return false, nil
		}
	}

	return false, nil
}
