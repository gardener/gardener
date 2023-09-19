// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"regexp"

	"github.com/Masterminds/semver"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/coredns"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CoreDNS", func() {
	var (
		ctx = context.TODO()

		managedResourceName = "shoot-core-coredns"
		namespace           = "some-namespace"
		clusterDomain       = "foo.bar"
		clusterIP           = "1.2.3.4"
		image               = "some-image:some-tag"
		cpaImage            = "cpa-image:cpa-tag"
		podNetworkCIDR      = "5.6.7.8/9"
		nodeNetworkCIDR     = "10.11.12.13/14"

		c         client.Client
		values    Values
		component component.DeployWaiter

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
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
		configMapYAML = func(rewritingEnabled bool, commonSuffixes []string) string {
			out := `apiVersion: v1
data:
  Corefile: |
    .:8053 {
      errors
      log . {
          class error
      }
      health {
          lameduck 15s
      }
      ready`
			if rewritingEnabled {
				out += `
      rewrite stop {
        name regex ([^\.]+)\.([^\.]+)\.svc\.foo\.bar\.svc\.foo\.bar {1}.{2}.svc.foo.bar
        answer name ([^\.]+)\.([^\.]+)\.svc\.foo\.bar {1}.{2}.svc.foo.bar.svc.foo.bar
        answer value ([^\.]+)\.([^\.]+)\.svc\.foo\.bar {1}.{2}.svc.foo.bar.svc.foo.bar
      }`
				for _, suffix := range commonSuffixes {
					out += `
      rewrite stop {
        name regex (.*)\.` + regexp.QuoteMeta(suffix) + `\.svc\.foo\.bar {1}.` + suffix + `
        answer name (.*)\.` + regexp.QuoteMeta(suffix) + ` {1}.` + suffix + `.svc.foo.bar
        answer value (.*)\.` + regexp.QuoteMeta(suffix) + ` {1}.` + suffix + `.svc.foo.bar
      }`
				}
			}
			out += `
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
			return out
		}
		configMapCustomYAML = `apiVersion: v1
data:
  changeme.override: '# checkout the docs on how to use: https://github.com/gardener/gardener/blob/master/docs/usage/custom-dns-config.md'
  changeme.server: '# checkout the docs on how to use: https://github.com/gardener/gardener/blob/master/docs/usage/custom-dns-config.md'
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
		deploymentYAMLFor = func(apiserverHost string, podAnnotations map[string]string, keepReplicas bool, useHALabel bool) string {
			out := `apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: system-component
`
			if useHALabel {
				out += `    high-availability-config.resources.gardener.cloud/type: server
`
			}

			out += `    k8s-app: kube-dns
    origin: gardener
  name: coredns
  namespace: kube-system
spec:
`
			if keepReplicas {
				out += `  replicas: 2
`
			}

			out += `  revisionHistoryLimit: 2
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
`
			if len(podAnnotations) > 0 {
				out += `      annotations:
`
			}

			for k, v := range podAnnotations {
				out += `        ` + k + `: ` + v + `
`
			}

			out += `      creationTimestamp: null
      labels:
        gardener.cloud/role: system-component
        k8s-app: kube-dns
        origin: gardener
    spec:
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
            memory: 1500Mi
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
      priorityClassName: system-cluster-critical
      securityContext:
        fsGroup: 1
        runAsNonRoot: true
        runAsUser: 65534
        seccompProfile:
          type: RuntimeDefault
        supplementalGroups:
        - 1
      serviceAccountName: coredns
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
		pdbYAML = `apiVersion: policy/v1
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
		hpaYAMLFor = func(k8sGreaterEqual123 bool) string {
			apiVersion := "autoscaling/v2beta1"
			target := `
      targetAverageUtilization: 70`
			status := `
  conditions: null
  currentMetrics: null
  currentReplicas: 0
  desiredReplicas: 0
`
			if k8sGreaterEqual123 {
				apiVersion = "autoscaling/v2"
				target = `
      target:
        averageUtilization: 70
        type: Utilization`
				status = `
  currentMetrics: null
  desiredReplicas: 0
`
			}
			out := `apiVersion: ` + apiVersion + `
kind: HorizontalPodAutoscaler
metadata:
  creationTimestamp: null
  labels:
    high-availability-config.resources.gardener.cloud/type: server
  name: coredns
  namespace: kube-system
spec:
  maxReplicas: 5
  metrics:
  - resource:
      name: cpu` + target + `
    type: Resource
  minReplicas: 2
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: coredns
status:` + status
			return out
		}
		cpasaYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  name: coredns-autoscaler
  namespace: kube-system
`
		cpacrYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: system:coredns-autoscaler
rules:
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - replicationcontrollers/scale
  verbs:
  - get
  - update
- apiGroups:
  - apps
  resources:
  - deployments/scale
  - replicasets/scale
  verbs:
  - get
  - update
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - get
  - create
`
		cpacrbYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: system:coredns-autoscaler
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:coredns-autoscaler
subjects:
- kind: ServiceAccount
  name: coredns-autoscaler
  namespace: kube-system
`
		cpaDeploymentYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: system-component
    k8s-app: coredns-autoscaler
    kubernetes.io/cluster-service: "true"
    origin: gardener
  name: coredns-autoscaler
  namespace: kube-system
spec:
  selector:
    matchLabels:
      k8s-app: coredns-autoscaler
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        gardener.cloud/role: system-component
        k8s-app: coredns-autoscaler
        kubernetes.io/cluster-service: "true"
        origin: gardener
    spec:
      containers:
      - command:
        - /cluster-proportional-autoscaler
        - --namespace=kube-system
        - --configmap=coredns-autoscaler
        - --target=deployment/coredns
        - --default-params={"linear":{"coresPerReplica":256,"nodesPerReplica":16,"min":2,"preventSinglePointFailure":true,"includeUnschedulableNodes":true}}
        - --logtostderr=true
        - --v=2
        image: ` + cpaImage + `
        imagePullPolicy: IfNotPresent
        name: autoscaler
        resources:
          limits:
            memory: 70Mi
          requests:
            cpu: 20m
            memory: 10Mi
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - all
          readOnlyRootFilesystem: true
      priorityClassName: system-cluster-critical
      securityContext:
        fsGroup: 65534
        runAsNonRoot: true
        runAsUser: 65534
        seccompProfile:
          type: RuntimeDefault
        supplementalGroups:
        - 65534
      serviceAccountName: coredns-autoscaler
status: {}
`

		cpaDeploymentVpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: coredns-autoscaler
  namespace: kube-system
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      controlledValues: RequestsOnly
      minAllowed:
        memory: 10Mi
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: coredns-autoscaler
  updatePolicy:
    updateMode: Auto
status: {}
`
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		values = Values{
			ClusterDomain:     clusterDomain,
			ClusterIP:         clusterIP,
			Image:             image,
			PodNetworkCIDR:    podNetworkCIDR,
			NodeNetworkCIDR:   &nodeNetworkCIDR,
			KubernetesVersion: semver.MustParse("1.24.0"),
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
			cpaEnabled       bool
			vpaEnabled       bool
			rewritingEnabled bool
			commonSuffixes   []string
		)

		BeforeEach(func() {
			cpaEnabled = false
			vpaEnabled = false
			rewritingEnabled = false
			commonSuffixes = []string{}
		})

		JustBeforeEach(func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
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
						Name: managedResource.Spec.SecretRefs[0].Name,
					}},
					KeepObjects: pointer.Bool(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))

			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(pointer.Bool(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
			if cpaEnabled {
				if vpaEnabled {
					Expect(managedResourceSecret.Data).To(HaveLen(14))
				} else {
					Expect(managedResourceSecret.Data).To(HaveLen(13))
				}
			} else {
				Expect(managedResourceSecret.Data).To(HaveLen(10))
			}
			Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__coredns.yaml"])).To(Equal(serviceAccountYAML))
			Expect(string(managedResourceSecret.Data["clusterrole____system_coredns.yaml"])).To(Equal(clusterRoleYAML))
			Expect(string(managedResourceSecret.Data["clusterrolebinding____system_coredns.yaml"])).To(Equal(clusterRoleBindingYAML))
			Expect(string(managedResourceSecret.Data["configmap__kube-system__coredns.yaml"])).To(Equal(configMapYAML(rewritingEnabled, commonSuffixes)))
			Expect(string(managedResourceSecret.Data["configmap__kube-system__coredns-custom.yaml"])).To(Equal(configMapCustomYAML))
			Expect(string(managedResourceSecret.Data["service__kube-system__kube-dns.yaml"])).To(Equal(serviceYAML))
			Expect(string(managedResourceSecret.Data["networkpolicy__kube-system__gardener.cloud--allow-dns.yaml"])).To(Equal(networkPolicyYAML))
			Expect(string(managedResourceSecret.Data["poddisruptionbudget__kube-system__coredns.yaml"])).To(Equal(pdbYAML))
			if cpaEnabled {
				Expect(string(managedResourceSecret.Data["horizontalpodautoscaler__kube-system__coredns.yaml"])).To(Equal(""))
				Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__coredns-autoscaler.yaml"])).To(Equal(cpasaYAML))
				Expect(string(managedResourceSecret.Data["clusterrole____system_coredns-autoscaler.yaml"])).To(Equal(cpacrYAML))
				Expect(string(managedResourceSecret.Data["clusterrolebinding____system_coredns-autoscaler.yaml"])).To(Equal(cpacrbYAML))
				Expect(string(managedResourceSecret.Data["deployment__kube-system__coredns-autoscaler.yaml"])).To(Equal(cpaDeploymentYAML))
				if vpaEnabled {
					Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__coredns-autoscaler.yaml"])).To(Equal(cpaDeploymentVpaYAML))
				} else {
					Expect(string(managedResourceSecret.Data["verticalpodautoscaler__kube-system__coredns-autoscaler.yaml"])).To(Equal(""))
				}
			} else {
				Expect(string(managedResourceSecret.Data["serviceaccount__kube-system__coredns-autoscaler.yaml"])).To(Equal(""))
				Expect(string(managedResourceSecret.Data["clusterrole____system_coredns-autoscaler.yaml"])).To(Equal(""))
				Expect(string(managedResourceSecret.Data["clusterrolebinding____system_coredns-autoscaler.yaml"])).To(Equal(""))
				Expect(string(managedResourceSecret.Data["deployment__kube-system__coredns-autoscaler.yaml"])).To(Equal(""))
			}
		})

		Context("kubernetes version > 1.23.0", func() {
			JustBeforeEach(func() {
				if !cpaEnabled {
					Expect(string(managedResourceSecret.Data["horizontalpodautoscaler__kube-system__coredns.yaml"])).To(Equal(hpaYAMLFor(true)))
				}
			})
			Context("w/o apiserver host, w/o pod annotations", func() {
				It("should successfully deploy all resources", func() {
					Expect(string(managedResourceSecret.Data["deployment__kube-system__coredns.yaml"])).To(Equal(deploymentYAMLFor("", nil, true, true)))
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
					Expect(string(managedResourceSecret.Data["deployment__kube-system__coredns.yaml"])).To(Equal(deploymentYAMLFor(apiserverHost, podAnnotations, true, true)))
				})
			})

			Context("w/ rewriting enabled", func() {
				BeforeEach(func() {
					rewritingEnabled = true
					commonSuffixes = []string{"gardener.cloud", "github.com"}
					values.SearchPathRewritesEnabled = true
					values.SearchPathRewriteCommonSuffixes = commonSuffixes
					component = New(c, namespace, values)
				})

				It("should successfully deploy all resources", func() {
					Expect(string(managedResourceSecret.Data["deployment__kube-system__coredns.yaml"])).To(Equal(deploymentYAMLFor("", nil, true, true)))
				})

				AfterEach(func() {
					rewritingEnabled = false
					commonSuffixes = []string{}
					values.SearchPathRewritesEnabled = false
					values.SearchPathRewriteCommonSuffixes = commonSuffixes
				})
			})

			Context("w/ cluster proportional autoscaler enabled", func() {
				BeforeEach(func() {
					cpaEnabled = true
					values.AutoscalingMode = gardencorev1beta1.CoreDNSAutoscalingModeClusterProportional
					values.ClusterProportionalAutoscalerImage = cpaImage
					component = New(c, namespace, values)
				})

				Context("w/o apiserver host, w/o pod annotations", func() {
					It("should successfully deploy all resources", func() {
						Expect(string(managedResourceSecret.Data["deployment__kube-system__coredns.yaml"])).To(Equal(deploymentYAMLFor("", nil, false, false)))
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
						Expect(string(managedResourceSecret.Data["deployment__kube-system__coredns.yaml"])).To(Equal(deploymentYAMLFor(apiserverHost, podAnnotations, false, false)))
					})
				})

				Context("w/ vpa enabled", func() {
					BeforeEach(func() {
						vpaEnabled = true
						values.WantsVerticalPodAutoscaler = true
						component = New(c, namespace, values)
					})

					Context("w/o apiserver host, w/o pod annotations", func() {
						It("should successfully deploy all resources", func() {
							Expect(string(managedResourceSecret.Data["deployment__kube-system__coredns.yaml"])).To(Equal(deploymentYAMLFor("", nil, false, false)))
						})
					})
				})
			})
		})

		Context("kubernetes version < 1.23.0, w/ apiserver host, w/ pod annotations", func() {
			var (
				apiserverHost  = "apiserver.host"
				podAnnotations = map[string]string{"foo": "bar"}
			)

			BeforeEach(func() {
				values.KubernetesVersion, _ = semver.NewVersion("v1.22.0")
				values.APIServerHost = &apiserverHost
				values.PodAnnotations = podAnnotations
				component = New(c, namespace, values)
			})

			It("should successfully deploy all resources", func() {
				Expect(string(managedResourceSecret.Data["horizontalpodautoscaler__kube-system__coredns.yaml"])).To(Equal(hpaYAMLFor(false)))
				Expect(string(managedResourceSecret.Data["deployment__kube-system__coredns.yaml"])).To(Equal(deploymentYAMLFor(apiserverHost, podAnnotations, true, true)))
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
