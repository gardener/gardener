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

package coredns_test

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/coredns"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("CoreDNS", func() {
	var (
		ctx = context.TODO()

		managedResourceName = "shoot-core-coredns"
		namespace           = "some-namespace"
		clusterDomain       = "foo.bar"
		clusterIP           = "1.2.3.4"
		image               = "some-image:some-tag"
		podNetworkCIDR      = "5.6.7.8/9"
		nodeNetworkCIDR     = "10.11.12.13/14"

		c         client.Client
		values    Values
		component component.DeployWaiter

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		values = Values{
			ClusterDomain:   clusterDomain,
			ClusterIP:       clusterIP,
			Image:           image,
			PodNetworkCIDR:  podNetworkCIDR,
			NodeNetworkCIDR: &nodeNetworkCIDR,
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
kind: ServiceAccount
metadata:
  creationTimestamp: null
  name: coredns
  namespace: kube-system
`
			clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: system:coredns
rules:
- apiGroups:
  - ""
  resources:
  - endpoints
  - services
  - pods
  - namespaces
  verbs:
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - get
- apiGroups:
  - discovery.k8s.io
  resources:
  - endpointslices
  verbs:
  - list
  - watch
`
			clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  name: system:coredns
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:coredns
subjects:
- kind: ServiceAccount
  name: coredns
  namespace: kube-system
`
			configMapYAML = `apiVersion: v1
data:
  Corefile: |
    .:8053 {
      errors
      log . {
          class error
      }
      health
      ready
      kubernetes ` + clusterDomain + ` in-addr.arpa ip6.arpa {
          pods insecure
          fallthrough in-addr.arpa ip6.arpa
          ttl 30
      }
      prometheus 0.0.0.0:9153
      forward . /etc/resolv.conf
      cache 30
      loop
      reload
      loadbalance round_robin
      import custom/*.override
    }
    import custom/*.server
kind: ConfigMap
metadata:
  creationTimestamp: null
  name: coredns
  namespace: kube-system
`
			configMapCustomYAML = `apiVersion: v1
data:
  changeme.override: '# checkout the docs on how to use: https://github.com/gardener/gardener/blob/master/docs/usage/custom-dns.md'
  changeme.server: '# checkout the docs on how to use: https://github.com/gardener/gardener/blob/master/docs/usage/custom-dns.md'
kind: ConfigMap
metadata:
  annotations:
    resources.gardener.cloud/ignore: "true"
  creationTimestamp: null
  name: coredns-custom
  namespace: kube-system
`
			serviceYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    k8s-app: kube-dns
    kubernetes.io/cluster-service: "true"
    kubernetes.io/name: CoreDNS
  name: kube-dns
  namespace: kube-system
spec:
  clusterIP: ` + clusterIP + `
  ports:
  - name: dns
    port: 53
    protocol: UDP
    targetPort: 8053
  - name: dns-tcp
    port: 53
    protocol: TCP
    targetPort: 8053
  - name: metrics
    port: 9153
    targetPort: 9153
  selector:
    k8s-app: kube-dns
status:
  loadBalancer: {}
`
			networkPolicyYAML = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  annotations:
    gardener.cloud/description: Allows CoreDNS to lookup DNS records, talk to the
      API Server. Also allows CoreDNS to be reachable via its service and its metrics
      endpoint.
  creationTimestamp: null
  name: gardener.cloud--allow-dns
  namespace: kube-system
spec:
  egress:
  - ports:
    - port: 443
      protocol: TCP
    - port: 53
      protocol: TCP
    - port: 53
      protocol: UDP
  ingress:
  - from:
    - namespaceSelector: {}
      podSelector: {}
    - ipBlock:
        cidr: ` + podNetworkCIDR + `
    - ipBlock:
        cidr: ` + nodeNetworkCIDR + `
    ports:
    - port: 9153
      protocol: TCP
    - port: 8053
      protocol: TCP
    - port: 8053
      protocol: UDP
  podSelector:
    matchExpressions:
    - key: k8s-app
      operator: In
      values:
      - kube-dns
  policyTypes:
  - Ingress
  - Egress
`
			deploymentYAMLFor = func(apiserverHost string, podAnnotations map[string]string) string {
				out := `apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: system-component
    k8s-app: kube-dns
    origin: gardener
  name: coredns
  namespace: kube-system
spec:
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      k8s-app: kube-dns
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
    type: RollingUpdate
  template:
    metadata:
      annotations:
`

				for k, v := range podAnnotations {
					out += `        ` + k + `: ` + v + `
`
				}

				out += `        scheduler.alpha.kubernetes.io/critical-pod: ""
      creationTimestamp: null
      labels:
        gardener.cloud/role: system-component
        k8s-app: kube-dns
        origin: gardener
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: k8s-app
                  operator: In
                  values:
                  - kube-dns
              topologyKey: kubernetes.io/hostname
            weight: 100
      containers:
      - args:
        - -conf
        - /etc/coredns/Corefile
`

				if apiserverHost != "" {
					out += `        env:
        - name: KUBERNETES_SERVICE_HOST
          value: ` + apiserverHost + `
`
				}

				out += `        image: ` + image + `
        imagePullPolicy: IfNotPresent
        livenessProbe:
          failureThreshold: 5
          httpGet:
            path: /health
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 60
          successThreshold: 1
          timeoutSeconds: 5
        name: coredns
        ports:
        - containerPort: 8053
          name: dns-udp
          protocol: UDP
        - containerPort: 8053
          name: dns-tcp
          protocol: TCP
        - containerPort: 9153
          name: metrics
          protocol: TCP
        readinessProbe:
          failureThreshold: 1
          httpGet:
            path: /ready
            port: 8181
            scheme: HTTP
          initialDelaySeconds: 30
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 2
        resources:
          limits:
            cpu: 250m
            memory: 500Mi
          requests:
            cpu: 50m
            memory: 15Mi
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - all
          readOnlyRootFilesystem: true
        volumeMounts:
        - mountPath: /etc/coredns
          name: config-volume
          readOnly: true
        - mountPath: /etc/coredns/custom
          name: custom-config-volume
          readOnly: true
      dnsPolicy: Default
      nodeSelector:
        worker.gardener.cloud/system-components: "true"
      priorityClassName: system-cluster-critical
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
      serviceAccountName: coredns
      tolerations:
      - key: CriticalAddonsOnly
        operator: Exists
      volumes:
      - configMap:
          items:
          - key: Corefile
            path: Corefile
          name: coredns
        name: config-volume
      - configMap:
          defaultMode: 420
          name: coredns-custom
          optional: true
        name: custom-config-volume
status: {}
`
				return out
			}
			pdbYAML = `apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  creationTimestamp: null
  labels:
    k8s-app: kube-dns
  name: coredns
  namespace: kube-system
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      k8s-app: kube-dns
status:
  currentHealthy: 0
  desiredHealthy: 0
  disruptionsAllowed: 0
  expectedPods: 0
`
			hpaYAML = `apiVersion: autoscaling/v2beta1
kind: HorizontalPodAutoscaler
metadata:
  creationTimestamp: null
  name: coredns
  namespace: kube-system
spec:
  maxReplicas: 5
  metrics:
  - resource:
      name: cpu
      targetAverageUtilization: 70
    type: Resource
  minReplicas: 2
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: coredns
status:
  conditions: null
  currentMetrics: null
  currentReplicas: 0
  desiredReplicas: 0
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
			Expect(managedResourceSecret.Data).To(HaveLen(10))
			Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__coredns.yaml"])).To(Equal(serviceAccountYAML))
			Expect(string(managedResourceSecret.Data["clusterrole____system_coredns.yaml"])).To(Equal(clusterRoleYAML))
			Expect(string(managedResourceSecret.Data["clusterrolebinding____system_coredns.yaml"])).To(Equal(clusterRoleBindingYAML))
			Expect(string(managedResourceSecret.Data["configmap__kube-system__coredns.yaml"])).To(Equal(configMapYAML))
			Expect(string(managedResourceSecret.Data["configmap__kube-system__coredns-custom.yaml"])).To(Equal(configMapCustomYAML))
			Expect(string(managedResourceSecret.Data["service__kube-system__kube-dns.yaml"])).To(Equal(serviceYAML))
			Expect(string(managedResourceSecret.Data["networkpolicy__kube-system__gardener.cloud--allow-dns.yaml"])).To(Equal(networkPolicyYAML))
			Expect(string(managedResourceSecret.Data["poddisruptionbudget__kube-system__coredns.yaml"])).To(Equal(pdbYAML))
			Expect(string(managedResourceSecret.Data["horizontalpodautoscaler__kube-system__coredns.yaml"])).To(Equal(hpaYAML))
		})

		Context("w/o apiserver host, w/o pod annotations", func() {
			It("should successfully deploy all resources", func() {
				Expect(string(managedResourceSecret.Data["deployment__kube-system__coredns.yaml"])).To(Equal(deploymentYAMLFor("", nil)))
			})
		})

		Context("w/ apiserver host, w/ pod annotations", func() {
			var (
				apiserverHost  = "apiserver.host"
				podAnnotations = map[string]string{"foo": "bar"}
			)

			BeforeEach(func() {
				values.APIServerHost = &apiserverHost
				values.PodAnnotations = podAnnotations
				component = New(c, namespace, values)
			})

			It("should successfully deploy all resources", func() {
				Expect(string(managedResourceSecret.Data["deployment__kube-system__coredns.yaml"])).To(Equal(deploymentYAMLFor(apiserverHost, podAnnotations)))
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
