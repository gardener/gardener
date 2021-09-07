// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/logger"
	mockcorev1 "github.com/gardener/gardener/pkg/mock/client-go/core/v1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mockio "github.com/gardener/gardener/pkg/mock/go/io"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"
	fakerestclient "k8s.io/client-go/rest/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("kubernetes", func() {
	const (
		namespace = "namespace"
		name      = "name"
	)

	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx     = context.TODO()
		fakeErr = fmt.Errorf("fake error")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Key", func() {
		It("should return an ObjectKey with namespace and name set", func() {
			Expect(Key(namespace, name)).To(Equal(client.ObjectKey{Namespace: namespace, Name: name}))
		})

		It("should return an ObjectKey with only name set", func() {
			Expect(Key(name)).To(Equal(client.ObjectKey{Name: name}))
		})

		It("should panic if nameOpt is longer than 1", func() {
			Expect(func() { Key("foo", "bar", "baz") }).To(Panic())
		})
	})

	Describe("#ObjectMeta", func() {
		It("should return an ObjectKey with namespace and name set", func() {
			Expect(ObjectMeta(namespace, name)).To(Equal(metav1.ObjectMeta{Namespace: namespace, Name: name}))
		})

		It("should return an ObjectKey with only name set", func() {
			Expect(ObjectMeta(name)).To(Equal(metav1.ObjectMeta{Name: name}))
		})

		It("should panic if nameOpt is longer than 1", func() {
			Expect(func() { ObjectMeta("foo", "bar", "baz") }).To(Panic())
		})
	})

	DescribeTable("#SetMetaDataLabel",
		func(labels map[string]string, key, value string, expectedLabels map[string]string) {
			original := &metav1.ObjectMeta{Labels: labels}
			modified := original.DeepCopy()

			SetMetaDataLabel(modified, key, value)
			modifiedWithOriginalLabels := modified.DeepCopy()
			modifiedWithOriginalLabels.Labels = labels
			Expect(modifiedWithOriginalLabels).To(Equal(original), "not only labels were modified")
			Expect(modified.Labels).To(Equal(expectedLabels))
		},
		Entry("nil labels", nil, "foo", "bar", map[string]string{"foo": "bar"}),
		Entry("non-nil non-conflicting labels", map[string]string{"bar": "baz"}, "foo", "bar", map[string]string{"bar": "baz", "foo": "bar"}),
		Entry("non-nil conflicting labels", map[string]string{"foo": "baz"}, "foo", "bar", map[string]string{"foo": "bar"}),
	)

	DescribeTable("#SetMetaDataAnnotation",
		func(annotations map[string]string, key, value string, expectedAnnotations map[string]string) {
			original := &metav1.ObjectMeta{Annotations: annotations}
			modified := original.DeepCopy()

			SetMetaDataAnnotation(modified, key, value)
			modifiedWithOriginalAnnotations := modified.DeepCopy()
			modifiedWithOriginalAnnotations.Annotations = annotations
			Expect(modifiedWithOriginalAnnotations).To(Equal(original), "not only annotations were modified")
			Expect(modified.Annotations).To(Equal(expectedAnnotations))
		},
		Entry("nil annotations", nil, "foo", "bar", map[string]string{"foo": "bar"}),
		Entry("non-nil non-conflicting annotations", map[string]string{"bar": "baz"}, "foo", "bar", map[string]string{"bar": "baz", "foo": "bar"}),
		Entry("non-nil conflicting annotations", map[string]string{"foo": "baz"}, "foo", "bar", map[string]string{"foo": "bar"}),
	)

	DescribeTable("#HasMetaDataAnnotation",
		func(annotations map[string]string, key, value string, result bool) {
			meta := &metav1.ObjectMeta{
				Annotations: annotations,
			}
			Expect(HasMetaDataAnnotation(meta, key, value)).To(BeIdenticalTo(result))
		},
		Entry("no annotations", map[string]string{}, "key", "value", false),
		Entry("matching annotation", map[string]string{"key": "value"}, "key", "value", true),
		Entry("no matching key", map[string]string{"key": "value"}, "key1", "value", false),
		Entry("no matching value", map[string]string{"key": "value"}, "key", "value1", false),
	)

	DescribeTable("#ValidDeploymentContainerImageVersion",
		func(containerName, minVersion string, expected bool) {
			fakeImage := "test:0.3.0"
			deployment := appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "lb-deployment",
									Image: fakeImage,
								},
							},
						},
					},
				},
			}
			ok, _ := ValidDeploymentContainerImageVersion(&deployment, containerName, minVersion)
			Expect(ok).To(Equal(expected))
		},
		Entry("invalid version", "lb-deployment", `0.4.0`, false),
		Entry("invalid container name", "deployment", "0.3.0", false),
	)

	Context("DeploymentLister", func() {
		var (
			aLabels = map[string]string{"label": "a"}
			bLabels = map[string]string{"label": "b"}

			n1ADeployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n1",
					Name:      "a",
					Labels:    aLabels,
				},
			}
			n1BDeployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n1",
					Name:      "b",
					Labels:    bLabels,
				},
			}
			n2ADeployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n2",
					Name:      "a",
					Labels:    aLabels,
				},
			}
			n2BDeployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n2",
					Name:      "b",
					Labels:    bLabels,
				},
			}

			deployments = []*appsv1.Deployment{n1ADeployment, n1BDeployment, n2ADeployment, n2BDeployment}
		)

		DescribeTable("#List",
			func(source []*appsv1.Deployment, selector labels.Selector, expected []*appsv1.Deployment) {
				lister := NewDeploymentLister(func() ([]*appsv1.Deployment, error) {
					return source, nil
				})

				actual, err := lister.List(selector)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(expected))
			},
			Entry("everything", deployments, labels.Everything(), deployments),
			Entry("nothing", deployments, labels.Nothing(), nil),
			Entry("a labels", deployments, labels.SelectorFromSet(labels.Set(aLabels)), []*appsv1.Deployment{n1ADeployment, n2ADeployment}),
			Entry("b labels", deployments, labels.SelectorFromSet(labels.Set(bLabels)), []*appsv1.Deployment{n1BDeployment, n2BDeployment}))

		Context("Deployments", func() {
			DescribeTable("#List",
				func(source []*appsv1.Deployment, namespace string, selector labels.Selector, expected []*appsv1.Deployment) {
					lister := NewDeploymentLister(func() ([]*appsv1.Deployment, error) {
						return source, nil
					})

					actual, err := lister.Deployments(namespace).List(selector)
					Expect(err).NotTo(HaveOccurred())
					Expect(actual).To(Equal(expected))
				},
				Entry("everything in n1", deployments, "n1", labels.Everything(), []*appsv1.Deployment{n1ADeployment, n1BDeployment}),
				Entry("nothing in n1", deployments, "n1", labels.Nothing(), nil),
				Entry("a labels in n1", deployments, "n1", labels.SelectorFromSet(labels.Set(aLabels)), []*appsv1.Deployment{n1ADeployment}),
				Entry("b labels in n1", deployments, "n1", labels.SelectorFromSet(labels.Set(bLabels)), []*appsv1.Deployment{n1BDeployment}),
				Entry("everything in n2", deployments, "n2", labels.Everything(), []*appsv1.Deployment{n2ADeployment, n2BDeployment}),
				Entry("nothing in n2", deployments, "n2", labels.Nothing(), nil),
				Entry("a labels in n2", deployments, "n2", labels.SelectorFromSet(labels.Set(aLabels)), []*appsv1.Deployment{n2ADeployment}),
				Entry("b labels in n2", deployments, "n2", labels.SelectorFromSet(labels.Set(bLabels)), []*appsv1.Deployment{n2BDeployment}))

			DescribeTable("#Get",
				func(source []*appsv1.Deployment, namespace, name string, deploymentMatcher, errMatcher gomegatypes.GomegaMatcher) {
					lister := NewDeploymentLister(func() ([]*appsv1.Deployment, error) {
						return source, nil
					})

					actual, err := lister.Deployments(namespace).Get(name)
					Expect(err).To(errMatcher)
					Expect(actual).To(deploymentMatcher)
				},
				Entry("a in n1", deployments, "n1", "a", Equal(n1ADeployment), Not(HaveOccurred())),
				Entry("b in n1", deployments, "n1", "b", Equal(n1BDeployment), Not(HaveOccurred())),
				Entry("c in n1", deployments, "n1", "c", BeNil(), HaveOccurred()),
				Entry("a in n2", deployments, "n2", "a", Equal(n2ADeployment), Not(HaveOccurred())),
				Entry("b in n2", deployments, "n2", "b", Equal(n2BDeployment), Not(HaveOccurred())),
				Entry("c in n2", deployments, "n2", "c", BeNil(), HaveOccurred()))
		})
	})

	Context("StatefulSetLister", func() {
		var (
			aLabels = map[string]string{"label": "a"}
			bLabels = map[string]string{"label": "b"}

			n1AStatefulSet = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n1",
					Name:      "a",
					Labels:    aLabels,
				},
			}
			n1BStatefulSet = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n1",
					Name:      "b",
					Labels:    bLabels,
				},
			}
			n2AStatefulSet = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n2",
					Name:      "a",
					Labels:    aLabels,
				},
			}
			n2BStatefulSet = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n2",
					Name:      "b",
					Labels:    bLabels,
				},
			}

			statefulSets = []*appsv1.StatefulSet{n1AStatefulSet, n1BStatefulSet, n2AStatefulSet, n2BStatefulSet}
		)

		DescribeTable("#List",
			func(source []*appsv1.StatefulSet, selector labels.Selector, expected []*appsv1.StatefulSet) {
				lister := NewStatefulSetLister(func() ([]*appsv1.StatefulSet, error) {
					return source, nil
				})

				actual, err := lister.List(selector)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(expected))
			},
			Entry("everything", statefulSets, labels.Everything(), statefulSets),
			Entry("nothing", statefulSets, labels.Nothing(), nil),
			Entry("a labels", statefulSets, labels.SelectorFromSet(labels.Set(aLabels)), []*appsv1.StatefulSet{n1AStatefulSet, n2AStatefulSet}),
			Entry("b labels", statefulSets, labels.SelectorFromSet(labels.Set(bLabels)), []*appsv1.StatefulSet{n1BStatefulSet, n2BStatefulSet}))

		Context("StatefulSets", func() {
			DescribeTable("#List",
				func(source []*appsv1.StatefulSet, namespace string, selector labels.Selector, expected []*appsv1.StatefulSet) {
					lister := NewStatefulSetLister(func() ([]*appsv1.StatefulSet, error) {
						return source, nil
					})

					actual, err := lister.StatefulSets(namespace).List(selector)
					Expect(err).NotTo(HaveOccurred())
					Expect(actual).To(Equal(expected))
				},
				Entry("everything in n1", statefulSets, "n1", labels.Everything(), []*appsv1.StatefulSet{n1AStatefulSet, n1BStatefulSet}),
				Entry("nothing in n1", statefulSets, "n1", labels.Nothing(), nil),
				Entry("a labels in n1", statefulSets, "n1", labels.SelectorFromSet(labels.Set(aLabels)), []*appsv1.StatefulSet{n1AStatefulSet}),
				Entry("b labels in n1", statefulSets, "n1", labels.SelectorFromSet(labels.Set(bLabels)), []*appsv1.StatefulSet{n1BStatefulSet}),
				Entry("everything in n2", statefulSets, "n2", labels.Everything(), []*appsv1.StatefulSet{n2AStatefulSet, n2BStatefulSet}),
				Entry("nothing in n2", statefulSets, "n2", labels.Nothing(), nil),
				Entry("a labels in n2", statefulSets, "n2", labels.SelectorFromSet(labels.Set(aLabels)), []*appsv1.StatefulSet{n2AStatefulSet}),
				Entry("b labels in n2", statefulSets, "n2", labels.SelectorFromSet(labels.Set(bLabels)), []*appsv1.StatefulSet{n2BStatefulSet}))

			DescribeTable("#Get",
				func(source []*appsv1.StatefulSet, namespace, name string, statefulSetMatcher, errMatcher gomegatypes.GomegaMatcher) {
					lister := NewStatefulSetLister(func() ([]*appsv1.StatefulSet, error) {
						return source, nil
					})

					actual, err := lister.StatefulSets(namespace).Get(name)
					Expect(err).To(errMatcher)
					Expect(actual).To(statefulSetMatcher)
				},
				Entry("a in n1", statefulSets, "n1", "a", Equal(n1AStatefulSet), Not(HaveOccurred())),
				Entry("b in n1", statefulSets, "n1", "b", Equal(n1BStatefulSet), Not(HaveOccurred())),
				Entry("c in n1", statefulSets, "n1", "c", BeNil(), HaveOccurred()),
				Entry("a in n2", statefulSets, "n2", "a", Equal(n2AStatefulSet), Not(HaveOccurred())),
				Entry("b in n2", statefulSets, "n2", "b", Equal(n2BStatefulSet), Not(HaveOccurred())),
				Entry("c in n2", statefulSets, "n2", "c", BeNil(), HaveOccurred()))
		})
	})

	Context("DaemonSetLister", func() {
		var (
			aLabels = map[string]string{"label": "a"}
			bLabels = map[string]string{"label": "b"}

			n1ADaemonSet = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n1",
					Name:      "a",
					Labels:    aLabels,
				},
			}
			n1BDaemonSet = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n1",
					Name:      "b",
					Labels:    bLabels,
				},
			}
			n2ADaemonSet = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n2",
					Name:      "a",
					Labels:    aLabels,
				},
			}
			n2BDaemonSet = &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n2",
					Name:      "b",
					Labels:    bLabels,
				},
			}

			daemonSets = []*appsv1.DaemonSet{n1ADaemonSet, n1BDaemonSet, n2ADaemonSet, n2BDaemonSet}
		)

		DescribeTable("#List",
			func(source []*appsv1.DaemonSet, selector labels.Selector, expected []*appsv1.DaemonSet) {
				lister := NewDaemonSetLister(func() ([]*appsv1.DaemonSet, error) {
					return source, nil
				})

				actual, err := lister.List(selector)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(expected))
			},
			Entry("everything", daemonSets, labels.Everything(), daemonSets),
			Entry("nothing", daemonSets, labels.Nothing(), nil),
			Entry("a labels", daemonSets, labels.SelectorFromSet(labels.Set(aLabels)), []*appsv1.DaemonSet{n1ADaemonSet, n2ADaemonSet}),
			Entry("b labels", daemonSets, labels.SelectorFromSet(labels.Set(bLabels)), []*appsv1.DaemonSet{n1BDaemonSet, n2BDaemonSet}))

		Context("DaemonSets", func() {
			DescribeTable("#List",
				func(source []*appsv1.DaemonSet, namespace string, selector labels.Selector, expected []*appsv1.DaemonSet) {
					lister := NewDaemonSetLister(func() ([]*appsv1.DaemonSet, error) {
						return source, nil
					})

					actual, err := lister.DaemonSets(namespace).List(selector)
					Expect(err).NotTo(HaveOccurred())
					Expect(actual).To(Equal(expected))
				},
				Entry("everything in n1", daemonSets, "n1", labels.Everything(), []*appsv1.DaemonSet{n1ADaemonSet, n1BDaemonSet}),
				Entry("nothing in n1", daemonSets, "n1", labels.Nothing(), nil),
				Entry("a labels in n1", daemonSets, "n1", labels.SelectorFromSet(labels.Set(aLabels)), []*appsv1.DaemonSet{n1ADaemonSet}),
				Entry("b labels in n1", daemonSets, "n1", labels.SelectorFromSet(labels.Set(bLabels)), []*appsv1.DaemonSet{n1BDaemonSet}),
				Entry("everything in n2", daemonSets, "n2", labels.Everything(), []*appsv1.DaemonSet{n2ADaemonSet, n2BDaemonSet}),
				Entry("nothing in n2", daemonSets, "n2", labels.Nothing(), nil),
				Entry("a labels in n2", daemonSets, "n2", labels.SelectorFromSet(labels.Set(aLabels)), []*appsv1.DaemonSet{n2ADaemonSet}),
				Entry("b labels in n2", daemonSets, "n2", labels.SelectorFromSet(labels.Set(bLabels)), []*appsv1.DaemonSet{n2BDaemonSet}))

			DescribeTable("#Get",
				func(source []*appsv1.DaemonSet, namespace, name string, daemonSetMatcher, errMatcher gomegatypes.GomegaMatcher) {
					lister := NewDaemonSetLister(func() ([]*appsv1.DaemonSet, error) {
						return source, nil
					})

					actual, err := lister.DaemonSets(namespace).Get(name)
					Expect(err).To(errMatcher)
					Expect(actual).To(daemonSetMatcher)
				},
				Entry("a in n1", daemonSets, "n1", "a", Equal(n1ADaemonSet), Not(HaveOccurred())),
				Entry("b in n1", daemonSets, "n1", "b", Equal(n1BDaemonSet), Not(HaveOccurred())),
				Entry("c in n1", daemonSets, "n1", "c", BeNil(), HaveOccurred()),
				Entry("a in n2", daemonSets, "n2", "a", Equal(n2ADaemonSet), Not(HaveOccurred())),
				Entry("b in n2", daemonSets, "n2", "b", Equal(n2BDaemonSet), Not(HaveOccurred())),
				Entry("c in n2", daemonSets, "n2", "c", BeNil(), HaveOccurred()))
		})
	})

	Context("WorkerLister", func() {
		var (
			aLabels = map[string]string{"label": "a"}
			bLabels = map[string]string{"label": "b"}

			n1AWorker = &extensionsv1alpha1.Worker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n1",
					Name:      "a",
					Labels:    aLabels,
				},
			}
			n1BWorker = &extensionsv1alpha1.Worker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n1",
					Name:      "b",
					Labels:    bLabels,
				},
			}
			n2AWorker = &extensionsv1alpha1.Worker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n2",
					Name:      "a",
					Labels:    aLabels,
				},
			}
			n2BWorker = &extensionsv1alpha1.Worker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n2",
					Name:      "b",
					Labels:    bLabels,
				},
			}

			machineDeployments = []*extensionsv1alpha1.Worker{n1AWorker, n1BWorker, n2AWorker, n2BWorker}
		)

		DescribeTable("#List",
			func(source []*extensionsv1alpha1.Worker, selector labels.Selector, expected []*extensionsv1alpha1.Worker) {
				lister := NewWorkerLister(func() ([]*extensionsv1alpha1.Worker, error) {
					return source, nil
				})

				actual, err := lister.List(selector)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(expected))
			},
			Entry("everything", machineDeployments, labels.Everything(), machineDeployments),
			Entry("nothing", machineDeployments, labels.Nothing(), nil),
			Entry("a labels", machineDeployments, labels.SelectorFromSet(labels.Set(aLabels)), []*extensionsv1alpha1.Worker{n1AWorker, n2AWorker}),
			Entry("b labels", machineDeployments, labels.SelectorFromSet(labels.Set(bLabels)), []*extensionsv1alpha1.Worker{n1BWorker, n2BWorker}))

		Context("Workers", func() {
			DescribeTable("#List",
				func(source []*extensionsv1alpha1.Worker, namespace string, selector labels.Selector, expected []*extensionsv1alpha1.Worker) {
					lister := NewWorkerLister(func() ([]*extensionsv1alpha1.Worker, error) {
						return source, nil
					})

					actual, err := lister.Workers(namespace).List(selector)
					Expect(err).NotTo(HaveOccurred())
					Expect(actual).To(Equal(expected))
				},
				Entry("everything in n1", machineDeployments, "n1", labels.Everything(), []*extensionsv1alpha1.Worker{n1AWorker, n1BWorker}),
				Entry("nothing in n1", machineDeployments, "n1", labels.Nothing(), nil),
				Entry("a labels in n1", machineDeployments, "n1", labels.SelectorFromSet(labels.Set(aLabels)), []*extensionsv1alpha1.Worker{n1AWorker}),
				Entry("b labels in n1", machineDeployments, "n1", labels.SelectorFromSet(labels.Set(bLabels)), []*extensionsv1alpha1.Worker{n1BWorker}),
				Entry("everything in n2", machineDeployments, "n2", labels.Everything(), []*extensionsv1alpha1.Worker{n2AWorker, n2BWorker}),
				Entry("nothing in n2", machineDeployments, "n2", labels.Nothing(), nil),
				Entry("a labels in n2", machineDeployments, "n2", labels.SelectorFromSet(labels.Set(aLabels)), []*extensionsv1alpha1.Worker{n2AWorker}),
				Entry("b labels in n2", machineDeployments, "n2", labels.SelectorFromSet(labels.Set(bLabels)), []*extensionsv1alpha1.Worker{n2BWorker}))

			DescribeTable("#Get",
				func(source []*extensionsv1alpha1.Worker, namespace, name string, machineDeploymentMatcher, errMatcher gomegatypes.GomegaMatcher) {
					lister := NewWorkerLister(func() ([]*extensionsv1alpha1.Worker, error) {
						return source, nil
					})

					actual, err := lister.Workers(namespace).Get(name)
					Expect(err).To(errMatcher)
					Expect(actual).To(machineDeploymentMatcher)
				},
				Entry("a in n1", machineDeployments, "n1", "a", Equal(n1AWorker), Not(HaveOccurred())),
				Entry("b in n1", machineDeployments, "n1", "b", Equal(n1BWorker), Not(HaveOccurred())),
				Entry("c in n1", machineDeployments, "n1", "c", BeNil(), HaveOccurred()),
				Entry("a in n2", machineDeployments, "n2", "a", Equal(n2AWorker), Not(HaveOccurred())),
				Entry("b in n2", machineDeployments, "n2", "b", Equal(n2BWorker), Not(HaveOccurred())),
				Entry("c in n2", machineDeployments, "n2", "c", BeNil(), HaveOccurred()))
		})
	})

	Describe("#WaitUntilResourceDeleted", func() {
		var (
			namespace = "bar"
			name      = "foo"
			key       = Key(namespace, name)
			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
			}
		)

		It("should wait until the resource is deleted", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, key, configMap),
				c.EXPECT().Get(ctx, key, configMap),
				c.EXPECT().Get(ctx, key, configMap).Return(apierrors.NewNotFound(schema.GroupResource{}, name)),
			)

			Expect(WaitUntilResourceDeleted(ctx, c, configMap, time.Microsecond)).To(Succeed())
		})

		It("should timeout", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			gomock.InOrder(
				c.EXPECT().Get(ctx, key, configMap),
				c.EXPECT().Get(ctx, key, configMap).DoAndReturn(func(_ context.Context, _ client.ObjectKey, _ client.Object) error {
					cancel()
					return nil
				}),
			)

			Expect(WaitUntilResourceDeleted(ctx, c, configMap, time.Microsecond)).To(HaveOccurred())
		})

		It("return an unexpected error", func() {
			expectedErr := fmt.Errorf("unexpected")
			c.EXPECT().Get(ctx, key, configMap).Return(expectedErr)

			err := WaitUntilResourceDeleted(ctx, c, configMap, time.Microsecond)
			Expect(err).To(HaveOccurred())
			Expect(err).To(BeIdenticalTo(expectedErr))
		})
	})

	Describe("#WaitUntilResourcesDeleted", func() {
		var (
			configMap = corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
			}
			configMapList *corev1.ConfigMapList
		)

		BeforeEach(func() {
			configMapList = &corev1.ConfigMapList{}
		})

		It("should wait until the resources are deleted w/ empty list", func() {
			c.EXPECT().List(ctx, configMapList).Return(nil)

			Expect(WaitUntilResourcesDeleted(ctx, c, configMapList, time.Microsecond)).To(Succeed())
		})

		It("should timeout w/ remaining elements", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			c.EXPECT().List(ctx, configMapList).DoAndReturn(func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
				cancel()
				configMapList.Items = append(configMapList.Items, configMap)
				return nil
			})

			Expect(WaitUntilResourcesDeleted(ctx, c, configMapList, time.Microsecond)).To(HaveOccurred())
		})

		It("return an unexpected error", func() {
			expectedErr := fmt.Errorf("unexpected")
			c.EXPECT().List(ctx, configMapList).Return(expectedErr)

			Expect(WaitUntilResourcesDeleted(ctx, c, configMapList, time.Microsecond)).To(BeIdenticalTo(expectedErr))
		})
	})

	DescribeTable("#TruncateLabelValue",
		func(s, expected string) {
			Expect(TruncateLabelValue(s)).To(Equal(expected))
		},
		Entry("< 63 chars", "foo", "foo"),
		Entry("= 63 chars", strings.Repeat("a", 63), strings.Repeat("a", 63)),
		Entry("> 63 chars", strings.Repeat("a", 64), strings.Repeat("a", 63)))

	Describe("#GetLoadBalancerIngress", func() {
		var (
			key     = Key(namespace, name)
			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
		)

		It("should return an unexpected client error", func() {
			expectedErr := fmt.Errorf("unexpected")

			c.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(&corev1.Service{})).Return(expectedErr)

			_, err := GetLoadBalancerIngress(ctx, c, service)

			Expect(err).To(HaveOccurred())
			Expect(err).To(BeIdenticalTo(expectedErr))
		})

		It("should return an error because no ingresses found", func() {
			c.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(&corev1.Service{}))

			_, err := GetLoadBalancerIngress(ctx, c, service)

			Expect(err).To(MatchError("`.status.loadBalancer.ingress[]` has no elements yet, i.e. external load balancer has not been created"))
		})

		It("should return an ip address", func() {
			var (
				ctx        = context.TODO()
				expectedIP = "1.2.3.4"
			)

			c.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(&corev1.Service{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, service *corev1.Service) error {
				service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: expectedIP}}
				return nil
			})

			ingress, err := GetLoadBalancerIngress(ctx, c, service)

			Expect(ingress).To(Equal(expectedIP))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an hostname address", func() {
			var (
				ctx              = context.TODO()
				expectedHostname = "cluster.local"
			)

			c.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(&corev1.Service{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, service *corev1.Service) error {
				service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{Hostname: expectedHostname}}
				return nil
			})

			ingress, err := GetLoadBalancerIngress(ctx, c, service)

			Expect(ingress).To(Equal(expectedHostname))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if neither ip nor hostname were set", func() {
			c.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(&corev1.Service{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, service *corev1.Service) error {
				service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{}}
				return nil
			})

			_, err := GetLoadBalancerIngress(ctx, c, service)

			Expect(err).To(MatchError("`.status.loadBalancer.ingress[]` has an element which does neither contain `.ip` nor `.hostname`"))
		})
	})

	Describe("#LookupObject", func() {
		var (
			key       = Key(namespace, name)
			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
			}

			apiReader *mockclient.MockClient
		)

		BeforeEach(func() {
			apiReader = mockclient.NewMockClient(ctrl)
		})

		It("should retrieve the obj when cached client can retrieve it", func() {
			c.EXPECT().Get(ctx, key, configMap)

			err := LookupObject(context.TODO(), c, apiReader, key, configMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error when cached client fails with error other than NotFound error", func() {
			expectedErr := fmt.Errorf("unexpected")
			c.EXPECT().Get(ctx, key, configMap).Return(expectedErr)

			err := LookupObject(context.TODO(), c, apiReader, key, configMap)
			Expect(err).To(BeIdenticalTo(expectedErr))
		})

		It("should retrieve the obj using the apiReader when cached client fails with NotFound error", func() {
			c.EXPECT().Get(ctx, key, configMap).Return(apierrors.NewNotFound(schema.GroupResource{}, name))
			apiReader.EXPECT().Get(ctx, key, configMap)

			err := LookupObject(context.TODO(), c, apiReader, key, configMap)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	DescribeTable("#FeatureGatesToCommandLineParameter",
		func(fg map[string]bool, matcher gomegatypes.GomegaMatcher) {
			Expect(FeatureGatesToCommandLineParameter(fg)).To(matcher)
		},
		Entry("nil map", nil, BeEmpty()),
		Entry("empty map", map[string]bool{}, BeEmpty()),
		Entry("map with one entry", map[string]bool{"foo": true}, Equal("--feature-gates=foo=true")),
		Entry("map with multiple entries", map[string]bool{"foo": true, "bar": false, "baz": true}, Equal("--feature-gates=bar=false,baz=true,foo=true")),
	)

	DescribeTable("#MapStringBoolToCommandLineParameter",
		func(m map[string]bool, param string, matcher gomegatypes.GomegaMatcher) {
			Expect(MapStringBoolToCommandLineParameter(m, param)).To(matcher)
		},
		Entry("nil map", nil, "--some-param=", BeEmpty()),
		Entry("empty map", map[string]bool{}, "--some-param=", BeEmpty()),
		Entry("map with one entry", map[string]bool{"foo": true}, "--some-param=", Equal("--some-param=foo=true")),
		Entry("map with multiple entries", map[string]bool{"foo": true, "bar": false, "baz": true}, "--some-param=", Equal("--some-param=bar=false,baz=true,foo=true")),
	)

	var (
		port1 = corev1.ServicePort{
			Name:     "port1",
			Protocol: corev1.ProtocolTCP,
			Port:     1234,
		}
		port2 = corev1.ServicePort{
			Name:       "port2",
			Port:       1234,
			TargetPort: intstr.FromInt(5678),
		}
		port3 = corev1.ServicePort{
			Name:       "port3",
			Protocol:   corev1.ProtocolTCP,
			Port:       1234,
			TargetPort: intstr.FromInt(5678),
			NodePort:   9012,
		}
		desiredPorts = []corev1.ServicePort{port1, port2, port3}
	)

	DescribeTable("#ReconcileServicePorts",
		func(existingPorts []corev1.ServicePort, serviceType corev1.ServiceType, matcher gomegatypes.GomegaMatcher) {
			Expect(ReconcileServicePorts(existingPorts, desiredPorts, serviceType)).To(matcher)
		},
		Entry("existing ports is nil", nil, corev1.ServiceTypeLoadBalancer, ConsistOf(port1, port2, port3)),
		Entry("no existing ports", []corev1.ServicePort{}, corev1.ServiceTypeLoadBalancer, ConsistOf(port1, port2, port3)),
		Entry("existing but undesired ports", []corev1.ServicePort{{Name: "foo"}}, corev1.ServiceTypeLoadBalancer, ConsistOf(port1, port2, port3)),
		Entry("existing and desired ports when spec.type=LoadBalancer", []corev1.ServicePort{{Name: port1.Name, NodePort: 1337}}, corev1.ServiceTypeLoadBalancer, ConsistOf(corev1.ServicePort{Name: port1.Name, Protocol: port1.Protocol, Port: port1.Port, NodePort: 1337}, port2, port3)),
		Entry("existing and desired ports when spec.type=ClusterIP", []corev1.ServicePort{{Name: port1.Name, NodePort: 1337}}, corev1.ServiceTypeClusterIP, ConsistOf(port1, port2, port3)),
		Entry("existing and both desired and undesired ports", []corev1.ServicePort{{Name: "foo"}, {Name: port1.Name, NodePort: 1337}}, corev1.ServiceTypeLoadBalancer, ConsistOf(corev1.ServicePort{Name: port1.Name, Protocol: port1.Protocol, Port: port1.Port, NodePort: 1337}, port2, port3)),
	)

	Describe("#WaitUntilLoadBalancerIsReady", func() {
		var (
			k8sShootClient kubernetes.Interface
			key            = client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: "load-balancer"}
			logger         = logrus.NewEntry(logger.NewNopLogger())
			scheme         *runtime.Scheme
		)

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			c.EXPECT().Scheme().Return(scheme).AnyTimes()
			k8sShootClient = fakeclientset.NewClientSetBuilder().
				WithClient(c).
				Build()
		})

		It("should return nil when the Service has .status.loadBalancer.ingress[]", func() {
			var (
				svc = &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "load-balancer",
						Namespace: metav1.NamespaceSystem,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{
									Hostname: "cluster.local",
								},
							},
						},
					},
				}
			)

			gomock.InOrder(
				c.EXPECT().Get(gomock.Any(), key, gomock.AssignableToTypeOf(&corev1.Service{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, obj *corev1.Service) error {
						*obj = *svc
						return nil
					}),
			)

			actual, err := WaitUntilLoadBalancerIsReady(ctx, k8sShootClient.Client(), metav1.NamespaceSystem, "load-balancer", 1*time.Second, logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal("cluster.local"))
		})

		It("should return err when the Service has no .status.loadBalancer.ingress[]", func() {
			var (
				svc = &corev1.Service{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Service",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "load-balancer",
						Namespace: metav1.NamespaceSystem,
					},
					Status: corev1.ServiceStatus{},
				}
				event = corev1.Event{
					Source:         corev1.EventSource{Component: "service-controller"},
					Message:        "Error syncing load balancer: an error occurred",
					FirstTimestamp: metav1.NewTime(time.Date(2020, time.January, 15, 0, 0, 0, 0, time.UTC)),
					LastTimestamp:  metav1.NewTime(time.Date(2020, time.January, 15, 0, 0, 0, 0, time.UTC)),
					Count:          1,
					Type:           corev1.EventTypeWarning,
				}
			)

			gomock.InOrder(
				c.EXPECT().Get(gomock.Any(), key, gomock.AssignableToTypeOf(&corev1.Service{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, obj *corev1.Service) error {
						*obj = *svc
						return nil
					}),
				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.EventList{}), gomock.Any()).DoAndReturn(
					func(_ context.Context, list *corev1.EventList, _ ...client.ListOption) error {
						list.Items = append(list.Items, event)
						return nil
					}),
			)

			actual, err := WaitUntilLoadBalancerIsReady(ctx, k8sShootClient.Client(), metav1.NamespaceSystem, "load-balancer", 1*time.Second, logger)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("-> Events:\n* service-controller reported"))
			Expect(err.Error()).To(ContainSubstring("Error syncing load balancer: an error occurred"))
			Expect(actual).To(BeEmpty())
		})
	})

	Describe("#FetchEventMessages", func() {
		var (
			reader     *mockclient.MockReader
			events     []corev1.Event
			serviceObj *corev1.Service
			scheme     *runtime.Scheme
		)

		BeforeEach(func() {
			reader = mockclient.NewMockReader(ctrl)

			events = []corev1.Event{
				{
					Source:         corev1.EventSource{Component: "service-controller"},
					Message:        "Error syncing load balancer: first error occurred",
					FirstTimestamp: metav1.NewTime(time.Date(2020, time.January, 15, 0, 0, 0, 0, time.UTC)),
					LastTimestamp:  metav1.NewTime(time.Date(2020, time.January, 15, 0, 0, 0, 0, time.UTC)),
					Count:          1,
					Type:           corev1.EventTypeWarning,
				},
				{
					Source:         corev1.EventSource{Component: "service-controller"},
					Message:        "Error syncing load balancer: second error occurred",
					FirstTimestamp: metav1.NewTime(time.Date(2020, time.January, 15, 1, 0, 0, 0, time.UTC)),
					LastTimestamp:  metav1.NewTime(time.Date(2020, time.January, 15, 1, 0, 0, 0, time.UTC)),
					Count:          1,
					Type:           corev1.EventTypeWarning,
				},
			}

			serviceObj = &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Service",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
		})

		Context("when only objects of available scheme are used", func() {
			BeforeEach(func() {
				scheme = runtime.NewScheme()
				Expect(corev1.AddToScheme(scheme)).To(Succeed())
			})

			It("should return an event message with only the latest event", func() {
				var listOpts []client.ListOption
				gomock.InOrder(
					reader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.EventList{}), gomock.Any()).DoAndReturn(
						func(_ context.Context, list *corev1.EventList, listOptions ...client.ListOption) error {
							list.Items = append(list.Items, events...)
							listOpts = listOptions
							return nil
						}),
				)

				msg, err := FetchEventMessages(ctx, scheme, reader, serviceObj, corev1.EventTypeWarning, 1)

				Expect(listOpts).To(ContainElement(client.MatchingFields{
					"involvedObject.apiVersion": serviceObj.APIVersion,
					"involvedObject.kind":       serviceObj.Kind,
					"involvedObject.name":       serviceObj.Name,
					"involvedObject.namespace":  serviceObj.Namespace,
					"type":                      corev1.EventTypeWarning,
				}))
				Expect(err).To(Not(HaveOccurred()))
				Expect(msg).To(ContainSubstring("-> Events:\n* service-controller reported"))
				Expect(msg).To(ContainSubstring("second error occurred"))
			})

			It("should return an event message with all events", func() {
				var listOpts []client.ListOption
				gomock.InOrder(
					reader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.EventList{}), gomock.Any()).DoAndReturn(
						func(_ context.Context, list *corev1.EventList, listOptions ...client.ListOption) error {
							list.Items = append(list.Items, events...)
							listOpts = listOptions
							return nil
						}),
				)

				msg, err := FetchEventMessages(ctx, scheme, reader, serviceObj, corev1.EventTypeWarning, len(events))

				Expect(listOpts).To(ContainElement(client.MatchingFields{
					"involvedObject.apiVersion": serviceObj.APIVersion,
					"involvedObject.kind":       serviceObj.Kind,
					"involvedObject.name":       serviceObj.Name,
					"involvedObject.namespace":  serviceObj.Namespace,
					"type":                      corev1.EventTypeWarning,
				}))
				Expect(err).To(Not(HaveOccurred()))
				Expect(msg).To(ContainSubstring("-> Events:\n* service-controller reported"))
				Expect(msg).To(ContainSubstring("first error occurred"))
				Expect(msg).To(ContainSubstring("second error occurred"))
			})

			It("should not return a message because no events exist", func() {
				var listOpts []client.ListOption
				gomock.InOrder(
					reader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.EventList{}), gomock.Any()).DoAndReturn(
						func(_ context.Context, list *corev1.EventList, listOptions ...client.ListOption) error {
							listOpts = listOptions
							return nil
						}),
				)

				msg, err := FetchEventMessages(ctx, scheme, reader, serviceObj, corev1.EventTypeWarning, len(events))

				Expect(listOpts).To(ContainElement(client.MatchingFields{
					"involvedObject.apiVersion": serviceObj.APIVersion,
					"involvedObject.kind":       serviceObj.Kind,
					"involvedObject.name":       serviceObj.Name,
					"involvedObject.namespace":  serviceObj.Namespace,
					"type":                      corev1.EventTypeWarning,
				}))
				Expect(err).To(Not(HaveOccurred()))
				Expect(msg).To(BeEmpty())
			})

			It("should not return a message because an error occurred", func() {
				var listOpts []client.ListOption
				gomock.InOrder(
					reader.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.EventList{}), gomock.Any()).DoAndReturn(
						func(_ context.Context, list *corev1.EventList, listOptions ...client.ListOption) error {
							listOpts = listOptions
							return errors.New("foo")
						}),
				)

				msg, err := FetchEventMessages(ctx, scheme, reader, serviceObj, corev1.EventTypeWarning, len(events))

				Expect(listOpts).To(ContainElement(client.MatchingFields{
					"involvedObject.apiVersion": serviceObj.APIVersion,
					"involvedObject.kind":       serviceObj.Kind,
					"involvedObject.name":       serviceObj.Name,
					"involvedObject.namespace":  serviceObj.Namespace,
					"type":                      corev1.EventTypeWarning,
				}))
				Expect(err).To(MatchError("error 'foo' occurred while fetching more details"))
				Expect(msg).To(BeEmpty())
			})
		})

		Context("when object type is not in provided scheme", func() {
			BeforeEach(func() {
				scheme = runtime.NewScheme()
				Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())
			})

			It("should not return a message because type kind is not in scheme", func() {
				msg, err := FetchEventMessages(ctx, scheme, reader, &corev1.Service{}, corev1.EventTypeWarning, len(events))

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to identify GVK for object"))
				Expect(msg).To(BeEmpty())
			})
		})
	})

	Describe("#MergeOwnerReferences", func() {
		It("should merge the new references into the list of existing references", func() {
			var (
				references = []metav1.OwnerReference{
					{
						UID: types.UID("1234"),
					},
				}
				newReferences = []metav1.OwnerReference{
					{
						UID: types.UID("1234"),
					},
					{
						UID: types.UID("1235"),
					},
				}
			)

			Expect(MergeOwnerReferences(references, newReferences...)).To(ConsistOf(newReferences))
		})
	})

	DescribeTable("#OwnedBy",
		func(obj client.Object, apiVersion, kind, name string, uid types.UID, matcher gomegatypes.GomegaMatcher) {
			Expect(OwnedBy(obj, apiVersion, kind, name, uid)).To(matcher)
		},
		Entry("no owner references", &corev1.Pod{}, "apiVersion", "kind", "name", types.UID("uid"), BeFalse()),
		Entry("owner not found", &corev1.Pod{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{APIVersion: "different-apiVersion", Kind: "different-kind", Name: "different-name", UID: types.UID("different-uid")}}}}, "apiVersion", "kind", "name", types.UID("uid"), BeFalse()),
		Entry("owner found", &corev1.Pod{ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{APIVersion: "apiVersion", Kind: "kind", Name: "name", UID: types.UID("uid")}}}}, "apiVersion", "kind", "name", types.UID("uid"), BeTrue()),
	)

	Describe("#NewestObject", func() {
		var podList *corev1.PodList

		BeforeEach(func() {
			podList = &corev1.PodList{}
		})

		It("should return an error because the List() call failed", func() {
			c.EXPECT().List(ctx, podList).Return(fakeErr)

			obj, err := NewestObject(ctx, c, podList, nil)
			Expect(err).To(MatchError(fakeErr))
			Expect(obj).To(BeNil())
		})

		It("should return nil because the list does not contain items", func() {
			c.EXPECT().List(ctx, podList)

			obj, err := NewestObject(ctx, c, podList, nil)
			Expect(err).To(BeNil())
			Expect(obj).To(BeNil())
		})

		var (
			obj1 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "obj1", CreationTimestamp: metav1.Now()}}
			obj2 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "obj2", CreationTimestamp: metav1.Time{Time: time.Now().Add(+time.Hour)}}}
			obj3 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "obj3", CreationTimestamp: metav1.Time{Time: time.Now().Add(-time.Hour)}}}
		)

		It("should return the newest object w/o filter func", func() {
			c.EXPECT().List(ctx, podList).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
				*list = corev1.PodList{Items: []corev1.Pod{*obj1, *obj2, *obj3}}
				return nil
			})

			obj, err := NewestObject(ctx, c, podList, nil)
			Expect(err).To(BeNil())
			Expect(obj).To(Equal(obj2))
			Expect(podList.Items).To(Equal([]corev1.Pod{*obj3, *obj1, *obj2}))
		})

		It("should return the newest object w/ filter func", func() {
			filterFn := func(o client.Object) bool {
				obj := o.(*corev1.Pod)
				return obj.Name != "obj2"
			}

			c.EXPECT().List(ctx, podList).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
				*list = corev1.PodList{Items: []corev1.Pod{*obj1, *obj2, *obj3}}
				return nil
			})

			obj, err := NewestObject(ctx, c, podList, filterFn)
			Expect(err).To(BeNil())
			Expect(obj).To(Equal(obj1))
			Expect(podList.Items).To(Equal([]corev1.Pod{*obj3, *obj1}))
		})
	})

	Describe("#NewestPodForDeployment", func() {
		var (
			name      = "deployment-name"
			namespace = "deployment-namespace"
			uid       = types.UID("deployment-uid")
			labels    = map[string]string{"foo": "bar"}

			podTemplatehashKey = "pod-template-hash"
			rs1PodTemplateHash = "11111"
			rs2PodTemplateHash = "22222"
			rs3PodTemplateHash = "33333"
			rs1Labels          = map[string]string{
				"foo":              "bar",
				podTemplatehashKey: rs1PodTemplateHash,
			}
			rs2Labels = map[string]string{
				"foo":              "bar",
				podTemplatehashKey: rs2PodTemplateHash,
			}
			rs3Labels = map[string]string{
				"foo":              "bar",
				podTemplatehashKey: rs3PodTemplateHash,
			}

			rsListOptions = []interface{}{
				client.InNamespace(namespace),
				client.MatchingLabels(labels),
			}
			podListOptions []interface{}

			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					UID:       uid,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: labels,
					},
				},
			}

			replicaSet1 = &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:              name + "-" + rs1PodTemplateHash,
					Labels:            rs1Labels,
					UID:               "replicaset1",
					CreationTimestamp: metav1.Now(),
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       name,
						UID:        uid,
					}},
				},
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: rs1Labels,
					},
				},
			}
			replicaSet2 = &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:              name + "-" + rs2PodTemplateHash,
					Labels:            rs2Labels,
					UID:               "replicaset2",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(+time.Hour)},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "other-deployment",
						UID:        "other-uid",
					}},
				},
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: rs2Labels,
					},
				},
			}
			replicaSet3 = &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:              name + "-" + rs3PodTemplateHash,
					Labels:            rs3Labels,
					UID:               "replicaset3",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-time.Hour)},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       name,
						UID:        uid,
					}},
				},
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: rs3Labels,
					},
				},
			}

			pod1 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Name:              "pod1",
				UID:               "pod1",
				Labels:            rs1Labels,
				CreationTimestamp: metav1.Now(),
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       replicaSet1.Name,
					UID:        replicaSet1.UID,
				}},
			}}
			pod2 = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Name:              "pod2",
				UID:               "pod2",
				Labels:            rs1Labels,
				CreationTimestamp: metav1.Time{Time: time.Now().Add(-time.Hour)},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       replicaSet1.Name,
					UID:        replicaSet1.UID,
				}},
			}}
		)

		BeforeEach(func() {
			podSelector, err := metav1.LabelSelectorAsSelector(replicaSet1.Spec.Selector)
			Expect(err).To(BeNil())
			podListOptions = append(rsListOptions, client.MatchingLabelsSelector{Selector: podSelector})
		})

		It("should return an error because the newest ReplicaSet determination failed", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), rsListOptions...).Return(fakeErr)

			pod, err := NewestPodForDeployment(ctx, c, deployment)
			Expect(err).To(MatchError(fakeErr))
			Expect(pod).To(BeNil())
		})

		It("should return nil because no replica set found", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), rsListOptions...)

			pod, err := NewestPodForDeployment(ctx, c, deployment)
			Expect(err).To(BeNil())
			Expect(pod).To(BeNil())
		})

		It("should return an error because listing pods failed", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), rsListOptions...).DoAndReturn(func(_ context.Context, list *appsv1.ReplicaSetList, _ ...client.ListOption) error {
				*list = appsv1.ReplicaSetList{Items: []appsv1.ReplicaSet{*replicaSet1, *replicaSet2, *replicaSet3}}
				return nil
			})
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.PodList{}), podListOptions...).Return(fakeErr)

			pod, err := NewestPodForDeployment(ctx, c, deployment)
			Expect(err).To(MatchError(fakeErr))
			Expect(pod).To(BeNil())
		})

		It("should return an error because the replicasSet has no pod selector", func() {
			rs := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
				Name:              "rs",
				Labels:            rs1Labels,
				UID:               "rs",
				CreationTimestamp: metav1.Now(),
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       name,
					UID:        uid,
				}},
			}}
			rsError := fmt.Errorf("no pod selector specified in replicaSet %s/%s", rs.Namespace, rs.Name)

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), rsListOptions...).DoAndReturn(func(_ context.Context, list *appsv1.ReplicaSetList, _ ...client.ListOption) error {
				*list = appsv1.ReplicaSetList{Items: []appsv1.ReplicaSet{*rs}}
				return nil
			})

			pod, err := NewestPodForDeployment(ctx, c, deployment)
			Expect(err).To(MatchError(rsError))
			Expect(pod).To(BeNil())
		})

		It("should return an error because the replicasSet has no matchLabels or matchExpressions", func() {
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "rs",
					Labels:            rs1Labels,
					UID:               "rs",
					CreationTimestamp: metav1.Now(),
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       name,
						UID:        uid,
					}},
				},
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels:      map[string]string{},
						MatchExpressions: []metav1.LabelSelectorRequirement{},
					}},
			}
			rsError := fmt.Errorf("no matchLabels or matchExpressions specified in replicaSet %s/%s", rs.Namespace, rs.Name)

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), rsListOptions...).DoAndReturn(func(_ context.Context, list *appsv1.ReplicaSetList, _ ...client.ListOption) error {
				*list = appsv1.ReplicaSetList{Items: []appsv1.ReplicaSet{*rs}}
				return nil
			})

			pod, err := NewestPodForDeployment(ctx, c, deployment)
			Expect(err).To(MatchError(rsError))
			Expect(pod).To(BeNil())
		})

		It("should return an error because the matchExpressions is invalid", func() {
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "rs",
					Labels:            rs1Labels,
					UID:               "rs",
					CreationTimestamp: metav1.Now(),
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       name,
						UID:        uid,
					}},
				},
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "foo", Operator: metav1.LabelSelectorOpIn, Values: []string{}}},
					}},
			}

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), rsListOptions...).DoAndReturn(func(_ context.Context, list *appsv1.ReplicaSetList, _ ...client.ListOption) error {
				*list = appsv1.ReplicaSetList{Items: []appsv1.ReplicaSet{*rs}}
				return nil
			})

			pod, err := NewestPodForDeployment(ctx, c, deployment)
			Expect(err).To(MatchError(ContainSubstring("for 'in', 'notin' operators, values set can't be empty")))
			Expect(pod).To(BeNil())
		})

		It("should return nil because no pod was found", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), rsListOptions...).DoAndReturn(func(_ context.Context, list *appsv1.ReplicaSetList, _ ...client.ListOption) error {
				*list = appsv1.ReplicaSetList{Items: []appsv1.ReplicaSet{*replicaSet1, *replicaSet2, *replicaSet3}}
				return nil
			})
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.PodList{}), podListOptions...)

			pod, err := NewestPodForDeployment(ctx, c, deployment)
			Expect(err).To(BeNil())
			Expect(pod).To(BeNil())
		})

		It("should return the newest pod", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&appsv1.ReplicaSetList{}), rsListOptions...).DoAndReturn(func(_ context.Context, list *appsv1.ReplicaSetList, _ ...client.ListOption) error {
				*list = appsv1.ReplicaSetList{Items: []appsv1.ReplicaSet{*replicaSet1, *replicaSet2, *replicaSet3}}
				return nil
			})
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.PodList{}), podListOptions...).DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
				*list = corev1.PodList{Items: []corev1.Pod{*pod1, *pod2}}
				return nil
			})

			pod, err := NewestPodForDeployment(ctx, c, deployment)
			Expect(err).To(BeNil())
			Expect(pod).To(Equal(pod1))
		})
	})

	Describe("#MostRecentCompleteLogs", func() {
		var (
			pods   *mockcorev1.MockPodInterface
			body   *mockio.MockReadCloser
			client *http.Client

			pod           *corev1.Pod
			podName       = "pod"
			containerName = "container"
		)

		BeforeEach(func() {
			pods = mockcorev1.NewMockPodInterface(ctrl)
			body = mockio.NewMockReadCloser(ctrl)
			client = fakerestclient.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
			})

			pod = &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: podName}}
		})

		It("should return an error if the log retrieval failed", func() {
			gomock.InOrder(
				pods.EXPECT().GetLogs(podName, gomock.AssignableToTypeOf(&corev1.PodLogOptions{})).Return(rest.NewRequestWithClient(&url.URL{}, "", rest.ClientContentConfig{}, client)),
				body.EXPECT().Read(gomock.Any()).Return(0, fakeErr),
				body.EXPECT().Close(),
			)

			actual, err := MostRecentCompleteLogs(ctx, pods, pod, containerName, nil, nil)
			Expect(err).To(MatchError(fakeErr))
			Expect(actual).To(BeEmpty())
		})

		DescribeTable("#OwnedBy",
			func(containerStatuses []corev1.ContainerStatus, containerName string, previous bool) {
				var (
					headBytes int64 = 1024
					tailLines int64 = 1337
					logs            = []byte("logs")
				)

				pod.Status.ContainerStatuses = containerStatuses

				tailLineOptions := &corev1.PodLogOptions{
					Container: containerName,
					TailLines: &tailLines,
					Previous:  previous,
				}

				bytesLimitOptions := &corev1.PodLogOptions{
					Container:  containerName,
					LimitBytes: &headBytes,
					Previous:   previous,
				}

				gomock.InOrder(
					pods.EXPECT().GetLogs(podName, tailLineOptions).Return(rest.NewRequestWithClient(&url.URL{}, "", rest.ClientContentConfig{}, client)),
					body.EXPECT().Read(gomock.Any()).DoAndReturn(func(data []byte) (int, error) {
						copy(data, logs)
						return len(logs), io.EOF
					}),
					body.EXPECT().Close(),

					pods.EXPECT().GetLogs(podName, bytesLimitOptions).Return(rest.NewRequestWithClient(&url.URL{}, "", rest.ClientContentConfig{}, client)),
					body.EXPECT().Read(gomock.Any()).DoAndReturn(func(data []byte) (int, error) {
						copy(data, logs)
						return len(logs), io.EOF
					}),
					body.EXPECT().Close(),
				)

				actual, err := MostRecentCompleteLogs(ctx, pods, pod, containerName, &tailLines, &headBytes)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(fmt.Sprintf("%s\n...\n%s", logs, logs)))
			},

			Entry("w/o container name, logs of current  container", []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}}}}, "", false),
			Entry("w/o container name, logs of previous container", []corev1.ContainerStatus{{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}, "", true),
			Entry("w/  container name, logs of current  container", []corev1.ContainerStatus{{}, {Name: containerName, State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}}}}, containerName, false),
			Entry("w/  container name, logs of previous container", []corev1.ContainerStatus{{}, {Name: containerName, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}, containerName, true),
		)
	})

	Describe("#CertificatesV1beta1UsagesToCertificatesV1Usages", func() {
		It("should correctly convert", func() {
			Expect(CertificatesV1beta1UsagesToCertificatesV1Usages([]certificatesv1beta1.KeyUsage{
				certificatesv1beta1.UsageSigning,
				certificatesv1beta1.UsageDigitalSignature,
				certificatesv1beta1.UsageContentCommitment,
				certificatesv1beta1.UsageKeyEncipherment,
				certificatesv1beta1.UsageKeyAgreement,
				certificatesv1beta1.UsageDataEncipherment,
				certificatesv1beta1.UsageCertSign,
				certificatesv1beta1.UsageCRLSign,
				certificatesv1beta1.UsageEncipherOnly,
				certificatesv1beta1.UsageDecipherOnly,
				certificatesv1beta1.UsageAny,
				certificatesv1beta1.UsageServerAuth,
				certificatesv1beta1.UsageClientAuth,
				certificatesv1beta1.UsageCodeSigning,
				certificatesv1beta1.UsageEmailProtection,
				certificatesv1beta1.UsageSMIME,
				certificatesv1beta1.UsageIPsecEndSystem,
				certificatesv1beta1.UsageIPsecTunnel,
				certificatesv1beta1.UsageIPsecUser,
				certificatesv1beta1.UsageTimestamping,
				certificatesv1beta1.UsageOCSPSigning,
				certificatesv1beta1.UsageMicrosoftSGC,
				certificatesv1beta1.UsageNetscapeSGC,
			})).To(Equal([]certificatesv1.KeyUsage{
				certificatesv1.UsageSigning,
				certificatesv1.UsageDigitalSignature,
				certificatesv1.UsageContentCommitment,
				certificatesv1.UsageKeyEncipherment,
				certificatesv1.UsageKeyAgreement,
				certificatesv1.UsageDataEncipherment,
				certificatesv1.UsageCertSign,
				certificatesv1.UsageCRLSign,
				certificatesv1.UsageEncipherOnly,
				certificatesv1.UsageDecipherOnly,
				certificatesv1.UsageAny,
				certificatesv1.UsageServerAuth,
				certificatesv1.UsageClientAuth,
				certificatesv1.UsageCodeSigning,
				certificatesv1.UsageEmailProtection,
				certificatesv1.UsageSMIME,
				certificatesv1.UsageIPsecEndSystem,
				certificatesv1.UsageIPsecTunnel,
				certificatesv1.UsageIPsecUser,
				certificatesv1.UsageTimestamping,
				certificatesv1.UsageOCSPSigning,
				certificatesv1.UsageMicrosoftSGC,
				certificatesv1.UsageNetscapeSGC,
			}))
		})
	})
})
