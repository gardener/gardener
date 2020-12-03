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

package webhooks_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"

	"github.com/gardener/gardener/pkg/logger"

	. "github.com/gardener/gardener/pkg/admissioncontroller/server/handlers/webhooks"
	core "github.com/gardener/gardener/pkg/apis/core/install"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/versioning"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
)

const (
	noSubResource = ""
	emptyMessage  = ""
)

var _ = Describe("Utils tests", func() {
	Context("DecodeAdmissionRequest", func() {
		var (
			scheme  = runtime.NewScheme()
			decoder = serializer.NewCodecFactory(scheme).UniversalDecoder()

			sizeRequest = int64(342)

			request = func() *http.Request {
				return createHTTPRequest(&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: corev1.SchemeGroupVersion.String(),
					},
				}, scheme, authenticationv1.UserInfo{}, admissionv1beta1.Create)
			}
		)

		core.Install(scheme)
		utilruntime.Must(kubernetesscheme.AddToScheme(scheme))

		DescribeTable("#DecodeAdmissionRequest",
			func(r func() *http.Request, limit int64, objMatcher gomegatypes.GomegaMatcher, errMatcher gomegatypes.GomegaMatcher) {
				into := &admissionv1beta1.AdmissionReview{}
				err := DecodeAdmissionRequest(r(), decoder, into, limit, logger.NewNopLogger())
				Expect(into).To(objMatcher)
				Expect(err).To(errMatcher)
			},
			Entry("should encode object", request, sizeRequest,
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Request": Not(BeNil()),
				})),
				Succeed(),
			),
			Entry("should not succeed because limit is exceeded", request, sizeRequest-1, Ignore(),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"ErrStatus": MatchFields(IgnoreExtras, Fields{
						"Reason": Equal(metav1.StatusReasonRequestEntityTooLarge),
						"Code":   Equal(int32(http.StatusRequestEntityTooLarge)),
					}),
				})),
			),
			Entry("should not succeed because of wrong content type", func() *http.Request {
				r := request()
				r.Header.Set("Content-Type", runtime.ContentTypeYAML)
				return r
			}, sizeRequest, Ignore(),
				MatchError(fmt.Sprintf("contentType not supported, expect %s", runtime.ContentTypeJSON)),
			),
		)
	})
})

func createHTTPRequest(obj runtime.Object, scheme *runtime.Scheme, user authenticationv1.UserInfo, op admissionv1beta1.Operation) *http.Request {
	admissionReview := &admissionv1beta1.AdmissionReview{}

	if obj != nil && !reflect.ValueOf(obj).IsNil() {
		writer := bytes.NewBuffer(nil)
		jsonSerializer := runtimeserializer.NewSerializerWithOptions(
			runtimeserializer.DefaultMetaFactory,
			scheme,
			scheme,
			runtimeserializer.SerializerOptions{Yaml: false},
		)
		codec := versioning.NewDefaultingCodecForScheme(
			scheme,
			jsonSerializer,
			jsonSerializer,
			obj.GetObjectKind().GroupVersionKind().GroupVersion(),
			runtime.DisabledGroupVersioner,
		)

		utilruntime.Must(codec.Encode(obj, writer))
		objData := writer.Bytes()

		gvr, _ := meta.UnsafeGuessKindToResource(obj.GetObjectKind().GroupVersionKind())
		v1Gvr := metav1.GroupVersionResource{
			Group:    gvr.Group,
			Version:  gvr.Version,
			Resource: gvr.Resource,
		}

		var name string
		if meta, ok := obj.(metav1.Object); ok {
			name = meta.GetName()
		}

		admissionReview.Request = &admissionv1beta1.AdmissionRequest{
			Name:            name,
			Operation:       op,
			Resource:        v1Gvr,
			RequestResource: &v1Gvr,
			UserInfo:        user,
			Object: runtime.RawExtension{
				Raw: objData,
			},
		}
	}

	return newRequest(admissionReview)
}

func createHTTPRequestForSubresource(name string) *http.Request {
	admissionReview := &admissionv1beta1.AdmissionReview{
		Request: &admissionv1beta1.AdmissionRequest{
			SubResource: name,
		},
	}

	return newRequest(admissionReview)
}

func newRequest(review *admissionv1beta1.AdmissionReview) *http.Request {
	data, err := json.Marshal(review)
	utilruntime.Must(err)
	request, err := http.NewRequest(http.MethodPost, "", bytes.NewReader(data))
	utilruntime.Must(err)
	request.Header.Add("Content-Type", runtime.ContentTypeJSON)
	return request
}

// decodeAdmissionResponse decodes the response from the given http request into the provided AdmissionResponse object.
func decodeAdmissionResponse(r *httptest.ResponseRecorder, into *admissionv1beta1.AdmissionReview) error {
	return json.Unmarshal(r.Body.Bytes(), into)
}
