// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package activity_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Project Activity controller tests", func() {
	var project *gardencorev1beta1.Project
	var testNamespace *corev1.Namespace

	BeforeEach(func() {
		By("Create test Namespace")
		testNamespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				// create dedicated namespace for each test run, so that we can run multiple tests concurrently for stress tests
				GenerateName: "garden-" + testID + "-",
				Labels:       map[string]string{testID: testRunID},
			},
		}
		Expect(testClient.Create(ctx, testNamespace)).To(Succeed())
		log.Info("Created Namespace for test", "namespaceName", testNamespace.Name)

		DeferCleanup(func() {
			By("Delete test Namespace")
			Expect(testClient.Delete(ctx, testNamespace)).To(Or(Succeed(), BeNotFoundError()))
		})

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

		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(project), &gardencorev1beta1.Project{})
		}).Should(Succeed())

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
			objResourceVersion := obj.GetResourceVersion()
			Eventually(func(g Gomega) string {
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
				return obj.GetResourceVersion()
			}).Should(Equal(objResourceVersion))

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
						SecretBindingName: ptr.To("mysecretbinding"),
						CloudProfileName:  ptr.To("cloudprofile1"),
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
							Domain: ptr.To("some-domain.example.com"),
						},
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.31.1",
						},
						Networking: &gardencorev1beta1.Networking{
							Type: ptr.To("foo-networking"),
						},
					},
				}
			},
			func(obj client.Object) {
				obj.(*gardencorev1beta1.Shoot).Spec.Hibernation = &gardencorev1beta1.Hibernation{Enabled: ptr.To(true)}
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
				obj.(*gardencorev1beta1.BackupEntry).Spec.SeedName = ptr.To("foo")
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
				obj.(*gardencorev1beta1.Quota).Spec.ClusterLifetimeDays = ptr.To[int32](14)
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
