// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package activity_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/project/activity"
)

var _ = Describe("Add", func() {
	var (
		c                           clock.Clock
		reconciler                  *Reconciler
		secretSecretBindingRef      *corev1.Secret
		secretCredentialsBindingRef *corev1.Secret
	)

	BeforeEach(func() {
		c = testing.NewFakeClock(time.Now())
		reconciler = &Reconciler{Clock: c}
		secretSecretBindingRef = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.Time{Time: c.Now().Add(-10 * time.Minute)},
				Namespace:         "garden-project",
				Labels:            map[string]string{"reference.gardener.cloud/secretbinding": "true"},
			},
		}
		secretCredentialsBindingRef = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.Time{Time: c.Now().Add(-10 * time.Minute)},
				Namespace:         "garden-project",
				Labels:            map[string]string{"reference.gardener.cloud/credentialsbinding": "true"},
			},
		}
	})

	Describe("OnlyNewlyCreatedObjects", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.OnlyNewlyCreatedObjects()
		})

		Describe("#Create", func() {
			It("should return true when object is created less than 1h ago", func() {
				Expect(p.Create(event.CreateEvent{Object: secretSecretBindingRef})).To(BeTrue())
			})

			It("should return true when object is created exactly 1h ago", func() {
				secretSecretBindingRef.CreationTimestamp.Time = c.Now().Add(-time.Hour)
				Expect(p.Create(event.CreateEvent{Object: secretSecretBindingRef})).To(BeTrue())
			})

			It("should return false when object is created more than 1h ago", func() {
				secretSecretBindingRef.CreationTimestamp.Time = c.Now().Add(-2 * time.Hour)
				Expect(p.Create(event.CreateEvent{Object: secretSecretBindingRef})).To(BeFalse())
			})
		})

		Describe("#Delete", func() {
			It("should return true", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return true", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return true", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeTrue())
			})
		})
	})

	Describe("NeedsSecretOrCredentialsBindingReferenceLabelPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.NeedsSecretOrCredentialsBindingReferenceLabelPredicate()
		})

		DescribeTable("#Create",
			func(getSecret func() *corev1.Secret, matchPredicate bool) {
				Expect(p.Create(event.CreateEvent{Object: getSecret()})).To(Equal(matchPredicate))
			},
			Entry("should return true when object has the secret binding ref label", func() *corev1.Secret { return secretSecretBindingRef }, true),
			Entry("should return true when object has the credentials binding ref label", func() *corev1.Secret { return secretCredentialsBindingRef }, true),
			Entry("should return false when object does not have the secret binding ref or credentials binding ref label", func() *corev1.Secret {
				secretSecretBindingRef.Labels = nil
				return secretSecretBindingRef
			}, false),
		)

		DescribeTable("#Update",
			func(getSecrets func() (*corev1.Secret, *corev1.Secret), matchPredicate bool) {
				oldSecret, newSecret := getSecrets()
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: newSecret})).To(Equal(matchPredicate))
			},
			Entry("should return true when both objects have the secret binding label", func() (*corev1.Secret, *corev1.Secret) {
				return secretSecretBindingRef, secretSecretBindingRef
			}, true),
			Entry("should return true when both objects have the credentials binding label", func() (*corev1.Secret, *corev1.Secret) {
				return secretCredentialsBindingRef, secretCredentialsBindingRef
			}, true),
			Entry("should return true when only new object has the secret binding label", func() (*corev1.Secret, *corev1.Secret) {
				oldSecret := secretSecretBindingRef.DeepCopy()
				oldSecret.Labels = nil
				return oldSecret, secretSecretBindingRef
			}, true),
			Entry("should return true when only new object has the credentials binding label", func() (*corev1.Secret, *corev1.Secret) {
				oldSecret := secretCredentialsBindingRef.DeepCopy()
				oldSecret.Labels = nil
				return oldSecret, secretCredentialsBindingRef
			}, true),
			Entry("should return true when only old object has the secret binding label", func() (*corev1.Secret, *corev1.Secret) {
				oldSecret := secretSecretBindingRef.DeepCopy()
				secretSecretBindingRef.Labels = nil
				return oldSecret, secretSecretBindingRef
			}, true),
			Entry("should return true when only old object has the secret credentials label", func() (*corev1.Secret, *corev1.Secret) {
				oldSecret := secretCredentialsBindingRef.DeepCopy()
				secretCredentialsBindingRef.Labels = nil
				return oldSecret, secretCredentialsBindingRef
			}, true),
			Entry("should return false when neither object has any of the labels", func() (*corev1.Secret, *corev1.Secret) {
				oldSecret := secretSecretBindingRef.DeepCopy()
				oldSecret.Labels = nil
				secretSecretBindingRef.Labels = nil
				return oldSecret, secretSecretBindingRef
			}, false),
		)

		DescribeTable("#Delete",
			func(getSecret func() *corev1.Secret, matchPredicate bool) {
				Expect(p.Delete(event.DeleteEvent{Object: getSecret()})).To(Equal(matchPredicate))
			},
			Entry("should return true when object has the secret binding label", func() *corev1.Secret { return secretSecretBindingRef }, true),
			Entry("should return true when object has the credentials binding label", func() *corev1.Secret { return secretCredentialsBindingRef }, true),
			Entry("should return false when object does not have any of the labels", func() *corev1.Secret {
				secretSecretBindingRef.Labels = nil
				return secretSecretBindingRef
			}, false),
		)

		Describe("#Generic", func() {
			It("should return true", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeTrue())
			})
		})
	})

	Describe("#MapObjectToProject", func() {
		var (
			ctx        = context.TODO()
			log        logr.Logger
			fakeClient client.Client

			project *gardencorev1beta1.Project
		)

		BeforeEach(func() {
			log = logr.Discard()
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithIndex(&gardencorev1beta1.Project{}, core.ProjectNamespace, indexer.ProjectNamespaceIndexerFunc).
				Build()
			reconciler.Client = fakeClient

			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "project",
				},
			}
		})

		It("should do nothing if the Project is not found", func() {
			Expect(reconciler.MapObjectToProject(log)(ctx, secretSecretBindingRef)).To(BeEmpty())
		})

		It("should do nothing if no related Project can be found", func() {
			Expect(fakeClient.Create(ctx, project)).To(Succeed())

			Expect(reconciler.MapObjectToProject(log)(ctx, secretSecretBindingRef)).To(BeEmpty())
		})

		It("should map the object to the Project", func() {
			project.Spec.Namespace = &secretSecretBindingRef.Namespace
			Expect(fakeClient.Create(ctx, project)).To(Succeed())

			Expect(reconciler.MapObjectToProject(log)(ctx, secretSecretBindingRef)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: project.Name}},
			))
		})
	})
})
