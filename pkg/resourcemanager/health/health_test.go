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

package health_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/resourcemanager/health"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestHealth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Health Suite")
}

var _ = Describe("health", func() {
	Context("CheckCustomResourceDefinition", func() {
		DescribeTable("crds",
			func(crd *apiextensionsv1.CustomResourceDefinition, matcher types.GomegaMatcher) {
				err := health.CheckCustomResourceDefinition(crd)
				Expect(err).To(matcher)
			},
			Entry("terminating", &apiextensionsv1.CustomResourceDefinition{
				Status: apiextensionsv1.CustomResourceDefinitionStatus{
					Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
						{
							Type:   apiextensionsv1.NamesAccepted,
							Status: apiextensionsv1.ConditionTrue,
						},
						{
							Type:   apiextensionsv1.Established,
							Status: apiextensionsv1.ConditionTrue,
						},
						{
							Type:   apiextensionsv1.Terminating,
							Status: apiextensionsv1.ConditionTrue,
						},
					},
				},
			}, HaveOccurred()),
			Entry("with conflicting name", &apiextensionsv1.CustomResourceDefinition{
				Status: apiextensionsv1.CustomResourceDefinitionStatus{
					Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
						{
							Type:   apiextensionsv1.NamesAccepted,
							Status: apiextensionsv1.ConditionFalse,
						},
						{
							Type:   apiextensionsv1.Established,
							Status: apiextensionsv1.ConditionFalse,
						},
					},
				},
			}, HaveOccurred()),
			Entry("healthy", &apiextensionsv1.CustomResourceDefinition{
				Status: apiextensionsv1.CustomResourceDefinitionStatus{
					Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
						{
							Type:   apiextensionsv1.NamesAccepted,
							Status: apiextensionsv1.ConditionTrue,
						},
						{
							Type:   apiextensionsv1.Established,
							Status: apiextensionsv1.ConditionTrue,
						},
					},
				},
			}, BeNil()),
		)
	})

	Context("CheckDaemonSet", func() {
		oneUnavailable := intstr.FromInt(1)

		DescribeTable("daemonsets",
			func(daemonSet *appsv1.DaemonSet, matcher types.GomegaMatcher) {
				err := health.CheckDaemonSet(daemonSet)
				Expect(err).To(matcher)
			},
			Entry("not observed at latest version", &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
			}, HaveOccurred()),
			Entry("not enough scheduled", &appsv1.DaemonSet{
				Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 1},
			}, HaveOccurred()),
			Entry("misscheduled pods", &appsv1.DaemonSet{
				Status: appsv1.DaemonSetStatus{NumberMisscheduled: 1},
			}, HaveOccurred()),
			Entry("too many unavailable pods", &appsv1.DaemonSet{
				Spec: appsv1.DaemonSetSpec{UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
					Type: appsv1.RollingUpdateDaemonSetStrategyType,
					RollingUpdate: &appsv1.RollingUpdateDaemonSet{
						MaxUnavailable: &oneUnavailable,
					},
				}},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 2,
					CurrentNumberScheduled: 2,
					NumberUnavailable:      2,
					NumberReady:            0,
				},
			}, HaveOccurred()),
			Entry("too less ready pods", &appsv1.DaemonSet{
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 1,
					CurrentNumberScheduled: 1,
				},
			}, HaveOccurred()),
			Entry("healthy", &appsv1.DaemonSet{
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 1,
					CurrentNumberScheduled: 1,
					NumberReady:            1,
				},
			}, BeNil()),
		)
	})

	Context("CheckDeployment", func() {
		DescribeTable("deployments",
			func(deployment *appsv1.Deployment, matcher types.GomegaMatcher) {
				err := health.CheckDeployment(deployment)
				Expect(err).To(matcher)
			},
			Entry("healthy", &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{
					{
						Type:   appsv1.DeploymentAvailable,
						Status: corev1.ConditionTrue,
					},
				}},
			}, BeNil()),
			Entry("healthy with progressing", &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{
					{
						Type:   appsv1.DeploymentAvailable,
						Status: corev1.ConditionTrue,
					},
					{
						Type:   appsv1.DeploymentProgressing,
						Status: corev1.ConditionTrue,
					},
				}},
			}, BeNil()),
			Entry("not observed at latest version", &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
			}, HaveOccurred()),
			Entry("not available", &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{
					{
						Type:   appsv1.DeploymentAvailable,
						Status: corev1.ConditionFalse,
					},
					{
						Type:   appsv1.DeploymentProgressing,
						Status: corev1.ConditionTrue,
					},
				}},
			}, HaveOccurred()),
			Entry("not progressing", &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{
					{
						Type:   appsv1.DeploymentAvailable,
						Status: corev1.ConditionTrue,
					},
					{
						Type:   appsv1.DeploymentProgressing,
						Status: corev1.ConditionFalse,
					},
				}},
			}, HaveOccurred()),
			Entry("available | progressing missing", &appsv1.Deployment{}, HaveOccurred()),
		)
	})

	Context("CheckPod", func() {
		DescribeTable("pods",
			func(pod *corev1.Pod, matcher types.GomegaMatcher) {
				err := health.CheckPod(pod)
				Expect(err).To(matcher)
			},
			Entry("pending", &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			}, HaveOccurred()),
			Entry("running", &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			}, BeNil()),
			Entry("succeeded", &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodSucceeded,
				},
			}, BeNil()),
		)
	})

	Context("CheckReplicaSet", func() {
		DescribeTable("replicasets",
			func(rs *appsv1.ReplicaSet, matcher types.GomegaMatcher) {
				err := health.CheckReplicaSet(rs)
				Expect(err).To(matcher)
			},
			Entry("not observed at latest version", &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
			}, HaveOccurred()),
			Entry("not enough ready replicas", &appsv1.ReplicaSet{
				Spec:   appsv1.ReplicaSetSpec{Replicas: replicas(2)},
				Status: appsv1.ReplicaSetStatus{ReadyReplicas: 1},
			}, HaveOccurred()),
			Entry("healthy", &appsv1.ReplicaSet{
				Spec:   appsv1.ReplicaSetSpec{Replicas: replicas(2)},
				Status: appsv1.ReplicaSetStatus{ReadyReplicas: 2},
			}, BeNil()),
		)
	})

	Context("CheckReplicationController", func() {
		DescribeTable("replicationcontroller",
			func(rc *corev1.ReplicationController, matcher types.GomegaMatcher) {
				err := health.CheckReplicationController(rc)
				Expect(err).To(matcher)
			},
			Entry("not observed at latest version", &corev1.ReplicationController{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
			}, HaveOccurred()),
			Entry("not enough ready replicas", &corev1.ReplicationController{
				Spec:   corev1.ReplicationControllerSpec{Replicas: replicas(2)},
				Status: corev1.ReplicationControllerStatus{ReadyReplicas: 1},
			}, HaveOccurred()),
			Entry("healthy", &corev1.ReplicationController{
				Spec:   corev1.ReplicationControllerSpec{Replicas: replicas(2)},
				Status: corev1.ReplicationControllerStatus{ReadyReplicas: 2},
			}, BeNil()),
		)
	})

	Context("CheckManagedResource", func() {
		DescribeTable("managedresource",
			func(mr v1alpha1.ManagedResource, matcher types.GomegaMatcher) {
				err := health.CheckManagedResource(&mr)
				Expect(err).To(matcher)
			},
			Entry("applied condition not true", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: v1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []v1alpha1.ManagedResourceCondition{
						{
							Type:   v1alpha1.ResourcesApplied,
							Status: v1alpha1.ConditionFalse,
						},
						{
							Type:   v1alpha1.ResourcesHealthy,
							Status: v1alpha1.ConditionTrue,
						},
					},
				},
			}, HaveOccurred()),
			Entry("healthy condition not true", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: v1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []v1alpha1.ManagedResourceCondition{
						{
							Type:   v1alpha1.ResourcesApplied,
							Status: v1alpha1.ConditionTrue,
						},
						{
							Type:   v1alpha1.ResourcesHealthy,
							Status: v1alpha1.ConditionFalse,
						},
					},
				},
			}, HaveOccurred()),
			Entry("conditions true", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: v1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []v1alpha1.ManagedResourceCondition{
						{
							Type:   v1alpha1.ResourcesApplied,
							Status: v1alpha1.ConditionTrue,
						},
						{
							Type:   v1alpha1.ResourcesHealthy,
							Status: v1alpha1.ConditionTrue,
						},
					},
				},
			}, Not(HaveOccurred())),
			Entry("no applied condition", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: v1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []v1alpha1.ManagedResourceCondition{
						{
							Type:   v1alpha1.ResourcesHealthy,
							Status: v1alpha1.ConditionTrue,
						},
					},
				},
			}, HaveOccurred()),
			Entry("no healthy condition", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: v1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []v1alpha1.ManagedResourceCondition{
						{
							Type:   v1alpha1.ResourcesApplied,
							Status: v1alpha1.ConditionTrue,
						},
					},
				},
			}, HaveOccurred()),
			Entry("no conditions", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: v1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
				},
			}, HaveOccurred()),
			Entry("outdated generation", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 2},
				Status: v1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
				},
			}, HaveOccurred()),
			Entry("no status", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 2},
			}, HaveOccurred()),
		)
	})

	Context("CheckManagedResourceApplied", func() {
		DescribeTable("managedresource",
			func(mr v1alpha1.ManagedResource, matcher types.GomegaMatcher) {
				err := health.CheckManagedResourceApplied(&mr)
				Expect(err).To(matcher)
			},
			Entry("applied condition not true", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: v1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []v1alpha1.ManagedResourceCondition{
						{
							Type:   v1alpha1.ResourcesApplied,
							Status: v1alpha1.ConditionFalse,
						},
					},
				},
			}, HaveOccurred()),
			Entry("condition true", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: v1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []v1alpha1.ManagedResourceCondition{
						{
							Type:   v1alpha1.ResourcesApplied,
							Status: v1alpha1.ConditionTrue,
						},
					},
				},
			}, Not(HaveOccurred())),
			Entry("no applied condition", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: v1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions:         []v1alpha1.ManagedResourceCondition{},
				},
			}, HaveOccurred()),
			Entry("no conditions", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: v1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
				},
			}, HaveOccurred()),
			Entry("outdated generation", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 2},
				Status: v1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
				},
			}, HaveOccurred()),
			Entry("no status", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 2},
			}, HaveOccurred()),
		)
	})

	Context("CheckManagedResourceHealthy", func() {
		DescribeTable("managedresource",
			func(mr v1alpha1.ManagedResource, matcher types.GomegaMatcher) {
				err := health.CheckManagedResourceHealthy(&mr)
				Expect(err).To(matcher)
			},
			Entry("healthy condition not true", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: v1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []v1alpha1.ManagedResourceCondition{
						{
							Type:   v1alpha1.ResourcesHealthy,
							Status: v1alpha1.ConditionFalse,
						},
					},
				},
			}, HaveOccurred()),
			Entry("condition true", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: v1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []v1alpha1.ManagedResourceCondition{
						{
							Type:   v1alpha1.ResourcesHealthy,
							Status: v1alpha1.ConditionTrue,
						},
					},
				},
			}, Not(HaveOccurred())),
			Entry("no healthy condition", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: v1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions:         []v1alpha1.ManagedResourceCondition{},
				},
			}, HaveOccurred()),
			Entry("no conditions", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: v1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
				},
			}, HaveOccurred()),
			Entry("no status", v1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Generation: 2},
			}, HaveOccurred()),
		)
	})

	Context("CheckStatefulSet", func() {
		DescribeTable("statefulsets",
			func(statefulSet *appsv1.StatefulSet, matcher types.GomegaMatcher) {
				err := health.CheckStatefulSet(statefulSet)
				Expect(err).To(matcher)
			},
			Entry("healthy", &appsv1.StatefulSet{
				Spec:   appsv1.StatefulSetSpec{Replicas: replicas(1)},
				Status: appsv1.StatefulSetStatus{CurrentReplicas: 1, ReadyReplicas: 1},
			}, BeNil()),
			Entry("healthy with nil replicas", &appsv1.StatefulSet{
				Status: appsv1.StatefulSetStatus{ReadyReplicas: 1},
			}, BeNil()),
			Entry("not observed at latest version", &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
			}, HaveOccurred()),
			Entry("not enough ready replicas", &appsv1.StatefulSet{
				Spec:   appsv1.StatefulSetSpec{Replicas: replicas(2)},
				Status: appsv1.StatefulSetStatus{ReadyReplicas: 1},
			}, HaveOccurred()),
		)
	})

	Context("CheckService", func() {
		var (
			ctrl    *gomock.Controller
			message = "foo"
		)
		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
		})
		AfterEach(func() {
			ctrl.Finish()
		})

		DescribeTable("services",
			func(service *corev1.Service, matcher types.GomegaMatcher) {
				scheme := kubernetesscheme.Scheme
				c := mockclient.NewMockClient(ctrl)
				c.EXPECT().Scheme().Return(kubernetesscheme.Scheme).AnyTimes()

				c.EXPECT().List(gomock.Any(), gomock.AssignableToTypeOf(&corev1.EventList{}), gomock.Any()).DoAndReturn(
					func(_ context.Context, list *corev1.EventList, _ ...client.ListOption) error {
						list.Items = []corev1.Event{
							{Message: message},
						}
						return nil
					},
				).MaxTimes(1)
				err := health.CheckService(context.Background(), scheme, c, service)
				Expect(err).To(matcher)
			},
			Entry("no LoadBalancer service", &corev1.Service{
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName},
			}, BeNil()),
			Entry("LoadBalancer w/ ingress status", &corev1.Service{
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{Hostname: "foo.bar"},
						},
					},
				},
			}, BeNil()),
			Entry("LoadBalancer w/o ingress status", &corev1.Service{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
				Spec:     corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
			}, MatchError(fmt.Sprintf("service is missing ingress status\n\n-> Events:\n*  reported <unknown> ago: %s", message))),
		)
	})
})

func replicas(i int32) *int32 {
	return &i
}
