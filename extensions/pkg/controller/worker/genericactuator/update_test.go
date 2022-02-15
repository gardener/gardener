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

package genericactuator

import (
	"context"
	"encoding/json"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("#tryUpdate", func() {
	It("should set state to obj, when conflict occurs", func() {
		s := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())
		objInFakeClient := newInfraObj()
		objInFakeClient.Status.Conditions = []gardencorev1beta1.Condition{
			{Type: "Health", Reason: "reason", Message: "messages", Status: "status", LastUpdateTime: metav1.Now()},
		}

		c := fake.NewClientBuilder().WithScheme(s).WithObjects(objInFakeClient).Build()
		infraObj := newInfraObj()
		transform := func() error {
			infraState, _ := json.Marshal(state{"someState"})
			infraObj.GetExtensionStatus().SetState(&runtime.RawExtension{Raw: infraState})
			return nil
		}

		u := &conflictErrManager{
			conflictsBeforeUpdate: 2,
			client:                c,
		}

		tryUpdateErr := tryUpdate(context.TODO(), retry.DefaultRetry, c, infraObj, u.updateFunc, transform)
		Expect(tryUpdateErr).NotTo(HaveOccurred())

		objFromFakeClient := &extensionsv1alpha1.Infrastructure{}
		err := c.Get(context.TODO(), kutil.Key("infraNamespace", "infraName"), objFromFakeClient)
		Expect(err).NotTo(HaveOccurred())
		Expect(objFromFakeClient).To(Equal(infraObj))
	})
})

type state struct {
	Name string `json:"name"`
}

func newInfraObj() *extensionsv1alpha1.Infrastructure {
	return &extensionsv1alpha1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "infraName",
			Namespace: "infraNamespace",
		},
	}
}

type conflictErrManager struct {
	conflictsBeforeUpdate int
	conflictsOccured      int
	client                client.Client
}

func (c *conflictErrManager) updateFunc(ctx context.Context, obj client.Object, o ...client.UpdateOption) error {
	if c.conflictsBeforeUpdate == c.conflictsOccured {
		return c.client.Status().Update(ctx, obj, o...)
	}

	c.conflictsOccured++
	return apierrors.NewConflict(schema.GroupResource{}, "", nil)
}
