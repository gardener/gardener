// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nodeexporter_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/nodeexporter"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Nodeexporter", func() {
	var (
		ctx = context.TODO()

		managedResourceName = "shoot-core-node-exporter"
		namespace           = "some-namespace"
		image               = "some-image:some-tag"

		c         client.Client
		values    Values
		component component.DeployWaiter

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		values = Values{
			Image:       image,
			PSPDisabled: true,
		}

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
    component: node-exporter
  name: node-exporter
  namespace: kube-system
`
			serviceYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    component: node-exporter
  name: node-exporter
  namespace: kube-system
spec:
  clusterIP: None
  ports:
  - name: metrics
    port: 16909
    protocol: TCP
    targetPort: 0
  selector:
    component: node-exporter
  type: ClusterIP
status:
  loadBalancer: {}
`
			daemonSetYAML = `apiVersion: apps/v1
kind: DaemonSet
metadata:
  creationTimestamp: null
  labels:
    component: node-exporter
    gardener.cloud/role: monitoring
    origin: gardener
  name: node-exporter
  namespace: kube-system
spec:
  selector:
    matchLabels:
      component: node-exporter
  template:
    metadata:
      creationTimestamp: null
      labels:
        component: node-exporter
        gardener.cloud/role: monitoring
        networking.gardener.cloud/from-seed: allowed
        networking.gardener.cloud/to-public-networks: allowed
        origin: gardener
    spec:
      automountServiceAccountToken: false
      containers:
      - command:
        - /bin/node_exporter
        - --web.listen-address=:16909
        - --path.procfs=/host/proc
        - --path.sysfs=/host/sys
        - --path.rootfs=/host
        - --log.level=error
        - --collector.disable-defaults
        - --collector.conntrack
        - --collector.cpu
        - --collector.diskstats
        - --collector.filefd
        - --collector.filesystem
        - --collector.filesystem.mount-points-exclude=^/(run|var)/.+$|^/(boot|dev|sys|usr)($|/.+$)
        - --collector.loadavg
        - --collector.meminfo
        - --collector.uname
        - --collector.stat
        - --collector.pressure
        image: some-image:some-tag
        imagePullPolicy: IfNotPresent
        livenessProbe:
          httpGet:
            path: /
            port: 16909
          initialDelaySeconds: 5
          timeoutSeconds: 5
        name: node-exporter
        ports:
        - containerPort: 16909
          hostPort: 16909
          name: scrape
          protocol: TCP
        readinessProbe:
          httpGet:
            path: /
            port: 16909
          initialDelaySeconds: 5
          timeoutSeconds: 5
        resources:
          limits:
            memory: 250Mi
          requests:
            cpu: 50m
            memory: 50Mi
        volumeMounts:
        - mountPath: /host
          name: host
          readOnly: true
      hostNetwork: true
      hostPID: true
      priorityClassName: system-cluster-critical
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        seccompProfile:
          type: RuntimeDefault
      serviceAccountName: node-exporter
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - effect: NoExecute
        operator: Exists
      volumes:
      - hostPath:
          path: /
        name: host
  updateStrategy:
    type: RollingUpdate
status:
  currentNumberScheduled: 0
  desiredNumberScheduled: 0
  numberMisscheduled: 0
  numberReady: 0
`
			podSecurityPolicyYAML = `apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  annotations:
    seccomp.security.alpha.kubernetes.io/allowedProfileNames: runtime/default
    seccomp.security.alpha.kubernetes.io/defaultProfileName: runtime/default
  creationTimestamp: null
  name: gardener.kube-system.node-exporter
spec:
  allowedHostPaths:
  - pathPrefix: /
  - pathPrefix: /sys
  - pathPrefix: /proc
  fsGroup:
    rule: RunAsAny
  hostNetwork: true
  hostPID: true
  hostPorts:
  - max: 16909
    min: 16909
  runAsUser:
    rule: MustRunAsNonRoot
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  volumes:
  - hostPath
`
			clusterRolePSP = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: gardener.cloud:psp:kube-system:node-exporter
rules:
- apiGroups:
  - policy
  - extensions
  resourceNames:
  - gardener.kube-system.node-exporter
  resources:
  - podsecuritypolicies
  verbs:
  - use
`
			roleBindingPSP = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  name: gardener.cloud:psp:node-exporter
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener.cloud:psp:kube-system:node-exporter
subjects:
- kind: ServiceAccount
  name: node-exporter
  namespace: kube-system
`
			vpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: node-exporter
  namespace: kube-system
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      controlledValues: RequestsOnly
      minAllowed:
        memory: 50Mi
  targetRef:
    apiVersion: apps/v1
    kind: DaemonSet
    name: node-exporter
  updatePolicy:
    updateMode: Auto
status: {}
`
		)

		JustBeforeEach(func() {
			component = New(c, namespace, values)
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
					KeepObjects: pointer.Bool(false),
				},
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
		})

		It("should successfully deploy all resources", func() {
			Expect(managedResourceSecret.Data).To(HaveLen(3))
			Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__node-exporter.yaml"])).To(Equal(serviceAccountYAML))
			Expect(string(managedResourceSecret.Data["service__kube-system__node-exporter.yaml"])).To(Equal(serviceYAML))
			Expect(string(managedResourceSecret.Data["daemonset__kube-system__node-exporter.yaml"])).To(Equal(daemonSetYAML))
		})

		Context("PSP is not disabled", func() {
			BeforeEach(func() {
				values.PSPDisabled = false
			})

			It("should successfully PSP related resources when psp is not disabled", func() {
				Expect(managedResourceSecret.Data).To(HaveLen(6))
				Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__node-exporter.yaml"])).To(Equal(serviceAccountYAML))
				Expect(string(managedResourceSecret.Data["service__kube-system__node-exporter.yaml"])).To(Equal(serviceYAML))
				Expect(string(managedResourceSecret.Data["daemonset__kube-system__node-exporter.yaml"])).To(Equal(daemonSetYAML))
				Expect(string(managedResourceSecret.Data["podsecuritypolicy____gardener.kube-system.node-exporter.yaml"])).To(Equal(podSecurityPolicyYAML))
				Expect(string(managedResourceSecret.Data["clusterrole____gardener.cloud_psp_kube-system_node-exporter.yaml"])).To(Equal(clusterRolePSP))
				Expect(string(managedResourceSecret.Data["rolebinding__kube-system__gardener.cloud_psp_node-exporter.yaml"])).To(Equal(roleBindingPSP))
			})
		})

		Context("PSP is not disabled, VPA enabled", func() {
			BeforeEach(func() {
				values.PSPDisabled = false
				values.VPAEnabled = true
			})

			It("should successfully PSP related resources when psp is not disabled", func() {
				Expect(managedResourceSecret.Data).To(HaveLen(7))
				Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__node-exporter.yaml"])).To(Equal(serviceAccountYAML))
				Expect(string(managedResourceSecret.Data["service__kube-system__node-exporter.yaml"])).To(Equal(serviceYAML))
				Expect(string(managedResourceSecret.Data["daemonset__kube-system__node-exporter.yaml"])).To(Equal(daemonSetYAML))
				Expect(string(managedResourceSecret.Data["podsecuritypolicy____gardener.kube-system.node-exporter.yaml"])).To(Equal(podSecurityPolicyYAML))
				Expect(string(managedResourceSecret.Data["clusterrole____gardener.cloud_psp_kube-system_node-exporter.yaml"])).To(Equal(clusterRolePSP))
				Expect(string(managedResourceSecret.Data["rolebinding__kube-system__gardener.cloud_psp_node-exporter.yaml"])).To(Equal(roleBindingPSP))
				Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__node-exporter.yaml"])).To(Equal(vpaYAML))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			component = New(c, namespace, values)
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
			component = New(c, namespace, values)

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
								Status: gardencorev1beta1.ConditionFalse,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

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
				})).To(Succeed())

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
