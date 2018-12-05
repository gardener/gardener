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
	"testing"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestKubernetes(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kubernetes Suite")
}

var _ = Describe("kubernetes", func() {
	Describe("#CreateTwoWayMergePatch", func() {
		It("should fail for two different object types", func() {
			_, err := CreateTwoWayMergePatch(&corev1.ConfigMap{}, &corev1.Secret{})
			Expect(err).To(HaveOccurred())
		})

		It("Should correctly create a patch", func() {
			patch, err := CreateTwoWayMergePatch(
				&corev1.ConfigMap{Data: map[string]string{"foo": "bar"}},
				&corev1.ConfigMap{Data: map[string]string{"foo": "baz"}})

			Expect(err).NotTo(HaveOccurred())
			Expect(patch).To(Equal([]byte(`{"data":{"foo":"baz"}}`)))
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

	DescribeTable("#IsEmptyPatch",
		func(patch string, expected bool) {
			Expect(IsEmptyPatch([]byte(patch))).To(Equal(expected))
		},
		Entry("non-empty-patch", `{"foo": "bar"}`, false),
		Entry("non-json-patch", `random input`, false),
		Entry("empty string", ``, true),
		Entry("empty string with spaces", `  `, true),
		Entry("empty json object", `{}`, true),
		Entry("empty json object with spaces", ` { } `, true),
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
									Name:  "aws-lb-readvertiser",
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
		Entry("invalid version", "aws-lb-readvertiser", `0.4.0`, false),
		Entry("invalid container name", "aws-readvertiser", "0.3.0", false),
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
				func(source []*appsv1.Deployment, namespace, name string, deploymentMatcher, errMatcher types.GomegaMatcher) {
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
				func(source []*appsv1.StatefulSet, namespace, name string, statefulSetMatcher, errMatcher types.GomegaMatcher) {
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
				func(source []*appsv1.DaemonSet, namespace, name string, daemonSetMatcher, errMatcher types.GomegaMatcher) {
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

	Context("MachineDeploymentLister", func() {
		var (
			aLabels = map[string]string{"label": "a"}
			bLabels = map[string]string{"label": "b"}

			n1AMachineDeployment = &machinev1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n1",
					Name:      "a",
					Labels:    aLabels,
				},
			}
			n1BMachineDeployment = &machinev1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n1",
					Name:      "b",
					Labels:    bLabels,
				},
			}
			n2AMachineDeployment = &machinev1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n2",
					Name:      "a",
					Labels:    aLabels,
				},
			}
			n2BMachineDeployment = &machinev1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n2",
					Name:      "b",
					Labels:    bLabels,
				},
			}

			machineDeployments = []*machinev1alpha1.MachineDeployment{n1AMachineDeployment, n1BMachineDeployment, n2AMachineDeployment, n2BMachineDeployment}
		)

		DescribeTable("#List",
			func(source []*machinev1alpha1.MachineDeployment, selector labels.Selector, expected []*machinev1alpha1.MachineDeployment) {
				lister := NewMachineDeploymentLister(func() ([]*machinev1alpha1.MachineDeployment, error) {
					return source, nil
				})

				actual, err := lister.List(selector)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(expected))
			},
			Entry("everything", machineDeployments, labels.Everything(), machineDeployments),
			Entry("nothing", machineDeployments, labels.Nothing(), nil),
			Entry("a labels", machineDeployments, labels.SelectorFromSet(labels.Set(aLabels)), []*machinev1alpha1.MachineDeployment{n1AMachineDeployment, n2AMachineDeployment}),
			Entry("b labels", machineDeployments, labels.SelectorFromSet(labels.Set(bLabels)), []*machinev1alpha1.MachineDeployment{n1BMachineDeployment, n2BMachineDeployment}))

		Context("MachineDeployments", func() {
			DescribeTable("#List",
				func(source []*machinev1alpha1.MachineDeployment, namespace string, selector labels.Selector, expected []*machinev1alpha1.MachineDeployment) {
					lister := NewMachineDeploymentLister(func() ([]*machinev1alpha1.MachineDeployment, error) {
						return source, nil
					})

					actual, err := lister.MachineDeployments(namespace).List(selector)
					Expect(err).NotTo(HaveOccurred())
					Expect(actual).To(Equal(expected))
				},
				Entry("everything in n1", machineDeployments, "n1", labels.Everything(), []*machinev1alpha1.MachineDeployment{n1AMachineDeployment, n1BMachineDeployment}),
				Entry("nothing in n1", machineDeployments, "n1", labels.Nothing(), nil),
				Entry("a labels in n1", machineDeployments, "n1", labels.SelectorFromSet(labels.Set(aLabels)), []*machinev1alpha1.MachineDeployment{n1AMachineDeployment}),
				Entry("b labels in n1", machineDeployments, "n1", labels.SelectorFromSet(labels.Set(bLabels)), []*machinev1alpha1.MachineDeployment{n1BMachineDeployment}),
				Entry("everything in n2", machineDeployments, "n2", labels.Everything(), []*machinev1alpha1.MachineDeployment{n2AMachineDeployment, n2BMachineDeployment}),
				Entry("nothing in n2", machineDeployments, "n2", labels.Nothing(), nil),
				Entry("a labels in n2", machineDeployments, "n2", labels.SelectorFromSet(labels.Set(aLabels)), []*machinev1alpha1.MachineDeployment{n2AMachineDeployment}),
				Entry("b labels in n2", machineDeployments, "n2", labels.SelectorFromSet(labels.Set(bLabels)), []*machinev1alpha1.MachineDeployment{n2BMachineDeployment}))

			DescribeTable("#Get",
				func(source []*machinev1alpha1.MachineDeployment, namespace, name string, machineDeploymentMatcher, errMatcher types.GomegaMatcher) {
					lister := NewMachineDeploymentLister(func() ([]*machinev1alpha1.MachineDeployment, error) {
						return source, nil
					})

					actual, err := lister.MachineDeployments(namespace).Get(name)
					Expect(err).To(errMatcher)
					Expect(actual).To(machineDeploymentMatcher)
				},
				Entry("a in n1", machineDeployments, "n1", "a", Equal(n1AMachineDeployment), Not(HaveOccurred())),
				Entry("b in n1", machineDeployments, "n1", "b", Equal(n1BMachineDeployment), Not(HaveOccurred())),
				Entry("c in n1", machineDeployments, "n1", "c", BeNil(), HaveOccurred()),
				Entry("a in n2", machineDeployments, "n2", "a", Equal(n2AMachineDeployment), Not(HaveOccurred())),
				Entry("b in n2", machineDeployments, "n2", "b", Equal(n2BMachineDeployment), Not(HaveOccurred())),
				Entry("c in n2", machineDeployments, "n2", "c", BeNil(), HaveOccurred()))
		})
	})

	Context("NodeLister", func() {
		var (
			aLabels = map[string]string{"label": "a"}
			bLabels = map[string]string{"label": "b"}

			n1ANode = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n1",
					Name:      "a",
					Labels:    aLabels,
				},
			}
			n1BNode = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n1",
					Name:      "b",
					Labels:    bLabels,
				},
			}
			n2ANode = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n2",
					Name:      "a",
					Labels:    aLabels,
				},
			}
			n2BNode = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "n2",
					Name:      "b",
					Labels:    bLabels,
				},
			}

			nodes = []*corev1.Node{n1ANode, n1BNode, n2ANode, n2BNode}
		)

		DescribeTable("#List",
			func(source []*corev1.Node, selector labels.Selector, expected []*corev1.Node) {
				lister := NewNodeLister(func() ([]*corev1.Node, error) {
					return source, nil
				})

				actual, err := lister.List(selector)
				Expect(err).NotTo(HaveOccurred())
				Expect(actual).To(Equal(expected))
			},
			Entry("everything", nodes, labels.Everything(), nodes),
			Entry("nothing", nodes, labels.Nothing(), nil),
			Entry("a labels", nodes, labels.SelectorFromSet(labels.Set(aLabels)), []*corev1.Node{n1ANode, n2ANode}),
			Entry("b labels", nodes, labels.SelectorFromSet(labels.Set(bLabels)), []*corev1.Node{n1BNode, n2BNode}))
	})
})
