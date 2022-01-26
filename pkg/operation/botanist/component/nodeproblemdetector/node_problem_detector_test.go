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

package nodeproblemdetector_test

import (
	"context"
	"strconv"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/nodeproblemdetector"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("NodeProblemDetector", func() {
	var (
		ctx = context.TODO()

		managedResourceName = "shoot-core-node-problem-detector"
		namespace           = "some-namespace"
		image               = "some-image:some-tag"

		c                          client.Client
		component                  component.DeployWaiter
		daemonSetPrometheusPort    = 20257
		daemonSetPrometheusAddress = "0.0.0.0"
		managedResource            *resourcesv1alpha1.ManagedResource
		managedResourceSecret      *corev1.Secret
		values                     Values
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		values = Values{
			Image:      image,
			VPAEnabled: false,
		}
		component = New(c, namespace, values)

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		var (
			serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  labels:
    app.kubernetes.io/instance: shoot-core
    app.kubernetes.io/name: node-problem-detector
  name: node-problem-detector
  namespace: kube-system
`
			clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  labels:
    app.kubernetes.io/instance: shoot-core
    app.kubernetes.io/name: node-problem-detector
  name: node-problem-detector
rules:
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - get
- apiGroups:
  - ""
  resources:
  - nodes/status
  verbs:
  - patch
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
  - update
`
			clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  labels:
    app.kubernetes.io/instance: shoot-core
    app.kubernetes.io/name: node-problem-detector
  name: node-problem-detector
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: node-problem-detector
subjects:
- kind: ServiceAccount
  name: node-problem-detector
  namespace: kube-system
`
			clusterRolePSPYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  labels:
    app.kubernetes.io/instance: shoot-core
    app.kubernetes.io/name: node-problem-detector
  name: node-problem-detector-psp
rules:
- apiGroups:
  - extensions
  - policy
  resourceNames:
  - node-problem-detector
  resources:
  - podsecuritypolicies
  verbs:
  - use
`
			clusterRoleBindingPSPYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  labels:
    app.kubernetes.io/instance: shoot-core
    app.kubernetes.io/name: node-problem-detector
  name: node-problem-detector-psp
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: node-problem-detector-psp
subjects:
- kind: ServiceAccount
  name: node-problem-detector
  namespace: kube-system
`
			podSecurityPolicyYAML = `apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  creationTimestamp: null
  labels:
    app.kubernetes.io/instance: shoot-core
    app.kubernetes.io/name: node-problem-detector
  name: node-problem-detector
spec:
  allowPrivilegeEscalation: true
  allowedCapabilities:
  - '*'
  allowedHostPaths:
  - pathPrefix: /etc/localtime
  - pathPrefix: /var/log
  - pathPrefix: /dev/kmsg
  fsGroup:
    rule: RunAsAny
  privileged: true
  runAsUser:
    rule: RunAsAny
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  volumes:
  - configMap
  - emptyDir
  - projected
  - secret
  - downwardAPI
  - hostPath
`
			hostPathFileOrCreate = corev1.HostPathFileOrCreate

			daemonsetYAMLFor = func(apiserverHost string, vpaEnabled bool) string {
				out := `apiVersion: apps/v1
kind: DaemonSet
metadata:
  creationTimestamp: null
  labels:
    app.kubernetes.io/instance: shoot-core
    app.kubernetes.io/name: node-problem-detector
    gardener.cloud/role: system-component
    origin: gardener
  name: node-problem-detector
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: node-problem-detector
      app.kubernetes.io/instance: shoot-core
      app.kubernetes.io/name: node-problem-detector
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: node-problem-detector
        app.kubernetes.io/instance: shoot-core
        app.kubernetes.io/name: node-problem-detector
        gardener.cloud/role: system-component
        networking.gardener.cloud/to-apiserver: allowed
        networking.gardener.cloud/to-dns: allowed
        origin: gardener
    spec:
      containers:
      - command:
        - /bin/sh
        - -c
        - exec /node-problem-detector --logtostderr --config.system-log-monitor=/config/kernel-monitor.json,/config/docker-monitor.json,/config/systemd-monitor.json
          .. --config.custom-plugin-monitor=/config/kernel-monitor-counter.json,/config/systemd-monitor-counter.json
          .. --config.system-stats-monitor=/config/system-stats-monitor.json --prometheus-address=` + daemonSetPrometheusAddress + `
          --prometheus-port=` + strconv.Itoa(daemonSetPrometheusPort) + `
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName`
				if apiserverHost != "" {
					out += `
        - name: KUBERNETES_SERVICE_HOST
          value: ` + apiserverHost + ``
				}
				out += `
        image: ` + image + `
        imagePullPolicy: IfNotPresent
        name: node-problem-detector
        ports:
        - containerPort: 20257
          name: exporter
        resources:`
				if vpaEnabled {
					out += `
          limits:
            cpu: 80m
            memory: 80Mi`
				} else {
					out += `
          limits:
            cpu: 200m
            memory: 100Mi`
				}
				out += `
          requests:
            cpu: 20m
            memory: 20Mi
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /var/log
          name: log
        - mountPath: /etc/localtime
          name: localtime
          readOnly: true
        - mountPath: /dev/kmsg
          name: kmsg
          readOnly: true
      dnsPolicy: Default
      priorityClassName: system-cluster-critical
      serviceAccountName: node-problem-detector
      terminationGracePeriodSeconds: 30
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - key: CriticalAddonsOnly
        operator: Exists
      - effect: NoExecute
        operator: Exists
      volumes:
      - hostPath:
          path: /var/log/
        name: log
      - hostPath:
          path: /etc/localtime
          type: ` + string(hostPathFileOrCreate) + `
        name: localtime
      - hostPath:
          path: /dev/kmsg
        name: kmsg
  updateStrategy: {}
status:
  currentNumberScheduled: 0
  desiredNumberScheduled: 0
  numberMisscheduled: 0
  numberReady: 0
`
				return out
			}
			vpaYAML = `apiVersion: autoscaling.k8s.io/v1beta2
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: node-problem-detector
  namespace: kube-system
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      minAllowed:
        cpu: 10m
        memory: 20Mi
  targetRef:
    apiVersion: apps/v1
    kind: DaemonSet
    name: node-problem-detector
  updatePolicy:
    updateMode: Auto
status: {}
`
		)

		JustBeforeEach(func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))
			Expect(component.Deploy(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(managedResource).To(DeepEqual(&resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResource.Name,
					Namespace:       managedResource.Namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResourceSecret.Name,
					}},
					KeepObjects: pointer.BoolPtr(false),
				},
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__node-problem-detector.yaml"])).To(Equal(serviceAccountYAML))
			Expect(string(managedResourceSecret.Data["clusterrole____node-problem-detector.yaml"])).To(Equal(clusterRoleYAML))
			Expect(string(managedResourceSecret.Data["clusterrolebinding____node-problem-detector.yaml"])).To(Equal(clusterRoleBindingYAML))
			Expect(string(managedResourceSecret.Data["clusterrole____node-problem-detector-psp.yaml"])).To(Equal(clusterRolePSPYAML))
			Expect(string(managedResourceSecret.Data["clusterrolebinding____node-problem-detector-psp.yaml"])).To(Equal(clusterRoleBindingPSPYAML))
			Expect(string(managedResourceSecret.Data["podsecuritypolicy____node-problem-detector.yaml"])).To(Equal(podSecurityPolicyYAML))
		})

		Context("w/o apiserver host, w/o vpaEnables", func() {
			It("should successfully deploy all resources", func() {
				Expect(string(managedResourceSecret.Data["daemonset__kube-system__node-problem-detector.yaml"])).To(Equal(daemonsetYAMLFor("", false)))
			})
		})

		Context("w/ apiserver host,w/ vpaEnables", func() {
			var (
				apiserverHost = "apiserver.host"
				vpaEnabled    = true
			)

			BeforeEach(func() {
				values.APIServerHost = &apiserverHost
				values.VPAEnabled = vpaEnabled
				component = New(c, namespace, values)
			})

			It("should successfully deploy all resources", func() {
				Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__node-problem-detector.yaml"])).To(Equal(vpaYAML))
				Expect(string(managedResourceSecret.Data["daemonset__kube-system__node-problem-detector.yaml"])).To(Equal(daemonsetYAMLFor(apiserverHost, vpaEnabled)))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, managedResourceSecret.Name)))
		})
	})

	Context("waiting functions", func() {
		var (
			fakeOps   *retryfake.Ops
			resetVars func()
		)

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			resetVars = test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			)
		})

		AfterEach(func() {
			resetVars()
		})
		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource doesn't become healthy", func() {
				oldTimeout := TimeoutWaitForManagedResource
				defer func() { TimeoutWaitForManagedResource = oldTimeout }()
				TimeoutWaitForManagedResource = time.Millisecond

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionFalse,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				}))

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				}))

				Expect(component.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {

			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResource)).To(Succeed())

				Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})
			It("should not return an error when it's already removed", func() {
				Expect(component.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
