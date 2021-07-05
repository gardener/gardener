// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seedadmissioncontroller_test

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Extension CRDs Webhook Handler", func() {
	var (
		c                            client.Client
		namespace                    = "shoot--foo--bar"
		deletionConfirmedAnnotations = map[string]string{gutil.ConfirmationDeletion: "true"}

		objects = []client.Object{
			&extensionsv1alpha1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
			&extensionsv1alpha1.BackupEntry{ObjectMeta: metav1.ObjectMeta{Name: namespace}},
			&extensionsv1alpha1.ContainerRuntime{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
			&extensionsv1alpha1.ControlPlane{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
			&extensionsv1alpha1.DNSRecord{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
			&extensionsv1alpha1.Extension{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
			&extensionsv1alpha1.Infrastructure{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
			&extensionsv1alpha1.Network{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
			&extensionsv1alpha1.OperatingSystemConfig{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},
			&extensionsv1alpha1.Worker{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "foo"}},

			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "backupbuckets.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "backupentries.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "containerruntimes.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "controlplanes.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "dnsrecords.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "extensions.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "infrastructures.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "networks.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "operatingsystemconfigs.extensions.gardener.cloud"}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "workers.extensions.gardener.cloud"}},
		}

		podObject = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: namespace},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "foo",
					Image: "foo:latest",
				}},
			},
		}
	)

	BeforeEach(func() {
		var err error
		c, err = client.New(restConfig, client.Options{Scheme: kubernetes.SeedScheme})
		Expect(err).NotTo(HaveOccurred())

		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		if err := c.Create(ctx, ns); err != nil && !apierrors.IsAlreadyExists(err) {
			Expect(err).NotTo(HaveOccurred())
		}

		By("applying CRDs")
		applier, err := kubernetes.NewChartApplierForConfig(restConfig)
		Expect(err).NotTo(HaveOccurred())
		repoRoot := filepath.Join("..", "..")
		Expect(applier.Apply(ctx, filepath.Join(repoRoot, "charts", "seed-bootstrap", "charts", "extensions"), "extensions", "")).To(Succeed())

		Eventually(func() bool {
			for _, object := range objects {
				err := c.Get(ctx, client.ObjectKeyFromObject(object), object)
				if meta.IsNoMatchError(err) {
					return false
				}
			}
			return true
		}, 1*time.Second, 50*time.Millisecond).Should(BeTrue())
	})

	objectID := func(obj client.Object) string {
		return fmt.Sprintf("%T/%s", obj, client.ObjectKeyFromObject(obj))
	}

	testDeletionUnconfirmed := func(ctx context.Context, obj client.Object) {
		Eventually(func() string {
			err := c.Delete(ctx, obj)
			return string(apierrors.ReasonForError(err))
		}, 1*time.Second, 50*time.Millisecond).Should(ContainSubstring("annotation to delete"), objectID(obj))
	}

	testDeletionConfirmed := func(ctx context.Context, obj client.Object) {
		Eventually(func() error {
			return c.Delete(ctx, obj)
		}, 1*time.Second, 50*time.Millisecond).ShouldNot(HaveOccurred(), objectID(obj))
		Eventually(func() bool {
			err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
			return apierrors.IsNotFound(err) || meta.IsNoMatchError(err)
		}, 1*time.Second, 50*time.Millisecond).Should(BeTrue(), objectID(obj))
	}

	Context("extension resources", func() {
		BeforeEach(func() {
			By("creating extension test objects")
			_, err := test.EnsureTestResources(ctx, c, filepath.Join("webhooks", "admission", "extensioncrds", "testdata"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should prevent the deletion because deletion is not confirmed", func() {
			for _, obj := range objects {
				testDeletionUnconfirmed(ctx, obj)
			}
		})

		It("should admit the deletion because deletion is confirmed", func() {
			for _, obj := range objects {
				_, err := controllerutil.CreateOrPatch(ctx, c, obj, func() error {
					obj.SetAnnotations(deletionConfirmedAnnotations)
					return nil
				})
				Expect(err).NotTo(HaveOccurred(), objectID(obj))
				testDeletionConfirmed(ctx, obj)
			}
		})
	})

	Context("other resources", func() {
		It("should not block deletion of other resources", func() {
			Expect(c.Create(ctx, podObject)).To(Succeed())
			testDeletionConfirmed(ctx, podObject)
		})
	})
})
