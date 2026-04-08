// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakekubernetes "k8s.io/client-go/kubernetes/fake"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/flow"
	. "github.com/gardener/gardener/pkg/utils/kubernetes/client"
	mockutilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client/mock"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

func TestClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Client Suite")
}

var _ = Describe("Cleaner", func() {
	var (
		ctrl       *gomock.Controller
		ctx        context.Context
		fakeClient client.Client

		fakeClock *testclock.FakeClock
		now       time.Time
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.Background()
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()

		now = time.Unix(60, 0)
		fakeClock = testclock.NewFakeClock(now)
	})
	AfterEach(func() {
		ctrl.Finish()
	})

	Context("Cleaner", func() {
		Describe("#Clean", func() {
			It("should delete the target object", func() {
				cleaner := NewCleaner(fakeClock, NewFinalizer())

				cm1 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "foo"}}
				Expect(fakeClient.Create(ctx, cm1)).To(Succeed())

				Expect(cleaner.Clean(ctx, fakeClient, cm1)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(cm1), &corev1.ConfigMap{})).To(BeNotFoundError())
			})

			It("should succeed if not found error occurs for target object", func() {
				cleaner := NewCleaner(fakeClock, NewFinalizer())

				cm1 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "foo"}}
				Expect(cleaner.Clean(ctx, fakeClient, cm1)).To(Succeed())
			})

			It("should succeed if no match error occurs for target object", func() {
				cleaner := NewCleaner(fakeClock, NewFinalizer())

				c := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
							return &meta.NoResourceMatchError{PartialResource: schema.GroupVersionResource{}}
						},
					}).Build()

				cm1 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "foo"}}
				Expect(cleaner.Clean(ctx, c, cm1)).To(Succeed())
			})

			It("should delete all objects matching the selector", func() {
				cleaner := NewCleaner(fakeClock, NewFinalizer())

				cm1 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "foo"}}
				cm2 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "bar"}}
				Expect(fakeClient.Create(ctx, cm1)).To(Succeed())
				Expect(fakeClient.Create(ctx, cm2)).To(Succeed())

				Expect(cleaner.Clean(ctx, fakeClient, &corev1.ConfigMapList{})).To(Succeed())

				list := &corev1.ConfigMapList{}
				Expect(fakeClient.List(ctx, list)).To(Succeed())
				Expect(list.Items).To(BeEmpty())
			})

			It("should succeed if not found error occurs for list type", func() {
				cleaner := NewCleaner(fakeClock, NewFinalizer())

				c := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
							return apierrors.NewNotFound(schema.GroupResource{}, "")
						},
					}).Build()

				Expect(cleaner.Clean(ctx, c, &corev1.ConfigMapList{})).To(Succeed())
			})

			It("should succeed if no match error occurs for list type", func() {
				cleaner := NewCleaner(fakeClock, NewFinalizer())

				c := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
							return &meta.NoResourceMatchError{PartialResource: schema.GroupVersionResource{}}
						},
					}).Build()

				Expect(cleaner.Clean(ctx, c, &corev1.ConfigMapList{})).To(Succeed())
			})

			It("should finalize the object if its deletion timestamp is over the finalize grace period", func() {
				cleaner := NewCleaner(fakeClock, NewFinalizer())

				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "bar", Finalizers: []string{"finalize.me"}},
				}
				Expect(fakeClient.Create(ctx, cm)).To(Succeed())
				Expect(fakeClient.Delete(ctx, cm)).To(Succeed())

				result := &corev1.ConfigMap{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(cm), result)).To(Succeed())
				fakeClock.SetTime(result.DeletionTimestamp.Add(21 * time.Second))

				Expect(cleaner.Clean(ctx, fakeClient, result, FinalizeGracePeriodSeconds(20))).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(cm), &corev1.ConfigMap{})).To(BeNotFoundError())
			})

			It("should finalize the namespace if its deletion timestamp is over the finalize grace period", func() {
				finalizer := NewNamespaceFinalizer()
				cleaner := NewCleaner(fakeClock, finalizer)

				subResourceUpdateCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						SubResourceUpdate: func(_ context.Context, _ client.Client, subResourceName string, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							subResourceUpdateCalled = true
							Expect(subResourceName).To(Equal("finalize"))
							ns, ok := obj.(*corev1.Namespace)
							Expect(ok).To(BeTrue())
							Expect(ns.Finalizers).To(BeEmpty())
							Expect(ns.Spec.Finalizers).To(BeEmpty())
							return nil
						},
					}).Build()

				fakeClock.SetTime(now)
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Name: "baz", Finalizers: []string{"some-finalizer"}},
					Spec:       corev1.NamespaceSpec{Finalizers: []corev1.FinalizerName{"kubernetes"}},
				}
				Expect(c.Create(ctx, ns)).To(Succeed())
				Expect(c.Delete(ctx, ns)).To(Succeed())

				result := &corev1.Namespace{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(ns), result)).To(Succeed())
				fakeClock.SetTime(result.DeletionTimestamp.Add(21 * time.Second))

				Expect(cleaner.Clean(ctx, c, result, FinalizeGracePeriodSeconds(20))).To(Succeed())
				Expect(subResourceUpdateCalled).To(BeTrue())
			})

			It("should not delete the object if its deletion timestamp is not over the finalize grace period", func() {
				cleaner := NewCleaner(fakeClock, NewFinalizer())

				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "bar", Finalizers: []string{"finalize.me"}},
				}
				Expect(fakeClient.Create(ctx, cm)).To(Succeed())
				Expect(fakeClient.Delete(ctx, cm)).To(Succeed())

				fakeClock.SetTime(now)
				result := &corev1.ConfigMap{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(cm), result)).To(Succeed())

				Expect(cleaner.Clean(ctx, fakeClient, result, FinalizeGracePeriodSeconds(20))).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(cm), result)).To(Succeed())
				Expect(result.Finalizers).To(ConsistOf("finalize.me"))
			})

			It("should not delete the object if its deletion timestamp is over the finalize grace period and no finalizer is left", func() {
				var (
					deletionTimestamp = metav1.NewTime(time.Unix(30, 0))
					now               = time.Unix(50, 0)
					cleaner           = NewCleaner(fakeClock, NewFinalizer())
				)

				cm2 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "bar", DeletionTimestamp: &deletionTimestamp}}
				c := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Get: func(_ context.Context, _ client.WithWatch, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
							if key.Name == "bar" && key.Namespace == "n" {
								cm2.DeepCopyInto(obj.(*corev1.ConfigMap))
								return nil
							}
							return apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, key.Name)
						},
					}).Build()

				fakeClock.SetTime(now)

				Expect(cleaner.Clean(ctx, c, cm2, FinalizeGracePeriodSeconds(10))).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(cm2), &corev1.ConfigMap{})).To(Succeed())
			})

			It("should finalize the list if the object's deletion timestamps are over the finalize grace period", func() {
				cleaner := NewCleaner(fakeClock, NewFinalizer())

				cmTemplate := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "bar", Finalizers: []string{"finalize.me"}},
				}
				for i := range 3 {
					cm1 := cmTemplate.DeepCopy()
					cm1.Name = fmt.Sprintf("bar-%d", i)
					Expect(fakeClient.Create(ctx, cm1)).To(Succeed())
					Expect(fakeClient.Delete(ctx, cm1)).To(Succeed())
				}

				result := &corev1.ConfigMap{}
				Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: "n", Name: "bar-2"}, result)).To(Succeed())
				fakeClock.SetTime(result.DeletionTimestamp.Add(21 * time.Second))

				Expect(cleaner.Clean(ctx, fakeClient, &corev1.ConfigMapList{}, FinalizeGracePeriodSeconds(20))).To(Succeed())

				cmList := &corev1.ConfigMapList{}
				Expect(fakeClient.List(ctx, cmList)).To(Succeed())
				Expect(cmList.Items).To(BeEmpty())
			})

			It("should ignore not found errors when finalizing objects", func() {
				cleaner := NewCleaner(fakeClock, NewFinalizer())

				patchCalled := false
				c := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
							patchCalled = true
							return apierrors.NewNotFound(schema.GroupResource{}, "")
						},
					}).Build()

				fakeClock.SetTime(now)
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "bar", Finalizers: []string{"finalize.me"}},
				}
				Expect(c.Create(ctx, cm)).To(Succeed())
				Expect(c.Delete(ctx, cm)).To(Succeed())

				result := &corev1.ConfigMap{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(cm), result)).To(Succeed())
				fakeClock.SetTime(result.DeletionTimestamp.Add(21 * time.Second))

				Expect(cleaner.Clean(ctx, c, &corev1.ConfigMapList{}, FinalizeGracePeriodSeconds(20))).To(Succeed())
				Expect(patchCalled).To(BeTrue())
			})

			It("should not delete the list if the object's deletion timestamp is not over the finalize grace period", func() {
				cleaner := NewCleaner(fakeClock, NewFinalizer())

				cmTemplate := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "bar", Finalizers: []string{"finalize.me"}},
				}

				for i := range 3 {
					cm1 := cmTemplate.DeepCopy()
					cm1.Name = fmt.Sprintf("bar-%d", i)
					Expect(fakeClient.Create(ctx, cm1)).To(Succeed())
					Expect(fakeClient.Delete(ctx, cm1)).To(Succeed())
				}

				fakeClock.SetTime(now)

				Expect(cleaner.Clean(ctx, fakeClient, &corev1.ConfigMapList{}, FinalizeGracePeriodSeconds(20))).To(Succeed())

				cmList := &corev1.ConfigMapList{}
				Expect(fakeClient.List(ctx, cmList)).To(Succeed())
				Expect(cmList.Items).To(HaveLen(3))
				for i := range 3 {
					Expect(cmList.Items[i].Finalizers).To(ConsistOf("finalize.me"))
				}
			})

			It("should not delete the list if the object's deletion timestamp is over the finalize grace period and no finalizers are left", func() {
				var (
					deletionTimestamp = metav1.NewTime(time.Unix(30, 0))
					now               = time.Unix(50, 0)
					cleaner           = NewCleaner(fakeClock, NewFinalizer())
				)

				cm1 := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "bar-1", DeletionTimestamp: &deletionTimestamp}}
				cm2 := corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "bar-2", DeletionTimestamp: &deletionTimestamp}}
				c := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						List: func(_ context.Context, _ client.WithWatch, list client.ObjectList, _ ...client.ListOption) error {
							cmList, ok := list.(*corev1.ConfigMapList)
							if ok {
								cmList.Items = []corev1.ConfigMap{cm1, cm2}
							}
							return nil
						},
					}).Build()

				fakeClock.SetTime(now)

				Expect(cleaner.Clean(ctx, c, &corev1.ConfigMapList{}, FinalizeGracePeriodSeconds(10))).To(Succeed())

				cmList := &corev1.ConfigMapList{}
				Expect(c.List(ctx, cmList)).To(Succeed())
				Expect(cmList.Items).To(HaveLen(2))
			})

			It("should ensure that no error occurs because resource is not present in the cluster", func() {
				cleaner := NewCleaner(fakeClock, NewFinalizer())

				c := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
							return &meta.NoResourceMatchError{}
						},
					}).Build()

				Expect(cleaner.Clean(ctx, c, &corev1.ConfigMapList{}, FinalizeGracePeriodSeconds(10))).To(Succeed())
			})
		})
	})

	Describe("VolumeSnapshotCleaner", func() {
		var (
			fakeClient client.Client
			cleaner    Cleaner
			labels     map[string]string
			cleanOps   []CleanOption

			cleanupContent, remainingContent map[string]*volumesnapshotv1.VolumeSnapshotContent
		)

		BeforeEach(func() {
			var (
				finalizers = []string{"foo/bar"}
			)

			labels = map[string]string{"action": "cleanup"}

			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()

			cleanupContent = map[string]*volumesnapshotv1.VolumeSnapshotContent{
				"content1": {
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: finalizers,
						Name:       "content1",
						Namespace:  "default",
						Labels:     labels,
					},
				},
				"content2": {
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: finalizers,
						Name:       "content2",
						Namespace:  "default",
						Annotations: map[string]string{
							"snapshot.storage.kubernetes.io/volumesnapshot-being-deleted": "yes",
							"snapshot.storage.kubernetes.io/volumesnapshot-being-created": "yes",
						},
						Labels: labels,
					},
				},
				"content3": {
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: finalizers,
						Name:       "content3",
						Namespace:  "default",
						Annotations: map[string]string{
							"snapshot.storage.kubernetes.io/volumesnapshot-being-created": "yes",
						},
						Labels: labels,
					},
				},
			}

			remainingContent = map[string]*volumesnapshotv1.VolumeSnapshotContent{
				// Object not in deletion.
				"content4": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "content4",
						Namespace: "default",
						Annotations: map[string]string{
							"snapshot.storage.kubernetes.io/volumesnapshot-being-created": "yes",
						},
						Labels: labels,
					},
				},
				// Object w/o matching label.
				"content5": {
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: finalizers,
						Name:       "content5",
						Namespace:  "default",
					},
				},
				// Object w/ deletionTimestamp before grace period passed (created 1s later, so shorter time since deletion).
				"content6": {
					ObjectMeta: metav1.ObjectMeta{
						Finalizers: finalizers,
						Name:       "content6",
						Namespace:  "default",
					},
				},
			}

			// Create objects that should be cleaned up (with deletion timestamps)
			for _, content := range cleanupContent {
				Expect(fakeClient.Create(ctx, content.DeepCopy())).To(Succeed())
			}

			// Delete cleanup content objects so they get deletion timestamps
			for name := range cleanupContent {
				Expect(fakeClient.Delete(ctx, &volumesnapshotv1.VolumeSnapshotContent{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: name}})).To(Succeed())
			}

			// Create remaining objects
			for _, content := range remainingContent {
				Expect(fakeClient.Create(ctx, content.DeepCopy())).To(Succeed())
			}

			// Delete content5 and content6 so they get deletion timestamps too
			for _, name := range []string{"content5", "content6"} {
				Expect(fakeClient.Delete(ctx, &volumesnapshotv1.VolumeSnapshotContent{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: name}})).To(Succeed())
			}

			// Read back an actual deletion timestamp from one of the cleanup objects to determine "now"
			vsc := &volumesnapshotv1.VolumeSnapshotContent{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "content6"}, vsc)).To(Succeed())
			deletionTimestamp := vsc.DeletionTimestamp

			// Set fakeClock to well past the grace period (29s) for the cleanup objects
			fakeClock.SetTime(deletionTimestamp.Add(30 * time.Second))

			cleaner = NewVolumeSnapshotContentCleaner(fakeClock)
			cleanOps = []CleanOption{
				ListWith{
					client.MatchingLabels(labels),
				},
				DeleteWith{
					client.GracePeriodSeconds(29),
				},
			}
		})

		It("should maintain the right annotations for all contents in the list to be cleaned up", func() {
			Expect(cleaner.Clean(ctx, fakeClient, &volumesnapshotv1.VolumeSnapshotContentList{}, cleanOps...)).To(Succeed())

			contents := &volumesnapshotv1.VolumeSnapshotContentList{}
			Expect(fakeClient.List(ctx, contents)).To(Succeed())

			for _, content := range contents.Items {
				if _, ok := cleanupContent[content.Name]; ok {
					Expect(content.Annotations).To(HaveKeyWithValue("snapshot.storage.kubernetes.io/volumesnapshot-being-deleted", "yes"))
					Expect(content.Annotations).NotTo(HaveKeyWithValue("snapshot.storage.kubernetes.io/volumesnapshot-being-created", "yes"))
					continue
				}
				expected := remainingContent[content.Name]
				Expect(expected).NotTo(BeNil())
				Expect(expected.Annotations).To(Equal(content.Annotations))
			}
		})

		It("should maintain the right annotations for the content to be cleaned up", func() {
			// Get the object as stored in the fake client (with deletion timestamp set by Delete)
			storedContent := &volumesnapshotv1.VolumeSnapshotContent{}
			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: "content1"}, storedContent)).To(Succeed())

			Expect(cleaner.Clean(ctx, fakeClient, storedContent, cleanOps...)).To(Succeed())

			content := &volumesnapshotv1.VolumeSnapshotContent{}
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(storedContent), content)).To(Succeed())

			Expect(content.Annotations).To(HaveKeyWithValue("snapshot.storage.kubernetes.io/volumesnapshot-being-deleted", "yes"))
			Expect(content.Annotations).NotTo(HaveKeyWithValue("snapshot.storage.kubernetes.io/volumesnapshot-being-created", "yes"))
		})
	})

	Describe("#EnsureGone", func() {
		var fakeClient client.Client

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		})

		It("should ensure that the object is gone when not found error occurs", func() {
			cm1 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "foo"}}
			Expect(EnsureGone(ctx, logr.Discard(), fakeClient, cm1)).To(Succeed())
		})

		It("should ensure that the object is gone when no match error occurs", func() {
			c := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return &meta.NoResourceMatchError{PartialResource: schema.GroupVersionResource{}}
					},
				}).Build()

			cm1 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "foo"}}
			Expect(EnsureGone(ctx, logr.Discard(), c, cm1)).To(Succeed())
		})

		It("should ensure that the list is gone", func() {
			list := corev1.ConfigMapList{}
			Expect(EnsureGone(ctx, logr.Discard(), fakeClient, &list)).To(Succeed())
		})

		It("should ensure that the list is gone when not found error occurs", func() {
			c := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).
				WithInterceptorFuncs(interceptor.Funcs{
					List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
						return apierrors.NewNotFound(schema.GroupResource{}, "")
					},
				}).Build()

			list := corev1.ConfigMapList{}
			Expect(EnsureGone(ctx, logr.Discard(), c, &list)).To(Succeed())
		})

		It("should ensure that the list is gone when no match error occurs", func() {
			c := fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).
				WithInterceptorFuncs(interceptor.Funcs{
					List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
						return &meta.NoResourceMatchError{PartialResource: schema.GroupVersionResource{}}
					},
				}).Build()

			list := corev1.ConfigMapList{}
			Expect(EnsureGone(ctx, logr.Discard(), c, &list)).To(Succeed())
		})

		It("should error that the object is still present", func() {
			cm1 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "foo"}}
			Expect(fakeClient.Create(ctx, cm1)).To(Succeed())

			Expect(EnsureGone(ctx, logr.Discard(), fakeClient, cm1)).To(Equal(NewObjectsRemaining(cm1)))
		})

		It("should ensure that the object is ignored", func() {
			cm1 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "foo"}}
			Expect(fakeClient.Create(ctx, cm1)).To(Succeed())

			Expect(EnsureGone(ctx, logr.Discard(), fakeClient, cm1, &CleanOptions{
				IgnoreLeftovers: []IgnoreLeftoverFunc{
					func(_ logr.Logger, _ client.Object) bool {
						return true
					},
				},
			})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(cm1), &corev1.ConfigMap{})).To(Succeed())
		})

		It("should error that the list is non-empty", func() {
			cm1 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "foo"}}
			cm2 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "bar"}}
			Expect(fakeClient.Create(ctx, cm1)).To(Succeed())
			Expect(fakeClient.Create(ctx, cm2)).To(Succeed())

			list := corev1.ConfigMapList{}
			err := EnsureGone(ctx, logr.Discard(), fakeClient, &list)
			Expect(err).To(HaveOccurred())
			Expect(AreObjectsRemaining(err)).To(BeTrue())
		})

		It("should ensure objects in list are ignored", func() {
			cm1 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "foo"}}
			cm2 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "bar"}}
			Expect(fakeClient.Create(ctx, cm1)).To(Succeed())
			Expect(fakeClient.Create(ctx, cm2)).To(Succeed())

			list := corev1.ConfigMapList{}
			Expect(EnsureGone(ctx, logr.Discard(), fakeClient, &list, &CleanOptions{
				IgnoreLeftovers: []IgnoreLeftoverFunc{
					func(_ logr.Logger, _ client.Object) bool {
						return true
					},
				},
			})).To(Succeed())

			Expect(fakeClient.List(ctx, &list)).To(Succeed())
			Expect(list.Items).To(HaveLen(2))
		})
	})

	Context("#CleanOps", func() {
		var (
			cleaner *mockutilclient.MockCleaner
			ensurer *mockutilclient.MockGoneEnsurer
			o       CleanOps
		)
		BeforeEach(func() {
			cleaner = mockutilclient.NewMockCleaner(ctrl)
			ensurer = mockutilclient.NewMockGoneEnsurer(ctrl)
			o = NewCleanOps(ensurer, cleaner)
		})

		Describe("CleanAndEnsureGone", func() {
			It("should clean and ensure that the object is gone", func() {
				cm1 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "foo"}}

				gomock.InOrder(
					cleaner.EXPECT().Clean(ctx, fakeClient, cm1),
					ensurer.EXPECT().EnsureGone(ctx, logr.Discard(), fakeClient, cm1),
				)

				Expect(o.CleanAndEnsureGone(ctx, logr.Discard(), fakeClient, cm1)).To(Succeed())
			})
		})
	})

	Context("#ApplyToObjects", func() {
		var (
			s          *runtime.Scheme
			fakeClient client.Client
			ctx        context.Context
		)

		BeforeEach(func() {
			ctx = context.Background()
			s = runtime.NewScheme()
			Expect(fakekubernetes.AddToScheme(s)).To(Succeed())

			fakeClient = fakeclient.NewClientBuilder().WithScheme(s).Build()
		})

		It("should apply the function to all the objects in the list", func() {
			for i := 1; i <= 5; i++ {
				Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("secret-%d", i), Namespace: "default"}})).To(Succeed())
			}

			fn := func(ctx context.Context, object client.Object) error {
				object.SetAnnotations(map[string]string{"test-annotation": "test"})

				return fakeClient.Update(ctx, object)
			}

			secretList := &corev1.SecretList{}

			Expect(fakeClient.List(ctx, secretList)).To(Succeed())
			Expect(ApplyToObjects(ctx, secretList, fn)).To(Succeed())

			Expect(fakeClient.List(ctx, secretList)).To(Succeed())
			for _, secret := range secretList.Items {
				Expect(secret.Annotations).To(HaveKeyWithValue("test-annotation", "test"))
			}
		})
	})

	Context("#ApplyToObjectKinds", func() {
		var (
			s          *runtime.Scheme
			fakeClient client.Client
			ctx        context.Context
		)

		BeforeEach(func() {
			ctx = context.Background()
			s = runtime.NewScheme()
			Expect(fakekubernetes.AddToScheme(s)).To(Succeed())

			fakeClient = fakeclient.NewClientBuilder().WithScheme(s).Build()
		})

		It("should apply the function to all the object kinds passed", func() {
			for i := 1; i <= 5; i++ {
				Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("secret-%d", i), Namespace: "default"}})).To(Succeed())
				Expect(fakeClient.Create(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("comfigmap-%d", i), Namespace: "default"}})).To(Succeed())
			}

			annotateResources := func(ctx context.Context, object client.Object) error {
				object.SetAnnotations(map[string]string{"test-annotation": "test"})

				return fakeClient.Update(ctx, object)
			}

			fn := func(_ string, objectList client.ObjectList) flow.TaskFn {
				return func(ctx context.Context) error {
					if err := fakeClient.List(ctx, objectList); err != nil {
						return err
					}

					return ApplyToObjects(ctx, objectList, annotateResources)
				}
			}

			Expect(ApplyToObjectKinds(ctx, fn, map[string]client.ObjectList{
				"Secret":    &corev1.SecretList{},
				"ConfigMap": &corev1.ConfigMapList{},
			})).To(Succeed())

			secretList := &corev1.SecretList{}
			configMapList := &corev1.ConfigMapList{}

			Expect(fakeClient.List(ctx, secretList)).To(Succeed())
			Expect(fakeClient.List(ctx, configMapList)).To(Succeed())

			for _, secret := range secretList.Items {
				Expect(secret.Annotations).To(HaveKeyWithValue("test-annotation", "test"))
			}
			for _, configMap := range configMapList.Items {
				Expect(configMap.Annotations).To(HaveKeyWithValue("test-annotation", "test"))
			}
		})
	})

	Context("#ForceDeleteObjects", func() {
		var (
			s          *runtime.Scheme
			fakeClient client.Client
			ctx        context.Context
		)

		BeforeEach(func() {
			ctx = context.Background()
			s = runtime.NewScheme()
			Expect(fakekubernetes.AddToScheme(s)).To(Succeed())

			fakeClient = fakeclient.NewClientBuilder().WithScheme(s).Build()
		})

		It("should finalize and delete all the objects in the list", func() {
			for i := 1; i <= 5; i++ {
				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:       fmt.Sprintf("secret-%d", i),
						Namespace:  "default",
						Finalizers: []string{"finalizer"},
						Labels:     map[string]string{"key": "value"},
					},
				})).To(Succeed())
			}

			Expect(fakeClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:       fmt.Sprintf("secret-%d", 6),
					Namespace:  "default",
					Finalizers: []string{"finalizer"},
				},
			})).To(Succeed())

			secretList := &corev1.SecretList{}
			Expect(fakeClient.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(HaveLen(6))

			taskFns := ForceDeleteObjects(fakeClient, "default", &corev1.SecretList{}, client.MatchingLabels{"key": "value"})
			Expect(flow.Parallel(taskFns)(ctx)).To(Succeed())

			Expect(fakeClient.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(HaveLen(1))
		})
	})
})
