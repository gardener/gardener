// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("#tryPatch", func() {
	It("should set state to obj, when conflict occurs", func() {
		s := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())
		objInFakeClient := newInfraObj()
		objInFakeClient.SetResourceVersion("1")
		objInFakeClient.Status.Conditions = []v1beta1.Condition{
			{Type: "Health", Reason: "reason", Message: "messages", Status: "status", LastUpdateTime: metav1.Now()},
		}

		c := fake.NewClientBuilder().WithScheme(s).WithObjects(objInFakeClient).Build()
		infraObj := objInFakeClient.DeepCopy()
		transform := func() error {
			infraState, _ := json.Marshal(state{"someState"})
			infraObj.GetExtensionStatus().SetState(&runtime.RawExtension{Raw: infraState})
			return nil
		}

		u := &conflictErrManager{
			conflictsBeforeUpdate: 2,
			client:                c,
		}

		tryPatchErr := tryPatch(context.Background(), retry.DefaultRetry, c, infraObj, u.patchFunc, transform)
		Expect(tryPatchErr).NotTo(HaveOccurred())

		objFromFakeClient := &extensionsv1alpha1.Infrastructure{}
		err := c.Get(context.Background(), Key("infraNamespace", "infraName"), objFromFakeClient)
		Expect(err).NotTo(HaveOccurred())
		Expect(objFromFakeClient).To(Equal(infraObj))
	})
})

func (c *conflictErrManager) patchFunc(ctx context.Context, obj client.Object, patch client.Patch, o ...client.PatchOption) error {
	if c.conflictsBeforeUpdate == c.conflictsOccured {
		return c.client.Status().Patch(ctx, obj, patch, o...)
	}

	c.conflictsOccured++
	return apierrors.NewConflict(schema.GroupResource{}, "", nil)
}
