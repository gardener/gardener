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

package podschedulername_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	. "github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/podschedulername"
)

var _ = Describe("#DefaultShootControlPlanePodsSchedulerName", func() {
	var (
		ctx     context.Context
		request admission.Request
		handler admission.Handler
	)

	BeforeEach(func() {
		ctx = context.Background()
		handler = admission.HandlerFunc(DefaultShootControlPlanePodsSchedulerName)

		request = admission.Request{}
		request.Operation = admissionv1.Create
		request.Kind = metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	})

	It("should ignore other operations than CREATE", func() {
		request.Operation = admissionv1.Delete
		expectAllowed(handler.Handle(ctx, request), ContainSubstring("not CREATE"))
		request.Operation = admissionv1.Update
		expectAllowed(handler.Handle(ctx, request), ContainSubstring("not CREATE"))
		request.Operation = admissionv1.Connect
		expectAllowed(handler.Handle(ctx, request), ContainSubstring("not CREATE"))
	})

	It("should ignore other resources than Pods", func() {
		request.Kind = metav1.GroupVersionKind{Group: "foo", Version: "bar", Kind: "baz"}
		expectAllowed(handler.Handle(ctx, request), ContainSubstring("not corev1.Pod"))
	})

	It("should ignore subresources", func() {
		request.SubResource = "logs"
		expectAllowed(handler.Handle(ctx, request), ContainSubstring("subresource"))
	})

	It("should default schedulerName", func() {
		expectPatched(handler.Handle(ctx, request), ContainSubstring("shoot control plane pod"), []jsonpatch.JsonPatchOperation{
			jsonpatch.NewOperation("replace", "/spec/schedulerName", "gardener-shoot-controlplane-scheduler"),
		})
	})
})

func expectAllowed(response admission.Response, reason gomegatypes.GomegaMatcher, optionalDescription ...interface{}) {
	Expect(response.Allowed).To(BeTrue(), optionalDescription...)
	Expect(string(response.Result.Reason)).To(reason, optionalDescription...)
}

func expectPatched(response admission.Response, reason gomegatypes.GomegaMatcher, patches []jsonpatch.JsonPatchOperation, optionalDescription ...interface{}) {
	expectAllowed(response, reason, optionalDescription...)
	Expect(response.Patches).To(Equal(patches))
}
