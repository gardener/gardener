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

package extensionresources_test

import (
	"context"
	"fmt"
	"net/http"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/extensionresources"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Describe("handler", func() {
	Describe("#validateExternalResources", func() {
		var (
			ctx    context.Context
			logger logr.Logger

			decoder *admission.Decoder
			handler admission.Handler

			request *admission.Request

			ctrl *gomock.Controller
			c    *mockclient.MockClient

			resources []dummyResource
		)

		BeforeEach(func() {
			ctx = context.TODO()
			logger = logzap.New(logzap.WriteTo(GinkgoWriter))

			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)

			var err error
			decoder, err = admission.NewDecoder(kubernetes.SeedScheme)
			Expect(err).NotTo(HaveOccurred())

			handler = extensionresources.New(logger)
			Expect(inject.APIReaderInto(c, handler)).To(BeTrue())
			Expect(admission.InjectDecoderInto(decoder, handler)).To(BeTrue())

			request = nil
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		// TODO request for other reource
		Context("ignored requests", func() {
			It("should ignore DELETE operations", func() {
				request = newRequest(admissionv1.Delete, "dnsrecords", "", "")
				expectAllowed(handler.Handle(ctx, *request), ContainSubstring("operation is not CREATE or UPDATE"))
			})

			It("should ignore CONNECT operations", func() {
				request = newRequest(admissionv1.Connect, "dnsrecords", "", "")
				expectAllowed(handler.Handle(ctx, *request), ContainSubstring("operation is not CREATE or UPDATE"))
			})

			It("should ignore different resources", func() {
				request = newRequest(admissionv1.Create, "customresourcedefinitions", "", "")
				expectAllowed(handler.Handle(ctx, *request), ContainSubstring("validation not found for the given resource"))
			})
		})

		Context("create resources", func() {
			var resources = []dummyResource{
				{
					kind: "backupbuckets",
					obj:  fmt.Sprintf(backupBucketFmt, "", "gcp", "backupprovider"),
				},
				{
					kind: "backupentries",
					obj:  fmt.Sprintf(backupEntryFmt, "", "gcp", "backupentry"),
				},
				{
					kind: "controlplanes",
					obj:  fmt.Sprintf(controlPlaneFmt, "", "gcp", "cloudprovider"),
				},
				{
					kind: "dnsrecords",
					obj:  fmt.Sprintf(dnsrecordFmt, "", "dnsrecord-external", "A"),
				},
				{
					kind: "extensions",
					obj:  fmt.Sprintf(extensionsFmt, "", "gcp", "seed-gcp"),
				},
				{
					kind: "infrastructures",
					obj:  fmt.Sprintf(infrastructureFmt, "", "gcp", "seed-gcp"),
				},
				{
					kind: "networks",
					obj:  fmt.Sprintf(networksFmt, "", "calico", "seed-gcp"),
				},
				{
					kind: "operatingsystemconfigs",
					obj:  fmt.Sprintf(operatingsysconfigFmt, "", "gcp", "seed-gcp"),
				},
				{
					kind: "workers",
					obj:  fmt.Sprintf(workerFmt, "", "gcp", "seed-gcp"),
				},
			}

			It("should create successfully the resources", func() {
				for _, r := range resources {
					request = newRequest(admissionv1.Create, r.kind, r.obj, "")
					expectAllowed(handler.Handle(ctx, *request), ContainSubstring("validation successful"), resourceToId(request.Resource))
				}
			})

			It("decoding of new dns record should fail", func() {
				dns := fmt.Sprintf(dnsrecordFmt, `"resourceVersion" "1",`, "name", "A")
				request = newRequest(admissionv1.Create, "dnsrecords", dns, "")
				expectErrored(handler.Handle(ctx, *request), ContainSubstring("could not decode ar"), BeEquivalentTo(http.StatusUnprocessableEntity))
			})
		})

		Context("update resources", func() {
			BeforeEach(func() {

				resources = []dummyResource{
					{
						kind:     "backupbuckets",
						obj:      fmt.Sprintf(backupBucketFmt, `"resourceVersion": "2",`, "gcp", "backupprovider"),
						oldobj:   fmt.Sprintf(backupBucketFmt, `"resourceVersion": "1",`, "gcp", "backupprovider"),
						wrongobj: fmt.Sprintf(backupBucketFmt, `"resourceVersion": "1",`, "azure", "backupprovider"),
					},
					{
						kind:     "backupentries",
						obj:      fmt.Sprintf(backupEntryFmt, `"resourceVersion": "2",`, "gcp", "backupentry-2"),
						oldobj:   fmt.Sprintf(backupEntryFmt, `"resourceVersion": "1",`, "gcp", "backupentry"),
						wrongobj: fmt.Sprintf(backupEntryFmt, `"resourceVersion": "1",`, "azure", "backupentry-2"),
					},
					{
						kind:     "controlplanes",
						obj:      fmt.Sprintf(controlPlaneFmt, `"resourceVersion": "2",`, "gcp", "cloudprovider-2"),
						oldobj:   fmt.Sprintf(controlPlaneFmt, `"resourceVersion": "1",`, "gcp", "cloudprovider"),
						wrongobj: fmt.Sprintf(controlPlaneFmt, `"resourceVersion": "1",`, "azure", "cloudprovider-2"),
					},
					{
						kind:     "dnsrecords",
						obj:      fmt.Sprintf(dnsrecordFmt, `"resourceVersion": "2",`, "dnsrecord-2", "A"),
						oldobj:   fmt.Sprintf(dnsrecordFmt, `"resourceVersion": "1",`, "dnsrecord", "A"),
						wrongobj: fmt.Sprintf(dnsrecordFmt, `"resourceVersion": "1",`, "dnsrecord-2", "TXT"),
					},
					{
						kind:     "extensions",
						obj:      fmt.Sprintf(extensionsFmt, `"resourceVersion": "2",`, "gcp", "cloudprovider-2"),
						oldobj:   fmt.Sprintf(extensionsFmt, `"resourceVersion": "1",`, "gcp", "cloudprovider"),
						wrongobj: fmt.Sprintf(extensionsFmt, `"resourceVersion": "1",`, "azure", "cloudprovider-2"),
					},
					{
						kind:     "infrastructures",
						obj:      fmt.Sprintf(infrastructureFmt, `"resourceVersion": "2",`, "gcp", "seed-gcp-2"),
						oldobj:   fmt.Sprintf(infrastructureFmt, `"resourceVersion": "1",`, "gcp", "seed-gcp"),
						wrongobj: fmt.Sprintf(infrastructureFmt, `"resourceVersion": "1",`, "azure", "seed-gcp-2"),
					},
					{
						kind:     "networks",
						obj:      fmt.Sprintf(networksFmt, `"resourceVersion": "2",`, "calico", "seed-gcp-2"),
						oldobj:   fmt.Sprintf(networksFmt, `"resourceVersion": "1",`, "calico", "seed-gcp"),
						wrongobj: fmt.Sprintf(networksFmt, `"resourceVersion": "1",`, "provisioner", "seed-gcp-2"),
					},
					{
						kind:     "operatingsystemconfigs",
						obj:      fmt.Sprintf(operatingsysconfigFmt, `"resourceVersion": "2",`, "gcp", "seed-gcp-2"),
						oldobj:   fmt.Sprintf(operatingsysconfigFmt, `"resourceVersion": "1",`, "gcp", "seed-gcp"),
						wrongobj: fmt.Sprintf(operatingsysconfigFmt, `"resourceVersion": "1",`, "azure", "seed-gcp-2"),
					},
					{
						kind:     "workers",
						obj:      fmt.Sprintf(workerFmt, `"resourceVersion": "2",`, "gcp", "seed-gcp-2"),
						oldobj:   fmt.Sprintf(workerFmt, `"resourceVersion": "1",`, "gcp", "seed-gcp"),
						wrongobj: fmt.Sprintf(workerFmt, `"resourceVersion": "1",`, "azure", "seed-gcp-2"),
					},
				}
			})

			It("should update successfully the resources", func() {
				for _, r := range resources {
					request = newRequest(admissionv1.Update, r.kind, r.obj, r.oldobj)
					expectAllowed(handler.Handle(ctx, *request), ContainSubstring("validation successful"), resourceToId(request.Resource))
				}
			})

			It("update should fail", func() {
				for _, r := range resources {
					request = newRequest(admissionv1.Update, r.kind, r.wrongobj, r.oldobj)
					expectDenied(handler.Handle(ctx, *request), ContainSubstring(""), resourceToId(request.Resource))
				}
			})
		})
	})
})

