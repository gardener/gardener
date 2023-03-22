// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
		c          clock.Clock
		reconciler *Reconciler
		secret     *corev1.Secret
	)

	BeforeEach(func() {
		c = testing.NewFakeClock(time.Now())
		reconciler = &Reconciler{Clock: c}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.Time{Time: c.Now().Add(-10 * time.Minute)},
				Namespace:         "garden-project",
				Labels:            map[string]string{"reference.gardener.cloud/secretbinding": "true"},
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
				Expect(p.Create(event.CreateEvent{Object: secret})).To(BeTrue())
			})

			It("should return true when object is created exactly 1h ago", func() {
				secret.CreationTimestamp.Time = c.Now().Add(-time.Hour)
				Expect(p.Create(event.CreateEvent{Object: secret})).To(BeTrue())
			})

			It("should return false when object is created more than 1h ago", func() {
				secret.CreationTimestamp.Time = c.Now().Add(-2 * time.Hour)
				Expect(p.Create(event.CreateEvent{Object: secret})).To(BeFalse())
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

	Describe("NeedsSecretBindingReferenceLabelPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.NeedsSecretBindingReferenceLabelPredicate()
		})

		Describe("#Create", func() {
			It("should return true when object has the label", func() {
				Expect(p.Create(event.CreateEvent{Object: secret})).To(BeTrue())
			})

			It("should return false when object does not have the label", func() {
				secret.Labels = nil
				Expect(p.Create(event.CreateEvent{Object: secret})).To(BeFalse())
			})
		})

		Describe("#Update", func() {
			It("should return true when both objects have the label", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: secret, ObjectNew: secret})).To(BeTrue())
			})

			It("should return true when only new object has the label", func() {
				oldSecret := secret.DeepCopy()
				oldSecret.Labels = nil
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: secret})).To(BeTrue())
			})

			It("should return true when only old object has the label", func() {
				oldSecret := secret.DeepCopy()
				secret.Labels = nil
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: secret})).To(BeTrue())
			})

			It("should return false when neither object has the label", func() {
				oldSecret := secret.DeepCopy()
				oldSecret.Labels = nil
				secret.Labels = nil
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: secret})).To(BeFalse())
			})
		})

		Describe("#Delete", func() {
			It("should return true when object has the label", func() {
				Expect(p.Delete(event.DeleteEvent{Object: secret})).To(BeTrue())
			})

			It("should return false when object does not have the label", func() {
				secret.Labels = nil
				Expect(p.Delete(event.DeleteEvent{Object: secret})).To(BeFalse())
			})
		})

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

			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "project",
				},
			}
		})

		It("should do nothing if the Project is not found", func() {
			Expect(reconciler.MapObjectToProject(ctx, log, fakeClient, secret)).To(BeEmpty())
		})

		It("should do nothing if no related Project can be found", func() {
			Expect(fakeClient.Create(ctx, project)).To(Succeed())

			Expect(reconciler.MapObjectToProject(ctx, log, fakeClient, secret)).To(BeEmpty())
		})

		It("should map the object to the Project", func() {
			project.Spec.Namespace = &secret.Namespace
			Expect(fakeClient.Create(ctx, project)).To(Succeed())

			Expect(reconciler.MapObjectToProject(ctx, log, fakeClient, secret)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: project.Name}},
			))
		})
	})
})
