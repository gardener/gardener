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

package podzoneaffinity_test

import (
	"context"
	"net/http"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/resourcemanager/webhook/podzoneaffinity"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Describe("Handler", func() {
	var (
		ctx        = context.Background()
		fakeClient client.Client
		encoder    runtime.Encoder
		log        logr.Logger

		request   admission.Request
		pod       *corev1.Pod
		namespace string

		patchType admissionv1.PatchType

		handler admission.Handler
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		decoder, err := admission.NewDecoder(kubernetesscheme.Scheme)
		Expect(err).NotTo(HaveOccurred())
		encoder = &json.Serializer{}

		namespace = "shoot--foo--bar"

		request = admission.Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				Kind:      metav1.GroupVersionKind{Group: "", Kind: "Pod", Version: "v1"},
				Operation: admissionv1.Create,
				Namespace: namespace,
			},
		}

		patchType = admissionv1.PatchTypeJSONPatch

		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))

		handler = NewHandler(log)
		Expect(admission.InjectDecoderInto(decoder, handler)).To(BeTrue())
		Expect(inject.ClientInto(fakeClient, handler)).To(BeTrue())

		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-0",
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{},
		}

		Expect(fakeClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		})).To(Succeed())
	})

	Describe("#Handle", func() {
		It("should allow when operation is not 'create'", func() {
			request.Operation = admissionv1.Update

			response := handler.Handle(ctx, request)

			Expect(response).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Reason: "only 'create' operation is handled",
						Code:   http.StatusOK,
					},
				},
			}))
		})

		It("should allow when resource is not corev1.Pod", func() {
			request.Kind = metav1.GroupVersionKind{Group: "", Kind: "ConfigMap", Version: "v1"}

			response := handler.Handle(ctx, request)

			Expect(response).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Reason: "resource is not corev1.Pod",
						Code:   http.StatusOK,
					},
				},
			}))
		})

		It("should allow when subresource is specified", func() {
			request.SubResource = "status"

			response := handler.Handle(ctx, request)

			Expect(response).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Reason: "subresources on pods are not supported",
						Code:   http.StatusOK,
					},
				},
			}))
		})

		It("should return an error because the pod cannot be decoded", func() {
			request.Object.Raw = []byte(`{]`)

			Expect(handler.Handle(ctx, request)).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Code:    int32(http.StatusUnprocessableEntity),
						Message: "couldn't get version/kind; json parse error: invalid character ']' looking for beginning of object key string",
					},
				},
			}))
		})

		It("should add the zone pod affinity if affinity is not set", func() {
			pod.Spec.Affinity = nil
			objData, err := runtime.Encode(encoder, pod)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData

			response := handler.Handle(ctx, request)

			Expect(response).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed:   true,
					PatchType: &patchType,
				},
				Patches: []jsonpatch.Operation{
					{
						Operation: "add",
						Path:      "/spec/affinity",
						Value: map[string]interface{}{
							"podAffinity": map[string]interface{}{
								"requiredDuringSchedulingIgnoredDuringExecution": []interface{}{
									map[string]interface{}{
										"labelSelector": map[string]interface{}{},
										"topologyKey":   "topology.kubernetes.io/zone",
									},
								},
							},
						},
					},
				},
			}))
		})

		It("should add the zone pod affinity for a specific zone if affinity is not set", func() {
			zone := "zone-a"

			Expect(fakeClient.Update(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
					Labels: map[string]string{
						"control-plane.shoot.gardener.cloud/enforce-zone": zone,
					},
				},
			})).To(Succeed())

			pod.Spec.Affinity = nil
			objData, err := runtime.Encode(encoder, pod)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData

			response := handler.Handle(ctx, request)

			Expect(response.Patches).To(ConsistOf([]jsonpatch.Operation{
				{
					Operation: "add",
					Path:      "/spec/affinity",
					Value: map[string]interface{}{
						"nodeAffinity": map[string]interface{}{
							"requiredDuringSchedulingIgnoredDuringExecution": map[string]interface{}{
								"nodeSelectorTerms": []interface{}{
									map[string]interface{}{
										"matchExpressions": []interface{}{
											map[string]interface{}{
												"key":      "topology.kubernetes.io/zone",
												"operator": "In",
												"values": []interface{}{
													zone,
												},
											},
										},
									},
								},
							},
						},
						"podAffinity": map[string]interface{}{
							"requiredDuringSchedulingIgnoredDuringExecution": []interface{}{
								map[string]interface{}{
									"labelSelector": map[string]interface{}{},
									"topologyKey":   "topology.kubernetes.io/zone",
								},
							},
						},
					},
				},
			}))

			Expect(response.AdmissionResponse).To(Equal(admissionv1.AdmissionResponse{
				Allowed:   true,
				PatchType: &patchType,
			}))
		})

		It("should add the zone pod affinity if another pod affinity is set", func() {
			pod.Spec.Affinity = &corev1.Affinity{
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			}
			objData, err := runtime.Encode(encoder, pod)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData

			response := handler.Handle(ctx, request)

			Expect(response).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed:   true,
					PatchType: &patchType,
				},
				Patches: []jsonpatch.Operation{
					{
						Operation: "add",
						Path:      "/spec/affinity/podAffinity/requiredDuringSchedulingIgnoredDuringExecution/1",
						Value: map[string]interface{}{
							"labelSelector": map[string]interface{}{},
							"topologyKey":   "topology.kubernetes.io/zone",
						},
					},
				},
			}))
		})

		It("should add the zone pod affinity for a specific zone if another node affinity is set", func() {
			zone := "zone-a"

			Expect(fakeClient.Update(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
					Labels: map[string]string{
						"control-plane.shoot.gardener.cloud/enforce-zone": zone,
					},
				},
			})).To(Succeed())

			pod.Spec.Affinity = &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "kubernetes.io/hostname",
										Operator: corev1.NodeSelectorOpNotIn,
										Values:   []string{"foo", "bar"},
									},
								},
							},
						},
					},
				},
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							TopologyKey:   "topology.kubernetes.io/zone",
							LabelSelector: &metav1.LabelSelector{},
						},
					},
				},
			}
			objData, err := runtime.Encode(encoder, pod)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData

			response := handler.Handle(ctx, request)

			Expect(response).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed:   true,
					PatchType: &patchType,
				},
				Patches: []jsonpatch.Operation{
					{
						Operation: "add",
						Path:      "/spec/affinity/nodeAffinity/requiredDuringSchedulingIgnoredDuringExecution/nodeSelectorTerms/1",
						Value: map[string]interface{}{
							"matchExpressions": []interface{}{
								map[string]interface{}{
									"key":      "topology.kubernetes.io/zone",
									"operator": "In",
									"values": []interface{}{
										zone,
									},
								},
							},
						},
					},
				},
			}))

			Expect(response.AdmissionResponse).To(Equal(admissionv1.AdmissionResponse{
				Allowed:   true,
				PatchType: &patchType,
			}))
		})

		It("should not change anything as required affinities are set", func() {
			zone := "zone-a"

			Expect(fakeClient.Update(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
					Labels: map[string]string{
						"control-plane.shoot.gardener.cloud/enforce-zone": zone,
					},
				},
			})).To(Succeed())

			pod.Spec.Affinity = &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "topology.kubernetes.io/zone",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{zone},
									},
								},
							},
						},
					},
				},
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							TopologyKey:   "topology.kubernetes.io/zone",
							LabelSelector: &metav1.LabelSelector{},
						},
					},
				},
			}
			objData, err := runtime.Encode(encoder, pod)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData

			response := handler.Handle(ctx, request)

			Expect(response.AdmissionResponse).To(Equal(admissionv1.AdmissionResponse{
				Allowed:   true,
				PatchType: nil,
				Patch:     nil,
			}))
		})

		It("should remove a conflicting terms", func() {
			zone := "zone-a"

			Expect(fakeClient.Update(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
					Labels: map[string]string{
						"control-plane.shoot.gardener.cloud/enforce-zone": zone,
					},
				},
			})).To(Succeed())

			pod.Spec.Affinity = &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key: "topology.kubernetes.io/zone",
									},
								},
							},
						},
					},
				},
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							TopologyKey: "topology.kubernetes.io/zone",
							Namespaces: []string{
								"default",
							},
						},
					},
				},
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							TopologyKey:   "topology.kubernetes.io/zone",
							LabelSelector: &metav1.LabelSelector{},
						},
						{
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			}
			objData, err := runtime.Encode(encoder, pod)
			Expect(err).NotTo(HaveOccurred())
			request.Object.Raw = objData

			response := handler.Handle(ctx, request)

			Expect(response.AdmissionResponse).To(Equal(admissionv1.AdmissionResponse{
				Allowed:   true,
				PatchType: &patchType,
			}))

			Expect(response.Patches).To(ConsistOf([]jsonpatch.Operation{
				{
					Operation: "replace",
					Path:      "/spec/affinity/nodeAffinity/requiredDuringSchedulingIgnoredDuringExecution/nodeSelectorTerms/0/matchExpressions/0/operator",
					Value:     "In",
				},
				{
					Operation: "add",
					Path:      "/spec/affinity/nodeAffinity/requiredDuringSchedulingIgnoredDuringExecution/nodeSelectorTerms/0/matchExpressions/0/values",
					Value:     []interface{}{"zone-a"},
				},
				{
					Operation: "add",
					Path:      "/spec/affinity/podAffinity/requiredDuringSchedulingIgnoredDuringExecution/0/labelSelector",
					Value:     map[string]interface{}{},
				},
				{
					Operation: "remove",
					Path:      "/spec/affinity/podAffinity/requiredDuringSchedulingIgnoredDuringExecution/0/namespaces",
					Value:     nil,
				},
				{
					Operation: "remove",
					Path:      "/spec/affinity/podAntiAffinity/requiredDuringSchedulingIgnoredDuringExecution/1",
					Value:     nil,
				},
				{
					Operation: "replace",
					Path:      "/spec/affinity/podAntiAffinity/requiredDuringSchedulingIgnoredDuringExecution/0/topologyKey",
					Value:     "kubernetes.io/hostname",
				},
				{
					Operation: "remove",
					Path:      "/spec/affinity/podAntiAffinity/requiredDuringSchedulingIgnoredDuringExecution/0/labelSelector",
					Value:     nil,
				},
			}))
		})
	})
})