type dummyResource struct {
	// kind of the object eg `dnsrecords`
	kind string

	// obj is valid object
	obj string

	// oldobj is fmt used for describing existing objects
	oldobj string

	// wrongobj is semantically wrong object (e.g. while updating fields that are not meant to)
	wrongobj string
}

func newRequest(operation admissionv1.Operation, kind, obj, oldobj string) *admission.Request {
	r := new(admission.Request)

	r.Operation = operation
	r.Resource = metav1.GroupVersionResource{
		Group:    extensionsv1alpha1.SchemeGroupVersion.Group,
		Version:  extensionsv1alpha1.SchemeGroupVersion.Version,
		Resource: kind}
	r.Object = runtime.RawExtension{Raw: []byte(obj)}
	r.OldObject = runtime.RawExtension{Raw: []byte(oldobj)}

	return r
}

func expectAllowed(r admission.Response, reason gomegatypes.GomegaMatcher, description ...interface{}) {
	Expect(string(r.Result.Reason)).To(reason, description...)
	Expect(r.Allowed).To(BeTrue(), description...)
}

func expectErrored(r admission.Response, reason, code gomegatypes.GomegaMatcher, description ...interface{}) {
	Expect(r.Result.Message).To(reason, description...)
	Expect(r.Result.Code).To(code, description...)
	Expect(r.Allowed).To(BeFalse(), description...)
}

func expectDenied(r admission.Response, reason gomegatypes.GomegaMatcher, description ...interface{}) {
	Expect(r.Result.Message).To(reason, description...)
	Expect(r.Allowed).To(BeFalse(), description...)
}

func resourceToId(resource metav1.GroupVersionResource) string {
	return fmt.Sprintf("%s/%s/%s", resource.Group, resource.Version, resource.Resource)
}
