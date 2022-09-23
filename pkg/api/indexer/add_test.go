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

package indexer_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	. "github.com/gardener/gardener/pkg/api/indexer"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("Add", func() {
	Describe("#AddAllFieldIndexes", func() {
		It("should add all expected field indexes", func() {
			indexer := &fakeIndexer{indexedFields: make(map[schema.GroupVersionKind][]string)}
			Expect(AddAllFieldIndexes(context.TODO(), indexer)).To(Succeed())

			Expect(indexer.indexedFields).To(And(
				HaveKeyWithValue(gardencorev1beta1.SchemeGroupVersion.WithKind("Project"), ConsistOf("spec.namespace")),
				HaveKeyWithValue(gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot"), ConsistOf("spec.seedName", "status.seedName")),
				HaveKeyWithValue(gardencorev1beta1.SchemeGroupVersion.WithKind("BackupBucket"), ConsistOf("spec.seedName")),
				HaveKeyWithValue(gardencorev1beta1.SchemeGroupVersion.WithKind("BackupEntry"), ConsistOf("spec.seedName")),
				HaveKeyWithValue(gardencorev1beta1.SchemeGroupVersion.WithKind("ControllerInstallation"), ConsistOf("spec.seedRef.name", "spec.registrationRef.name")),
				HaveKeyWithValue(operationsv1alpha1.SchemeGroupVersion.WithKind("Bastion"), ConsistOf("spec.shootRef.name")),
				HaveKeyWithValue(seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed"), ConsistOf("spec.shoot.name")),
			))
		})
	})
})

type fakeIndexer struct {
	indexedFields map[schema.GroupVersionKind][]string
}

func (f *fakeIndexer) IndexField(_ context.Context, obj client.Object, field string, _ client.IndexerFunc) error {
	gvk, err := apiutil.GVKForObject(obj, kubernetes.GardenScheme)
	if err != nil {
		return err
	}

	f.indexedFields[gvk] = append(f.indexedFields[gvk], field)
	return nil
}
