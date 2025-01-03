// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresources

import (
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

// CheckAndRemoveGCSuppressionAnnotation asserts that the specified secret was recently annotated for garbage collection
// suppression. It then removes the annotation and sets secret.Annotations to nil, the removed annotation was the last
// one in the map.
//
// This function assumes that no more than one minute has elapsed since the GC suppression annotation was placed on the
// object.
//
// Remarks: This function is used by unit tests. Note that the content of a GC suppression annotation is time-dependent.
// By removing the dynamic annotation, this function enables the unit test to handle the rest of the secret with the
// simple "expect known fixed content" pattern, even if it did not isolate calls to the clock.
func CheckAndRemoveGCSuppressionAnnotation(secret *corev1.Secret) {
	now := time.Now()
	gcSuppressionAnnotationValue := secret.Annotations[AnnotationKeySuppressGarbageCollectionUntil]
	Expect(gcSuppressionAnnotationValue).NotTo(BeEmpty())
	suppessUntilTime, err := time.Parse(time.RFC3339, gcSuppressionAnnotationValue)
	Expect(err).NotTo(HaveOccurred())
	Expect(now.Add(AnnotationValueSuppressGarbageCollectionUntilDelay + 1*time.Minute).After(suppessUntilTime)).To(BeTrue())
	Expect(now.Add(AnnotationValueSuppressGarbageCollectionUntilDelay - 1*time.Second).Before(suppessUntilTime)).To(BeTrue())
	delete(secret.Annotations, AnnotationKeySuppressGarbageCollectionUntil)
	if len(secret.Annotations) == 0 {
		secret.Annotations = nil
	}
}
