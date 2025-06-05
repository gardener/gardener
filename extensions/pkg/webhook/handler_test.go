// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	. "github.com/gardener/gardener/extensions/pkg/webhook"
	extensionsmockwebhook "github.com/gardener/gardener/extensions/pkg/webhook/mock"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
)

var logger = log.Log.WithName("controlplane-webhook-test")

var _ = Describe("Handler", func() {
	const (
		name      = "foo"
		namespace = "default"
	)

	var (
		ctrl       *gomock.Controller
		mgr        *mockmanager.MockManager
		fakeClient client.Client

		objTypes = []Type{{Obj: &corev1.Service{}}}
		svc      = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		}

		req admission.Request
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		scheme := runtime.NewScheme()
		utilruntime.Must(corev1.AddToScheme(scheme))
		utilruntime.Must(extensionsv1alpha1.AddToScheme(scheme))

		fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme).Build()

		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetScheme().Return(scheme)
		mgr.EXPECT().GetClient().Return(fakeClient)

		req = admission.Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"},
				Name:      name,
				Namespace: namespace,
				Operation: admissionv1.Create,
				Object:    runtime.RawExtension{Raw: encode(svc)},
			},
		}

	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Handle", func() {
		It("should return an allowing response if the resource wasn't changed by mutator", func() {
			// Create mock mutator
			mutator := extensionsmockwebhook.NewMockMutator(ctrl)
			mutator.EXPECT().Mutate(context.TODO(), svc, nil).Return(nil)

			// Create handler
			h, err := NewBuilder(mgr, logger).WithMutator(mutator, objTypes...).Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(err).NotTo(HaveOccurred())

			// Call Handle and check response
			resp := h.Handle(context.TODO(), req)
			Expect(resp).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Code: 200,
					},
				},
			}))
		})

		It("should return an allowing response if the resource wasn't changed by mutator and it's update", func() {
			// Create mock mutator
			mutator := extensionsmockwebhook.NewMockMutator(ctrl)

			oldSvc := svc.DeepCopy()
			oldSvc.Generation = 2

			mutator.EXPECT().Mutate(context.TODO(), svc, oldSvc).Return(nil)

			// Create handler
			h, err := NewBuilder(mgr, logger).WithMutator(mutator, objTypes...).Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(err).NotTo(HaveOccurred())

			req.Operation = admissionv1.Update
			req.OldObject = runtime.RawExtension{Raw: encode(oldSvc)}

			// Call Handle and check response
			resp := h.Handle(context.TODO(), req)
			Expect(resp).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Code: 200,
					},
				},
			}))
		})

		It("should return a patch response if the resource was changed by mutator", func() {
			// Create mock mutator
			mutator := extensionsmockwebhook.NewMockMutator(ctrl)
			mutator.EXPECT().Mutate(context.TODO(), svc, nil).DoAndReturn(func(_ context.Context, obj, _ client.Object) error {
				obj.SetAnnotations(map[string]string{"foo": "bar"})
				return nil
			})

			// Create handler
			h, err := NewBuilder(mgr, logger).WithMutator(mutator, objTypes...).Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(err).NotTo(HaveOccurred())

			// Call Handle and check response
			resp := h.Handle(context.TODO(), req)
			pt := admissionv1.PatchTypeJSONPatch
			Expect(resp).To(Equal(admission.Response{
				Patches: []jsonpatch.JsonPatchOperation{
					{
						Operation: "add",
						Path:      "/metadata/annotations",
						Value:     map[string]any{"foo": "bar"},
					},
				},
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed:   true,
					PatchType: &pt,
				},
			}))
		})

		It("should return an error response if the mutator returned an error", func() {
			// Create mock mutator
			mutator := extensionsmockwebhook.NewMockMutator(ctrl)
			mutator.EXPECT().Mutate(context.TODO(), svc, nil).Return(errors.New("test error"))

			// Create handler
			h, err := NewBuilder(mgr, logger).WithMutator(mutator, objTypes...).Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(err).NotTo(HaveOccurred())

			// Call Handle and check response
			resp := h.Handle(context.TODO(), req)
			Expect(resp).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Code:    http.StatusUnprocessableEntity,
						Message: "test error",
					},
				},
			}))
		})

		It("should inject the shoot client into the context", func() {
			// Create mock mutator
			mutator := &mutatorWantsShootClient{MockMutator: extensionsmockwebhook.NewMockMutator(ctrl)}

			ctxWithClient := context.Background()
			mutator.MockMutator.EXPECT().Mutate(gomock.Any(), svc, nil).DoAndReturn(func(ctx context.Context, _ client.Object, _ client.Object) error {
				ctxWithClient = ctx
				return nil
			})

			// Create handler
			h, err := NewBuilder(mgr, logger).WithMutator(mutator, objTypes...).Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(err).NotTo(HaveOccurred())

			// Prepare shoot client retrieval
			controlPlaneNamespace := "shoot--test--test"
			ipAddress := "100.64.0.10"
			ctxWithRemoteAddress := context.WithValue(ctxWithClient, RemoteAddrContextKey{}, ipAddress)

			Expect(fakeClient.Create(context.Background(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-apiserver",
					Namespace: controlPlaneNamespace,
					Labels: map[string]string{
						"app":  "kubernetes",
						"role": "apiserver",
					},
				},
				Status: corev1.PodStatus{
					PodIP: ipAddress,
				},
			})).To(Succeed())

			Expect(fakeClient.Create(context.Background(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gardener",
					Namespace: controlPlaneNamespace,
				},
				Data: map[string][]byte{"kubeconfig": []byte(`apiVersion: v1
clusters:
- cluster:
    server: https://` + ipAddress + `
  name: test
contexts:
- context:
    cluster: test
    user: test
  name: test
current-context: test
kind: Config
preferences: {}
users:
- name: test
  user: {}
`)},
			})).To(Succeed())

			// Call Handle and check response
			resp := h.Handle(ctxWithRemoteAddress, req)
			Expect(resp).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Code: 200,
					},
				},
			}))

			// Check context has shoot client
			val := ctxWithClient.Value(ShootClientContextKey{})
			Expect(val).NotTo(BeNil())

			_, isClient := val.(client.Client)
			Expect(isClient).To(BeTrue())
		})

		It("should inject the cluster into the context", func() {
			// Create mock mutator
			mutator := &mutatorWantsClusterObject{MockMutator: extensionsmockwebhook.NewMockMutator(ctrl)}

			ctxWithClient := context.Background()
			mutator.MockMutator.EXPECT().Mutate(gomock.Any(), svc, nil).DoAndReturn(func(ctx context.Context, _ client.Object, _ client.Object) error {
				ctxWithClient = ctx
				return nil
			})

			// Create handler
			h, err := NewBuilder(mgr, logger).WithMutator(mutator, objTypes...).Build()
			Expect(err).NotTo(HaveOccurred())
			Expect(err).NotTo(HaveOccurred())

			// Prepare shoot client retrieval
			controlPlaneNamespace := "shoot--test--test"
			ipAddress := "100.64.0.10"
			ctxWithRemoteAddress := context.WithValue(ctxWithClient, RemoteAddrContextKey{}, ipAddress)

			Expect(fakeClient.Create(context.Background(), &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-apiserver",
					Namespace: controlPlaneNamespace,
					Labels: map[string]string{
						"app":  "kubernetes",
						"role": "apiserver",
					},
				},
				Status: corev1.PodStatus{
					PodIP: ipAddress,
				},
			})).To(Succeed())

			cluster := &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: controlPlaneNamespace,
				},
			}

			Expect(fakeClient.Create(context.Background(), cluster)).To(Succeed())

			// Call Handle and check response
			resp := h.Handle(ctxWithRemoteAddress, req)
			Expect(resp).To(Equal(admission.Response{
				AdmissionResponse: admissionv1.AdmissionResponse{
					Allowed: true,
					Result: &metav1.Status{
						Code: 200,
					},
				},
			}))

			// Check context cluster object
			val := ctxWithClient.Value(ClusterObjectContextKey{})
			Expect(val).NotTo(BeNil())

			clusterObj, isCluster := val.(*extensionscontroller.Cluster)
			Expect(isCluster).To(BeTrue())
			Expect(clusterObj.ObjectMeta.GetName()).To(Equal(controlPlaneNamespace))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

type mutatorWantsClusterObject struct {
	*extensionsmockwebhook.MockMutator
}

func (m mutatorWantsClusterObject) WantsClusterObject() bool {
	return true
}

type mutatorWantsShootClient struct {
	*extensionsmockwebhook.MockMutator
}

func (m mutatorWantsShootClient) WantsShootClient() bool {
	return true
}
