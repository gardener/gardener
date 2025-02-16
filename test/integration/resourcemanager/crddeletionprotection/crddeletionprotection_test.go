// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crddeletionprotection_test

import (
	"context"
	"fmt"
	"strings"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	"github.com/gardener/gardener/pkg/component/extensions/crds"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Extension CRDs Webhook Handler", func() {
	var (
		deletionConfirmedAnnotations = map[string]string{v1beta1constants.ConfirmationDeletion: "true"}

		crdObjects []client.Object
		objects    []client.Object
	)

	BeforeEach(func() {
		crdObjects = []client.Object{
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "backupbuckets.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "backupentries.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "bastions.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "containerruntimes.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "controlplanes.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "dnsrecords.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "etcds.druid.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "extensions.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "infrastructures.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "managedresources.resources.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "networks.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "operatingsystemconfigs.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "workers.extensions.gardener.cloud"}},
		}
		objects = []client.Object{
			&extensionsv1alpha1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
			&extensionsv1alpha1.BackupEntry{ObjectMeta: metav1.ObjectMeta{Name: "shoot--foo--bar"}},
			&extensionsv1alpha1.ContainerRuntime{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace.Name, Name: "foo"}},
			&extensionsv1alpha1.ControlPlane{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace.Name, Name: "foo"}},
			&extensionsv1alpha1.DNSRecord{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace.Name, Name: "foo"}},
			&druidv1alpha1.Etcd{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace.Name, Name: "foo"}},
			&extensionsv1alpha1.Extension{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace.Name, Name: "foo"}},
			&extensionsv1alpha1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace.Name, Name: "foo"}},
			&extensionsv1alpha1.Network{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace.Name, Name: "foo"}},
			&extensionsv1alpha1.OperatingSystemConfig{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace.Name, Name: "foo"}},
			&extensionsv1alpha1.Worker{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace.Name, Name: "foo"}},
		}
		objects = append(objects, crdObjects...)

		By("Apply CRDs")
		applier, err := kubernetes.NewApplierForConfig(restConfig)
		Expect(err).NotTo(HaveOccurred())

		c, err := client.New(restConfig, client.Options{})
		Expect(err).NotTo(HaveOccurred())

		crdDeployer, err := crds.NewCRD(c, applier, true, true)
		Expect(err).NotTo(HaveOccurred())

		Expect(crdDeployer.Deploy(ctx)).To(Succeed())

		manifestReader := kubernetes.NewManifestReader([]byte(strings.Join([]string{
			etcd.CRD,
			resourcemanager.CRD,
		}, "\n---\n")))
		Expect(applier.ApplyManifest(ctx, manifestReader, kubernetes.DefaultMergeFuncs)).To(Succeed())

		Eventually(func() bool {
			for _, object := range objects {
				err := testClient.Get(ctx, client.ObjectKeyFromObject(object), object)
				if meta.IsNoMatchError(err) {
					return false
				}
			}
			return true
		}).Should(BeTrue())
	})

	objectID := func(obj client.Object) string {
		return fmt.Sprintf("%T/%s", obj, client.ObjectKeyFromObject(obj))
	}

	testDeletionUnconfirmed := func(ctx context.Context, obj client.Object) {
		Eventually(func() string {
			err := testClient.Delete(ctx, obj)
			return err.Error()
		}).Should(ContainSubstring("annotation to delete"), objectID(obj))
	}

	testDeleteCollectionUnconfirmed := func(ctx context.Context, obj client.Object) {
		Eventually(func() string {
			err := testClient.DeleteAllOf(ctx, obj, client.InNamespace(obj.GetNamespace()))
			return err.Error()
		}).Should(ContainSubstring("annotation to delete"), objectID(obj))
	}

	testDeletionConfirmed := func(ctx context.Context, obj client.Object) {
		Eventually(func() error {
			return testClient.Delete(ctx, obj)
		}).ShouldNot(HaveOccurred(), objectID(obj))
		Eventually(func() bool {
			err := testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
			return apierrors.IsNotFound(err) || meta.IsNoMatchError(err)
		}).Should(BeTrue(), objectID(obj))
	}

	testDeleteCollectionConfirmed := func(ctx context.Context, obj client.Object) {
		Eventually(func() error {
			return testClient.DeleteAllOf(ctx, obj, client.InNamespace(obj.GetNamespace()))
		}).ShouldNot(HaveOccurred(), objectID(obj))
		Eventually(func() bool {
			err := testClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
			return apierrors.IsNotFound(err) || meta.IsNoMatchError(err)
		}).Should(BeTrue(), objectID(obj))
	}

	Context("extension resources", func() {
		BeforeEach(func() {
			By("Create extension test objects")
			_, err := test.EnsureTestResources(ctx, testClient, testNamespace.Name, "testdata")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should prevent the deletion because deletion is not confirmed (DELETE)", func() {
			for _, obj := range objects {
				testDeletionUnconfirmed(ctx, obj)
			}
		})

		It("should prevent the deletion because deletion is not confirmed for all objects (DELETECOLLECTION)", func() {
			for i := 0; i < len(crdObjects)-1; i++ {
				obj := crdObjects[i]
				_, err := controllerutil.CreateOrPatch(ctx, testClient, obj, func() error {
					obj.SetAnnotations(deletionConfirmedAnnotations)
					return nil
				})
				Expect(err).NotTo(HaveOccurred(), objectID(obj))
			}
			testDeleteCollectionUnconfirmed(ctx, crdObjects[0])
		})

		It("should admit the deletion because deletion is confirmed (DELETE)", func() {
			for _, obj := range objects {
				_, err := controllerutil.CreateOrPatch(ctx, testClient, obj, func() error {
					obj.SetAnnotations(deletionConfirmedAnnotations)
					return nil
				})
				Expect(err).NotTo(HaveOccurred(), objectID(obj))
				testDeletionConfirmed(ctx, obj)
			}
		})

		It("should admit the deletion because deletion is confirmed (DELETECOLLECTION)", func() {
			for _, obj := range crdObjects {
				_, err := controllerutil.CreateOrPatch(ctx, testClient, obj, func() error {
					obj.SetAnnotations(deletionConfirmedAnnotations)
					return nil
				})
				Expect(err).NotTo(HaveOccurred(), objectID(obj))
				crd := &apiextensionsv1.CustomResourceDefinition{}
				Expect(testClient.Get(context.TODO(), client.ObjectKey{Name: obj.GetName()}, crd)).To(Succeed())
			}

			testDeleteCollectionConfirmed(ctx, crdObjects[0])
		})
	})

	Context("other resources", func() {
		It("should not block deletion of other resources", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-",
					Namespace:    testNamespace.Name,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "foo",
						Image: "foo:latest",
					}},
				},
			}

			Expect(testClient.Create(ctx, pod)).To(Succeed())
			testDeletionConfirmed(ctx, pod)
		})
	})
})
