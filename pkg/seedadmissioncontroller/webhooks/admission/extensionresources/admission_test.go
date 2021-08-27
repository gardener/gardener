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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/seedadmissioncontroller/webhooks/admission/extensionresources"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
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
		)

		BeforeEach(func() {
			ctx = context.TODO()
			logger = logzap.New(logzap.WriteTo(GinkgoWriter))

			ctrl = gomock.NewController(GinkgoT())

			var err error
			decoder, err = admission.NewDecoder(kubernetes.SeedScheme)
			Expect(err).NotTo(HaveOccurred())

			handler = extensionresources.New(logger)
			Expect(admission.InjectDecoderInto(decoder, handler)).To(BeTrue())

			request = nil
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Context("ignored requests", func() {
			It("should ignore DELETE operations", func() {
				request = newRequest(admissionv1.Delete, "dnsrecords", nil, nil)
				expectAllowed(handler.Handle(ctx, *request), ContainSubstring("operation is not CREATE or UPDATE"))
			})

			It("should ignore CONNECT operations", func() {
				request = newRequest(admissionv1.Connect, "dnsrecords", nil, nil)
				expectAllowed(handler.Handle(ctx, *request), ContainSubstring("operation is not CREATE or UPDATE"))
			})

			It("should ignore different resources", func() {
				request = newRequest(admissionv1.Create, "customresourcedefinitions", nil, nil)
				expectAllowed(handler.Handle(ctx, *request), ContainSubstring("validation not found for the given resource"))
			})
		})

		Context("create resources", func() {
			DescribeTable("should create successfully the resource",
				func(kind string, obj runtime.Object) {
					request = newRequest(admissionv1.Create, kind, obj, nil)
					expectAllowed(handler.Handle(ctx, *request), ContainSubstring("validation successful"), resourceToId(request.Resource))
				},

				Entry("for backupbuckets", "backupbuckets", backupBucket),
				Entry("for backupentries", "backupentries", backupEntry),
				Entry("for bastions", "bastions", bastion),
				Entry("for containerruntime", "containerruntimes", containerRuntime),
				Entry("for controlplanes", "controlplanes", controlPlane),
				Entry("for dnsrecords", "dnsrecords", dnsrecord),
				Entry("for extensions", "extensions", extension),
				Entry("for infrastructures", "infrastructures", infrastructure),
				Entry("for networks", "networks", network),
				Entry("for operatingsystemconfigs", "operatingsystemconfigs", operatingsysconfig),
				Entry("for workers", "workers", worker),
			)

			It("decoding of new dns record should fail", func() {
				request = newRequest(admissionv1.Create, "dnsrecords", dnsrecord, nil)

				// intentionally break JSON validity
				obj := string(request.Object.Raw)
				request.Object.Raw = []byte(strings.Replace(obj, "{", "", 1))

				expectErrored(handler.Handle(ctx, *request), ContainSubstring("could not decode object"), BeEquivalentTo(http.StatusUnprocessableEntity))
			})
		})

		Context("update resources", func() {
			DescribeTable("should update successful the resource",
				func(kind string, new, old runtime.Object) {
					request = newRequest(admissionv1.Update, kind, new, old)
					expectAllowed(handler.Handle(ctx, *request), ContainSubstring("validation successful"), resourceToId(request.Resource))
				},

				Entry("for backupbuckets", "backupbuckets", func() runtime.Object {
					o := backupBucket.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.SecretRef.Name = "backupbucket-external"

					return o
				}(), func() runtime.Object {
					o := backupBucket.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for backupentries", "backupentries", func() runtime.Object {
					o := backupEntry.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.SecretRef.Name = "backupentry-external-2"

					return o
				}(), func() runtime.Object {
					o := backupEntry.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for backupentries", "backupentries", func() runtime.Object {
					o := backupEntry.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.SecretRef.Name = "backupentry-external-2"

					return o
				}(), func() runtime.Object {
					o := backupEntry.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for bastions", "bastions", func() runtime.Object {
					o := bastion.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.Ingress[0].IPBlock.CIDR = "1.1.1.1/16"

					return o
				}(), func() runtime.Object {
					o := bastion.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				// TODO: Fix this with #4561
				Entry("for containerruntime", "containerruntimes", func() runtime.Object {
					o := containerRuntime.DeepCopy()
					o.ResourceVersion = "2"

					return o
				}(), func() runtime.Object {
					o := containerRuntime.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for controlplanes", "controlplanes", func() runtime.Object {
					o := controlPlane.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.SecretRef.Name = "cloudprovider"

					return o
				}(), func() runtime.Object {
					o := controlPlane.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for dnsrecords", "dnsrecords", func() runtime.Object {
					o := dnsrecord.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.SecretRef.Name = "dnsrecord-external"

					return o
				}(), func() runtime.Object {
					o := dnsrecord.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for extensions", "extensions", func() runtime.Object {
					o := extension.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.ProviderConfig = &runtime.RawExtension{}

					return o
				}(), func() runtime.Object {
					o := extension.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for infrastructures", "infrastructures", func() runtime.Object {
					o := infrastructure.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.SecretRef.Name = "infrastructure-external"

					return o
				}(), func() runtime.Object {
					o := infrastructure.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for networks", "networks", func() runtime.Object {
					o := network.DeepCopy()
					o.ResourceVersion = "2"

					return o
				}(), func() runtime.Object {
					o := network.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for operatingsystemconfigs", "operatingsystemconfigs", func() runtime.Object {
					o := operatingsysconfig.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.ReloadConfigFilePath = pointer.String("path/to/file")

					return o
				}(), func() runtime.Object {
					o := operatingsysconfig.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for workers", "workers", func() runtime.Object {
					o := worker.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.SecretRef.Name = "workers-external"

					return o
				}(), func() runtime.Object {
					o := worker.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
			)

			DescribeTable("update should fail",
				func(kind string, wrong, old runtime.Object) {
					request = newRequest(admissionv1.Update, kind, wrong, old)
					expectDenied(handler.Handle(ctx, *request), ContainSubstring(""), resourceToId(request.Resource))
				},

				Entry("for backupbuckets", "backupbuckets", func() runtime.Object {
					o := backupBucket.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.SecretRef.Name = "backupbucket-external"
					o.Spec.Type = "azure"

					return o
				}(), func() runtime.Object {
					o := backupBucket.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for backupentries", "backupentries", func() runtime.Object {
					o := backupEntry.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.SecretRef.Name = "backupentry-external"
					o.Spec.Type = "azure"

					return o
				}(), func() runtime.Object {
					o := backupEntry.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for bastions", "bastions", func() runtime.Object {
					o := bastion.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.Ingress[0].IPBlock.CIDR = "1.1.1.1/16"
					o.Spec.Type = "azure"

					return o
				}(), func() runtime.Object {
					o := bastion.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				// TODO: Introduce entryfor ContainerRuntime with #4561
				Entry("for controlplanes", "controlplanes", func() runtime.Object {
					o := controlPlane.DeepCopy()
					o.ResourceVersion = "1"
					o.Spec.SecretRef.Name = "cloudprovider"
					o.Spec.Type = "azure"

					return o
				}(), func() runtime.Object {
					o := controlPlane.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for dnsrecords", "dnsrecords", func() runtime.Object {
					o := dnsrecord.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.SecretRef.Name = "dnsrecord-external"
					o.Spec.RecordType = "TXT"

					return o
				}(), func() runtime.Object {
					o := dnsrecord.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for extensions", "extensions", func() runtime.Object {
					o := extension.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.Type = "azure"

					return o
				}(), func() runtime.Object {
					o := extension.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for infrastructures", "infrastructures", func() runtime.Object {
					o := infrastructure.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.SecretRef.Name = "infrastructure-external"
					o.Spec.Type = "azure"

					return o
				}(), func() runtime.Object {
					o := infrastructure.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for networks", "networks", func() runtime.Object {
					o := network.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.PodCIDR = "1.1.1.1/16"

					return o
				}(), func() runtime.Object {
					o := network.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for operatingsystemconfigs", "operatingsystemconfigs", func() runtime.Object {
					o := operatingsysconfig.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.Type = "azure"

					return o
				}(), func() runtime.Object {
					o := operatingsysconfig.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
				Entry("for workers", "workers", func() runtime.Object {
					o := worker.DeepCopy()
					o.ResourceVersion = "2"
					o.Spec.Type = "azure"

					return o
				}(), func() runtime.Object {
					o := worker.DeepCopy()
					o.ResourceVersion = "1"

					return o
				}()),
			)
		})
	})
})

func newRequest(operation admissionv1.Operation, kind string, obj, oldobj runtime.Object) *admission.Request {
	r := new(admission.Request)

	r.Operation = operation
	r.Resource = metav1.GroupVersionResource{
		Group:    extensionsv1alpha1.SchemeGroupVersion.Group,
		Version:  extensionsv1alpha1.SchemeGroupVersion.Version,
		Resource: kind}
	r.Object = runtime.RawExtension{Raw: marshalObject(obj)}
	r.OldObject = runtime.RawExtension{Raw: marshalObject(oldobj)}

	return r
}

func marshalObject(obj runtime.Object) []byte {
	o, err := json.Marshal(obj)
	if err != nil {
		return nil
	}

	return o
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
