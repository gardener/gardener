// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Project Activity controller tests", func() {
	var project *gardencorev1beta1.Project

	BeforeEach(func() {
		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test-" + utils.ComputeSHA256Hex([]byte(testRunID + CurrentSpecReport().LeafNodeLocation.String()))[:5],
				Labels: map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: &testNamespace.Name,
			},
		}

		By("Create Project")
		Expect(testClient.Create(ctx, project)).To(Succeed())
		log.Info("Created Project", "project", client.ObjectKeyFromObject(project))

		DeferCleanup(func() {
			By("Delete Project")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, project))).To(Succeed())

			By("Wait for Project to be gone")
			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(project), project)
			}).Should(BeNotFoundError())
		})

		fakeClock.SetTime(time.Now())
	})

	test := func(
		kind string,
		createObj func() client.Object,
		mutateObj func(client.Object),
		needsLabel bool,
	) {
		It("should update the lastActivityTimestamp in the Project status", Offset(1), func() {
			obj := createObj()

			if needsLabel {
				labels := obj.GetLabels()
				labels["reference.gardener.cloud/secretbinding"] = "true"
				obj.SetLabels(labels)
			}

			By("Fetch current lastActivityTimestamp")
			var lastActivityTimestamp *metav1.Time
			Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
			lastActivityTimestamp = project.Status.LastActivityTimestamp

			By("Create " + kind)
			Expect(testClient.Create(ctx, obj)).To(Succeed())
			log.Info("Created object", "kind", kind, "object", client.ObjectKeyFromObject(obj))

			By("Wait until manager has observed " + kind + " creation")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
			}).Should(Succeed())

			By("Ensure lastActivityTimestamp was updated after object creation")
			lastActivityTimestamp = assertLastActivityTimestampUpdated(ctx, project, lastActivityTimestamp)

			fakeClock.Step(2 * time.Hour)

			By("Update " + kind)
			mutateObj(obj)
			Expect(testClient.Update(ctx, obj)).To(Succeed())
			log.Info("Updated object", "kind", kind, "object", client.ObjectKeyFromObject(obj))

			By("Wait until manager has observed updated" + kind)
			updatedObjMeta := &metav1.PartialObjectMetadata{}
			updatedObjMeta.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())

			Eventually(func(g Gomega) string {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(obj), updatedObjMeta)).To(Succeed())
				return updatedObjMeta.GetResourceVersion()
			}).Should(Equal(obj.GetResourceVersion()))

			By("Ensure lastActivityTimestamp was updated after object update")
			lastActivityTimestamp = assertLastActivityTimestampUpdated(ctx, project, lastActivityTimestamp)

			fakeClock.Step(2 * time.Hour)

			By("Delete " + kind)
			Expect(testClient.Delete(ctx, obj)).To(Succeed())
			log.Info("Deleted object", "kind", kind, "object", client.ObjectKeyFromObject(obj))

			By("Wait until manager has observed " + kind + " deletion")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
			}).Should(BeNotFoundError())

			By("Ensure lastActivityTimestamp was updated after object deletion")
			_ = assertLastActivityTimestampUpdated(ctx, project, lastActivityTimestamp)
		})

		It("should ignore CREATE events if object is older than 1 hour", Offset(1), func() {
			obj := createObj()

			fakeClock.Step(2 * time.Hour)

			By("Create " + kind)
			Expect(testClient.Create(ctx, obj)).To(Succeed())
			log.Info("Created object", "kind", kind, "object", client.ObjectKeyFromObject(obj))

			By("Wait until manager has observed " + kind)
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
			}).Should(Succeed())

			By("Ensure lastActivityTimestamp was not updated")
			assertLastActivityTimestampUnchanged(ctx, project)
		})

		if needsLabel {
			It("should ignore events if object is not labeled properly", Offset(1), func() {
				obj := createObj()

				By("Create " + kind)
				Expect(testClient.Create(ctx, obj)).To(Succeed())
				log.Info("Created object", "kind", kind, "object", client.ObjectKeyFromObject(obj))

				By("Ensure lastActivityTimestamp was not updated")
				assertLastActivityTimestampUnchanged(ctx, project)

				By("Update " + kind)
				mutateObj(obj)
				Expect(testClient.Update(ctx, obj)).To(Succeed())
				log.Info("Updated object", "kind", kind, "object", client.ObjectKeyFromObject(obj))

				By("Ensure lastActivityTimestamp was not updated")
				assertLastActivityTimestampUnchanged(ctx, project)

				By("Delete " + kind)
				Expect(testClient.Delete(ctx, obj)).To(Succeed())
				log.Info("Deleted object", "kind", kind, "object", client.ObjectKeyFromObject(obj))

				By("Ensure lastActivityTimestamp was not updated")
				assertLastActivityTimestampUnchanged(ctx, project)
			})
		}
	}

	Context("when object is Shoot", func() {
		test(
			"Shoot",
			func() client.Object {
				return &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "test-",
						Namespace:    testNamespace.Name,
						Labels:       map[string]string{testID: testRunID},
					},
					Spec: gardencorev1beta1.ShootSpec{
						SecretBindingName: "mysecretbinding",
						CloudProfileName:  "cloudprofile1",
						Region:            "europe-central-1",
						Provider: gardencorev1beta1.Provider{
							Type: "foo-provider",
							Workers: []gardencorev1beta1.Worker{
								{
									Name:    "cpu-worker",
									Minimum: 3,
									Maximum: 3,
									Machine: gardencorev1beta1.Machine{
										Type: "large",
									},
								},
							},
						},
						DNS: &gardencorev1beta1.DNS{
							Domain: pointer.String("some-domain.example.com"),
						},
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.20.1",
						},
						Networking: gardencorev1beta1.Networking{
							Type: "foo-networking",
						},
					},
				}
			},
			func(obj client.Object) {
				obj.(*gardencorev1beta1.Shoot).Spec.Hibernation = &gardencorev1beta1.Hibernation{Enabled: pointer.Bool(true)}
			},
			false,
		)
	})

	Context("when object is BackupEntry", func() {
		test(
			"BackupEntry",
			func() client.Object {
				return &gardencorev1beta1.BackupEntry{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "test-",
						Namespace:    testNamespace.Name,
						Labels:       map[string]string{testID: testRunID},
					},
					Spec: gardencorev1beta1.BackupEntrySpec{
						BucketName: "foo",
					},
				}
			},
			func(obj client.Object) {
				obj.(*gardencorev1beta1.BackupEntry).Spec.SeedName = pointer.String("foo")
			},
			false,
		)
	})

	Context("when object is Secret", func() {
		test(
			"Secret",
			func() client.Object {
				return &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "test-",
						Namespace:    testNamespace.Name,
						Labels:       map[string]string{testID: testRunID},
					},
				}
			},
			func(obj client.Object) {
				obj.(*corev1.Secret).Data = map[string][]byte{"foo": nil}
			},
			true,
		)
	})

	Context("when object is Quota", func() {
		test(
			"Quota",
			func() client.Object {
				return &gardencorev1beta1.Quota{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "test-",
						Namespace:    testNamespace.Name,
						Labels:       map[string]string{testID: testRunID},
					},
					Spec: gardencorev1beta1.QuotaSpec{
						Scope: corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
						},
					},
				}
			},
			func(obj client.Object) {
				obj.(*gardencorev1beta1.Quota).Spec.ClusterLifetimeDays = pointer.Int32(14)
			},
			true,
		)
	})
})

func assertLastActivityTimestampUpdated(ctx context.Context, project *gardencorev1beta1.Project, oldLastActivityTimestamp *metav1.Time) *metav1.Time {
	EventuallyWithOffset(1, func(g Gomega) {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
		g.Expect(project.Status.LastActivityTimestamp).NotTo(BeNil())
		if oldLastActivityTimestamp != nil {
			g.Expect(project.Status.LastActivityTimestamp.Sub(oldLastActivityTimestamp.Time)).To(BeNumerically(">", 0))
		}
	}).Should(Succeed())

	return project.Status.LastActivityTimestamp
}

func assertLastActivityTimestampUnchanged(ctx context.Context, project *gardencorev1beta1.Project) {
	ConsistentlyWithOffset(1, func(g Gomega) {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(project), project)).To(Succeed())
		g.Expect(project.Status.LastActivityTimestamp).To(BeNil())
	}).Should(Succeed())
}
