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

package seedadmission_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gardener/gardener/pkg/apis/core"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardenlogger "github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/seedadmission"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"gomodules.xyz/jsonpatch/v2"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DescribeTableArgs struct {
	ExpectedPatche *jsonpatch.JsonPatchOperation
	Annotations    map[string]string
	Cluster        *extensionsv1alpha1.Cluster
	Object         *runtime.Object
}

var _ = Describe("Pods", func() {
	Describe("#MutatePod", func() {
		var (
			ctx     = context.Background()
			logger  = gardenlogger.NewNopLogger()
			request *admissionv1beta1.AdmissionRequest

			ctrl *gomock.Controller
			c    *mockclient.MockClient
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)

			request = &admissionv1beta1.AdmissionRequest{}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		var (
			resource    = metav1.GroupVersionResource{Group: corev1.SchemeGroupVersion.Group, Version: corev1.SchemeGroupVersion.Version, Resource: "pods"}
			fooResource = metav1.GroupVersionResource{Group: "foo", Version: "bar", Resource: "baz"}
			key         = "fluentbit.io/exclude"
		)

		testingPurpuse := core.ShootPurpose("testing")
		developmentPurpuse := core.ShootPurpose("development")
		notHibernation := core.Hibernation{Enabled: pointer.BoolPtr(false)}
		notHibernatedTestingShoot := &core.Shoot{
			Spec: core.ShootSpec{
				Purpose:     &testingPurpuse,
				Hibernation: &notHibernation,
			},
		}
		notHibernatedTestingShootRaw, _ := json.Marshal(notHibernatedTestingShoot)
		testingCluster := &extensionsv1alpha1.Cluster{
			Spec: extensionsv1alpha1.ClusterSpec{
				Shoot: runtime.RawExtension{Raw: notHibernatedTestingShootRaw},
			},
		}
		developmentNotHibernatedShoot := &core.Shoot{
			Spec: core.ShootSpec{
				Purpose:     &developmentPurpuse,
				Hibernation: &notHibernation,
			},
		}
		developmentNotHibernatedShootRaw, _ := json.Marshal(developmentNotHibernatedShoot)
		developmentNotHibernatedCluster := &extensionsv1alpha1.Cluster{
			Spec: extensionsv1alpha1.ClusterSpec{
				Shoot: runtime.RawExtension{Raw: developmentNotHibernatedShootRaw},
			},
		}
		annotationFluentBitExludeTrue := map[string]string{
			"fluentbit.io/exclude": "true",
		}
		espectedPatcheAddFluentBitExludeTrue := &jsonpatch.JsonPatchOperation{
			Operation: "add",
			Path:      "/metadata/annotations",
			Value: map[string]string{
				key: "true",
			},
		}
		espectedPatcheReplaceFluentBitExludeTrue := &jsonpatch.JsonPatchOperation{
			Operation: "replace",
			Path:      "/metadata/annotations/" + strings.ReplaceAll(key, "/", "~1"),
			Value:     "true",
		}

		It("should ignore types other than Pods", func() {
			request.Resource = fooResource
			patchs, err := MutatePod(ctx, c, logger, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(patchs).To(BeNil())
		})

		Context("old object is set", func() {
			var obj *unstructured.Unstructured
			var objJSON []byte
			var cluster *extensionsv1alpha1.Cluster

			BeforeEach(func() {
				obj = &unstructured.Unstructured{}
				objJSON = getObjectJSONWithLabelsAnnotations(obj, resource, nil, nil)
				request.OldObject = runtime.RawExtension{Raw: objJSON}
				request.Kind = metav1.GroupVersionKind{Kind: "Pod"}
				request.Resource = resource
				request.Name = "machine-controller-manager"
				request.Namespace = "shoot--dev--test"
				cluster = &extensionsv1alpha1.Cluster{}
			})

			It("should return an error because the old object cannot be decoded", func() {
				request.OldObject = runtime.RawExtension{Raw: []byte("foo")}

				_, err := MutatePod(ctx, c, logger, request)
				Expect(err).To(HaveOccurred(), resourceToId(resource))
				Expect(err.Error()).To(ContainSubstring("invalid character"), resourceToId(resource))
			})

			It("should return no error because the GET cluster call returned 'not found'", func() {
				c.EXPECT().Get(ctx, kutil.Key(request.Namespace), cluster).Return(apierrors.NewNotFound(core.Resource("cluster"), request.Namespace))

				patchs, err := MutatePod(ctx, c, logger, request)
				Expect(err).ToNot(HaveOccurred(), resourceToId(resource))
				Expect(patchs).To(BeNil())
			})

			It("should return error because the GET cluster call returned an error", func() {
				fakeErr := errors.New("fake get cluster error")
				c.EXPECT().Get(ctx, kutil.Key(request.Namespace), cluster).Return(fakeErr)

				_, err := MutatePod(ctx, c, logger, request)
				Expect(err).To(HaveOccurred(), resourceToId(resource))
				Expect(err.Error()).To(ContainSubstring("fake get cluster error"), resourceToId(resource))
			})

			DescribeTable("It should work properly",
				func(args DescribeTableArgs) {
					if args.Annotations != nil {
						objJSON = getObjectJSONWithLabelsAnnotations(obj, resource, nil, args.Annotations)
						request.OldObject = runtime.RawExtension{Raw: objJSON}
					}

					c.EXPECT().Get(ctx, kutil.Key(request.Namespace), cluster).DoAndReturn(populateClusterFunc(args.Cluster))

					patches, err := MutatePod(ctx, c, logger, request)
					Expect(err).ToNot(HaveOccurred(), resourceToId(resource))

					if args.ExpectedPatche != nil {
						Expect(patches).To(ConsistOf(*args.ExpectedPatche))
					} else {
						Expect(patches).To(BeNil())
					}
				},
				Entry("It should add annotation to object", DescribeTableArgs{
					ExpectedPatche: espectedPatcheAddFluentBitExludeTrue,
					Annotations:    nil,
					Cluster:        testingCluster,
				}),
				Entry("It should replace annotation to object", DescribeTableArgs{
					ExpectedPatche: espectedPatcheReplaceFluentBitExludeTrue,
					Annotations:    annotationFluentBitExludeTrue,
					Cluster:        testingCluster,
				}),
				Entry("It should not add annotation to object", DescribeTableArgs{
					ExpectedPatche: nil,
					Annotations:    nil,
					Cluster:        developmentNotHibernatedCluster,
				}),
				Entry("It should not replace annotation to object", DescribeTableArgs{
					ExpectedPatche: nil,
					Annotations:    annotationFluentBitExludeTrue,
					Cluster:        developmentNotHibernatedCluster,
				}),
			)
		})

		Context("new object is set", func() {
			var obj *unstructured.Unstructured
			var objJSON []byte
			var cluster *extensionsv1alpha1.Cluster
			BeforeEach(func() {
				obj = &unstructured.Unstructured{}
				objJSON = getObjectJSONWithLabelsAnnotations(obj, resource, nil, nil)
				request.Object = runtime.RawExtension{Raw: objJSON}
				request.Kind = metav1.GroupVersionKind{Kind: "Pod"}
				request.Resource = resource
				request.Name = "machine-controller-manager"
				request.Namespace = "shoot--dev--test"
				cluster = &extensionsv1alpha1.Cluster{}
			})

			It("should return an error because the new object cannot be decoded", func() {
				request.Object = runtime.RawExtension{Raw: []byte("foo")}

				_, err := MutatePod(ctx, c, logger, request)
				Expect(err).To(HaveOccurred(), resourceToId(resource))
				Expect(err.Error()).To(ContainSubstring("invalid character"), resourceToId(resource))
			})

			It("should return no error because the GET cluster call returned 'not found'", func() {
				c.EXPECT().Get(ctx, kutil.Key(request.Namespace), cluster).Return(apierrors.NewNotFound(core.Resource("cluster"), request.Namespace))

				patchs, err := MutatePod(ctx, c, logger, request)
				Expect(err).ToNot(HaveOccurred(), resourceToId(resource))
				Expect(patchs).To(BeNil())
			})

			It("should return error because the GET cluster call returned an error", func() {
				fakeErr := errors.New("fake get cluster error")

				c.EXPECT().Get(ctx, kutil.Key(request.Namespace), cluster).Return(fakeErr)

				_, err := MutatePod(ctx, c, logger, request)
				Expect(err).To(HaveOccurred(), resourceToId(resource))
				Expect(err.Error()).To(ContainSubstring("fake get cluster error"), resourceToId(resource))
			})

			DescribeTable("It should work properly",
				func(args DescribeTableArgs) {
					if args.Annotations != nil {
						objJSON = getObjectJSONWithLabelsAnnotations(obj, resource, nil, args.Annotations)
						request.OldObject = runtime.RawExtension{Raw: objJSON}
					}

					c.EXPECT().Get(ctx, kutil.Key(request.Namespace), cluster).DoAndReturn(populateClusterFunc(args.Cluster))

					patches, err := MutatePod(ctx, c, logger, request)
					Expect(err).ToNot(HaveOccurred(), resourceToId(resource))

					if args.ExpectedPatche != nil {
						Expect(patches).To(ConsistOf(*args.ExpectedPatche))
					} else {
						Expect(patches).To(BeNil())
					}
				},
				Entry("It should add annotation to object", DescribeTableArgs{
					ExpectedPatche: espectedPatcheAddFluentBitExludeTrue,
					Annotations:    nil,
					Cluster:        testingCluster,
				}),
				Entry("It should replace annotation to object", DescribeTableArgs{
					ExpectedPatche: espectedPatcheReplaceFluentBitExludeTrue,
					Annotations:    annotationFluentBitExludeTrue,
					Cluster:        testingCluster,
				}),
				Entry("It should not add annotation to object", DescribeTableArgs{
					ExpectedPatche: nil,
					Annotations:    nil,
					Cluster:        developmentNotHibernatedCluster,
				}),
				Entry("It should not replace annotation to object", DescribeTableArgs{
					ExpectedPatche: nil,
					Annotations:    annotationFluentBitExludeTrue,
					Cluster:        developmentNotHibernatedCluster,
				}),
			)
		})

		Context("object must be looked up", func() {
			var obj *unstructured.Unstructured
			var cluster *extensionsv1alpha1.Cluster

			BeforeEach(func() {
				obj = &unstructured.Unstructured{}
				request.Resource = resource
				request.Name = "machine-controller-manager"
				request.Namespace = "shoot--dev--test"
				prepareRequestAndObjectWithResource(request, obj, resource)
				cluster = &extensionsv1alpha1.Cluster{}
			})

			It("should return an error because the GET call failed", func() {
				fakeErr := errors.New("fake")

				c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), gomock.AssignableToTypeOf(&unstructured.Unstructured{})).Return(fakeErr)

				_, err := MutatePod(ctx, c, logger, request)
				Expect(err).To(HaveOccurred(), resourceToId(resource))
				Expect(err).To(Equal(err))
			})

			It("should return no error because the GET call returned 'not found'", func() {
				c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), gomock.AssignableToTypeOf(&unstructured.Unstructured{})).Return(apierrors.NewNotFound(core.Resource(resource.Resource), "name"))

				patchs, err := MutatePod(ctx, c, logger, request)
				Expect(err).ToNot(HaveOccurred(), resourceToId(resource))
				Expect(patchs).To(BeNil())
			})

			It("should return no error because the GET cluster call returned 'not found'", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(request.Namespace, request.Name), obj),
					c.EXPECT().Get(ctx, kutil.Key(request.Namespace), cluster).Return(apierrors.NewNotFound(core.Resource("cluster"), request.Namespace)),
				)

				patchs, err := MutatePod(ctx, c, logger, request)
				Expect(err).ToNot(HaveOccurred(), resourceToId(resource))
				Expect(patchs).To(BeNil())
			})

			It("should return error because the GET cluster call returned an error", func() {
				fakeErr := errors.New("fake get cluster error")

				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(request.Namespace, request.Name), obj),
					c.EXPECT().Get(ctx, kutil.Key(request.Namespace), cluster).Return(fakeErr),
				)

				_, err := MutatePod(ctx, c, logger, request)
				Expect(err).To(HaveOccurred(), resourceToId(resource))
				Expect(err.Error()).To(ContainSubstring("fake get cluster error"), resourceToId(resource))
			})

			DescribeTable("It should work properly",
				func(args DescribeTableArgs) {

					gomock.InOrder(
						c.EXPECT().Get(ctx, kutil.Key(request.Namespace, request.Name), obj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, o runtime.Object) error {
							prepareObjectWithLabelsAnnotations(o, resource, nil, args.Annotations)
							return nil
						}),
						c.EXPECT().Get(ctx, kutil.Key(request.Namespace), cluster).DoAndReturn(populateClusterFunc(args.Cluster)),
					)

					patches, err := MutatePod(ctx, c, logger, request)
					Expect(err).ToNot(HaveOccurred(), resourceToId(resource))

					if args.ExpectedPatche != nil {
						Expect(patches).To(ConsistOf(*args.ExpectedPatche))
					} else {
						Expect(patches).To(BeNil())
					}
				},
				Entry("It should add annotation to object", DescribeTableArgs{
					ExpectedPatche: espectedPatcheAddFluentBitExludeTrue,
					Annotations:    nil,
					Cluster:        testingCluster,
				}),
				Entry("It should replace annotation to object", DescribeTableArgs{
					ExpectedPatche: espectedPatcheReplaceFluentBitExludeTrue,
					Annotations:    annotationFluentBitExludeTrue,
					Cluster:        testingCluster,
				}),
				Entry("It should not add annotation to object", DescribeTableArgs{
					ExpectedPatche: nil,
					Annotations:    nil,
					Cluster:        developmentNotHibernatedCluster,
				}),
				Entry("It should not replace annotation to object", DescribeTableArgs{
					ExpectedPatche: nil,
					Annotations:    annotationFluentBitExludeTrue,
					Cluster:        developmentNotHibernatedCluster,
				}),
			)

		})
	})
})

func populateClusterFunc(cluster *extensionsv1alpha1.Cluster) func(_ context.Context, _ client.ObjectKey, o runtime.Object) error {
	return func(_ context.Context, _ client.ObjectKey, o runtime.Object) error {
		cl, ok := o.(*extensionsv1alpha1.Cluster)
		if !ok {
			return fmt.Errorf("Error casting runtime object to cluster")
		}
		cluster.DeepCopyInto(cl)
		return nil
	}
}
