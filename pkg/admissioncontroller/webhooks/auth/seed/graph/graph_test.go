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

package graph

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ = Describe("graph", func() {
	var (
		ctx = context.TODO()

		fakeInformerSeed *controllertest.FakeInformer
		fakeInformers    *informertest.FakeInformers

		logger logr.Logger
		graph  *graph

		seed1                *gardencorev1beta1.Seed
		seed1SecretRef       = corev1.SecretReference{Namespace: "foo", Name: "bar"}
		seed1BackupSecretRef = corev1.SecretReference{Namespace: "bar", Name: "baz"}
	)

	BeforeEach(func() {
		scheme := kubernetes.GardenScheme
		Expect(metav1.AddMetaToScheme(scheme)).To(Succeed())

		fakeInformerSeed = &controllertest.FakeInformer{}

		fakeInformers = &informertest.FakeInformers{
			Scheme: scheme,
			InformersByGVK: map[schema.GroupVersionKind]toolscache.SharedIndexInformer{
				gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"): fakeInformerSeed,
			},
		}

		logger = logzap.New(logzap.WriteTo(GinkgoWriter))
		graph = New(logger)
		Expect(graph.Setup(ctx, fakeInformers)).To(Succeed())

		seed1 = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{Name: "seed1"},
			Spec: gardencorev1beta1.SeedSpec{
				SecretRef: &seed1SecretRef,
				Backup:    &gardencorev1beta1.SeedBackup{SecretRef: seed1BackupSecretRef},
			},
		}
	})

	It("should behave as expected for gardencorev1beta1.Seed", func() {
		By("add")
		fakeInformerSeed.Add(seed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(3))
		Expect(graph.graph.Edges().Len()).To(Equal(2))
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1SecretRef.Namespace, seed1SecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1BackupSecretRef.Namespace, seed1BackupSecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())

		By("update (irrelevant change)")
		seed1Copy := seed1.DeepCopy()
		seed1.Spec.Provider.Type = "providertype"
		fakeInformerSeed.Update(seed1Copy, seed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(3))
		Expect(graph.graph.Edges().Len()).To(Equal(2))
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1SecretRef.Namespace, seed1SecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1BackupSecretRef.Namespace, seed1BackupSecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())

		By("update (remove secret ref)")
		seed1Copy = seed1.DeepCopy()
		seed1.Spec.SecretRef = nil
		fakeInformerSeed.Update(seed1Copy, seed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(2))
		Expect(graph.graph.Edges().Len()).To(Equal(1))
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1SecretRef.Namespace, seed1SecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1BackupSecretRef.Namespace, seed1BackupSecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())

		By("update (remove backup secret ref)")
		seed1Copy = seed1.DeepCopy()
		seed1.Spec.Backup = nil
		fakeInformerSeed.Update(seed1Copy, seed1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1SecretRef.Namespace, seed1SecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1BackupSecretRef.Namespace, seed1BackupSecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())

		By("update (both secret refs)")
		seed1Copy = seed1.DeepCopy()
		seed1.Spec.Backup = &gardencorev1beta1.SeedBackup{SecretRef: seed1BackupSecretRef}
		seed1.Spec.SecretRef = &seed1SecretRef
		fakeInformerSeed.Update(seed1Copy, seed1)
		Expect(graph.graph.Nodes().Len()).To(Equal(3))
		Expect(graph.graph.Edges().Len()).To(Equal(2))
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1SecretRef.Namespace, seed1SecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1BackupSecretRef.Namespace, seed1BackupSecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeTrue())

		By("delete")
		fakeInformerSeed.Delete(seed1)
		Expect(graph.graph.Nodes().Len()).To(BeZero())
		Expect(graph.graph.Edges().Len()).To(BeZero())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1SecretRef.Namespace, seed1SecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
		Expect(graph.HasPathFrom(VertexTypeSecret, seed1BackupSecretRef.Namespace, seed1BackupSecretRef.Name, VertexTypeSeed, "", seed1.Name)).To(BeFalse())
	})
})
