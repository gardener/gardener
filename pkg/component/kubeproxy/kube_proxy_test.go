// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeproxy_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/kubeproxy"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/pkg/utils/version"
)

var _ = Describe("KubeProxy", func() {
	var (
		ctx = context.TODO()

		namespace      = "some-namespace"
		kubeconfig     = []byte("some-kubeconfig")
		podNetworkCIDR = "4.5.6.7/8"
		imageAlpine    = "some-alpine:image"

		c         client.Client
		component Interface
		values    Values

		managedResourceCentral       *resourcesv1alpha1.ManagedResource
		managedResourceSecretCentral *corev1.Secret

		managedResourceForPool = func(pool WorkerPool) *resourcesv1alpha1.ManagedResource {
			return &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-core-kube-proxy-" + pool.Name + "-v" + pool.KubernetesVersion.String(),
					Namespace: namespace,
					Labels: map[string]string{
						"component":          "kube-proxy",
						"role":               "pool",
						"pool-name":          pool.Name,
						"kubernetes-version": pool.KubernetesVersion.String(),
					},
				},
			}
		}
		managedResourceSecretForPool = func(pool WorkerPool) *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedresource-" + managedResourceForPool(pool).Name,
					Namespace: namespace,
					Labels: map[string]string{
						"component":          "kube-proxy",
						"role":               "pool",
						"pool-name":          pool.Name,
						"kubernetes-version": pool.KubernetesVersion.String(),
					},
				},
			}
		}

		managedResourceForPoolForMajorMinorVersionOnly = func(pool WorkerPool) *resourcesv1alpha1.ManagedResource {
			return &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-core-kube-proxy-" + pool.Name + "-v" + fmt.Sprintf("%d.%d", pool.KubernetesVersion.Major(), pool.KubernetesVersion.Minor()),
					Namespace: namespace,
					Labels: map[string]string{
						"component":          "kube-proxy",
						"role":               "pool",
						"pool-name":          pool.Name,
						"kubernetes-version": fmt.Sprintf("%d.%d", pool.KubernetesVersion.Major(), pool.KubernetesVersion.Minor()),
					},
				},
			}
		}
		managedResourceSecretForPoolForMajorMinorVersionOnly = func(pool WorkerPool) *corev1.Secret {
			return &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedresource-" + managedResourceForPoolForMajorMinorVersionOnly(pool).Name,
					Namespace: namespace,
					Labels: map[string]string{
						"component":          "kube-proxy",
						"role":               "pool",
						"pool-name":          pool.Name,
						"kubernetes-version": fmt.Sprintf("%d.%d", pool.KubernetesVersion.Major(), pool.KubernetesVersion.Minor()),
					},
				},
			}
		}
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		values = Values{
			IPVSEnabled: true,
			FeatureGates: map[string]bool{
				"Foo": true,
				"Bar": false,
			},
			ImageAlpine: imageAlpine,
			Kubeconfig:  kubeconfig,
			VPAEnabled:  false,
			WorkerPools: []WorkerPool{
				{Name: "pool1", KubernetesVersion: semver.MustParse("1.24.13"), Image: "some-image:some-tag1"},
				{Name: "pool2", KubernetesVersion: semver.MustParse("1.25.4"), Image: "some-image:some-tag2"},
			},
		}
		component = New(c, namespace, values)

		managedResourceCentral = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot-core-kube-proxy",
				Namespace: namespace,
				Labels:    map[string]string{"component": "kube-proxy"},
			},
		}
		managedResourceSecretCentral = &corev1.Secret{}
	})

	Describe("#Deploy", func() {
		var (
			serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  name: kube-proxy
  namespace: kube-system
`

			clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: gardener.cloud:target:node-proxier
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:node-proxier
subjects:
- kind: ServiceAccount
  name: kube-proxy
  namespace: kube-system
`

			serviceYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    app: kubernetes
    role: proxy
  name: kube-proxy
  namespace: kube-system
spec:
  clusterIP: None
  ports:
  - name: metrics
    port: 10249
    protocol: TCP
    targetPort: 0
  selector:
    app: kubernetes
    role: proxy
  type: ClusterIP
status:
  loadBalancer: {}
`

			secretName = "kube-proxy-e3a80e6d"
			secretYAML = `apiVersion: v1
data:
  kubeconfig: ` + utils.EncodeBase64(kubeconfig) + `
immutable: true
kind: Secret
metadata:
  creationTimestamp: null
  labels:
    resources.gardener.cloud/garbage-collectable-reference: "true"
  name: ` + secretName + `
  namespace: kube-system
type: Opaque
`

			configMapNameFor = func(ipvsEnabled bool) string {
				if !ipvsEnabled {
					return "kube-proxy-config-b2209acf"
				}
				return "kube-proxy-config-ff9faa85"
			}
			configMapYAMLFor = func(ipvsEnabled bool) string {
				out := `apiVersion: v1
data:
  config.yaml: |
    apiVersion: kubeproxy.config.k8s.io/v1alpha1
    bindAddress: ""
    bindAddressHardFail: false
    clientConnection:
      acceptContentTypes: ""
      burst: 0
      contentType: ""
      kubeconfig: /var/lib/kube-proxy-kubeconfig/kubeconfig
      qps: 0`
				if ipvsEnabled {
					out += `
    clusterCIDR: ""`
				} else {
					out += `
    clusterCIDR: ` + podNetworkCIDR
				}
				out += `
    configSyncPeriod: 0s
    conntrack:
      maxPerCore: 524288
      min: null
      tcpCloseWaitTimeout: null
      tcpEstablishedTimeout: null
    detectLocal:
      bridgeInterface: ""
      interfaceNamePrefix: ""
    detectLocalMode: ""
    enableProfiling: false
    featureGates:
      Bar: false
      Foo: true
    healthzBindAddress: ""
    hostnameOverride: ""
    iptables:
      localhostNodePorts: null
      masqueradeAll: false
      masqueradeBit: null
      minSyncPeriod: 0s
      syncPeriod: 0s
    ipvs:
      excludeCIDRs: null
      minSyncPeriod: 0s
      scheduler: ""
      strictARP: false
      syncPeriod: 0s
      tcpFinTimeout: 0s
      tcpTimeout: 0s
      udpTimeout: 0s
    kind: KubeProxyConfiguration
    logging:
      flushFrequency: 0
      options:
        json:
          infoBufferSize: "0"
      verbosity: 0
    metricsBindAddress: 0.0.0.0:10249`
				if ipvsEnabled {
					out += `
    mode: ipvs`
				} else {
					out += `
    mode: iptables`
				}
				out += `
    nodePortAddresses: null
    oomScoreAdj: null
    portRange: ""
    showHiddenMetricsForVersion: ""
    winkernel:
      enableDSR: false
      forwardHealthCheckVip: false
      networkName: ""
      rootHnsEndpointName: ""
      sourceVip: ""
immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    resources.gardener.cloud/garbage-collectable-reference: "true"
  name: ` + configMapNameFor(ipvsEnabled) + `
  namespace: kube-system
`
				return out
			}

			configMapConntrackFixScriptName = "kube-proxy-conntrack-fix-script-40092541"
			configMapConntrackFixScriptYAML = `apiVersion: v1
data:
  conntrack_fix.sh: |
    #!/bin/sh -e
    trap "kill -s INT 1" TERM
    apk add conntrack-tools
    sleep 120 & wait
    date
    # conntrack example:
    # tcp      6 113 SYN_SENT src=21.73.193.93 dst=21.71.0.65 sport=1413 dport=443 \
    #   [UNREPLIED] src=21.71.0.65 dst=21.73.193.93 sport=443 dport=1413 mark=0 use=1
    eval "$(
      conntrack -L -p tcp --state SYN_SENT \
      | sed 's/=/ /g'                      \
      | awk '$6 !~ /^10\./ &&
             $8 !~ /^10\./ &&
             $6  == $17    &&
             $8  == $15    &&
             $10 == $21    &&
             $12 == $19 {
               printf "conntrack -D -p tcp -s %s --sport %s -d %s --dport %s;\n",
                                              $6,        $10,  $8,        $12}'
    )"
    while true; do
      date
      sleep 3600 & wait
    done
immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    app: kubernetes
    gardener.cloud/role: system-component
    origin: gardener
    resources.gardener.cloud/garbage-collectable-reference: "true"
    role: proxy
  name: ` + configMapConntrackFixScriptName + `
  namespace: kube-system
`

			configMapCleanupScriptName = "kube-proxy-cleanup-script-b2743fa8"
			configMapCleanupScriptYAML = `apiVersion: v1
data:
  cleanup.sh: |
    #!/bin/sh -e
    OLD_KUBE_PROXY_MODE="$(cat "$1")"
    if [ -z "${OLD_KUBE_PROXY_MODE}" ] || [ "${OLD_KUBE_PROXY_MODE}" = "${KUBE_PROXY_MODE}" ]; then
      echo "${KUBE_PROXY_MODE}" >"$1"
      echo "Nothing to cleanup - the mode didn't change."
      exit 0
    fi

    # Workaround kube-proxy bug (https://github.com/kubernetes/kubernetes/issues/109286) when switching from ipvs to iptables mode.
    # The fix (https://github.com/kubernetes/kubernetes/pull/109288) is present in 1.25+.
    if [ "${EXECUTE_WORKAROUND_FOR_K8S_ISSUE_109286}" = "true" ]; then
      if iptables -t filter -L KUBE-NODE-PORT; then
        echo "KUBE-NODE-PORT chain exists, flushing it..."
        iptables -t filter -F KUBE-NODE-PORT
      fi
    fi

    /usr/local/bin/kube-proxy --v=2 --cleanup --config=/var/lib/kube-proxy-config/config.yaml --proxy-mode="${OLD_KUBE_PROXY_MODE}"
    echo "${KUBE_PROXY_MODE}" >"$1"
immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    app: kubernetes
    gardener.cloud/role: system-component
    origin: gardener
    resources.gardener.cloud/garbage-collectable-reference: "true"
    role: proxy
  name: ` + configMapCleanupScriptName + `
  namespace: kube-system
`

			podSecurityPolicyYAML = `apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  annotations:
    seccomp.security.alpha.kubernetes.io/allowedProfileNames: runtime/default
    seccomp.security.alpha.kubernetes.io/defaultProfileName: runtime/default
  creationTimestamp: null
  name: gardener.kube-system.kube-proxy
spec:
  allowedCapabilities:
  - NET_ADMIN
  allowedHostPaths:
  - pathPrefix: /usr/share/ca-certificates
  - pathPrefix: /var/run/dbus/system_bus_socket
  - pathPrefix: /lib/modules
  - pathPrefix: /var/lib/kube-proxy
  fsGroup:
    rule: RunAsAny
  hostNetwork: true
  hostPorts:
  - max: 10249
    min: 10249
  privileged: true
  runAsUser:
    rule: RunAsAny
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  volumes:
  - hostPath
  - secret
  - configMap
  - projected
`

			clusterRolePSPYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: gardener.cloud:psp:kube-system:kube-proxy
rules:
- apiGroups:
  - policy
  - extensions
  resourceNames:
  - gardener.kube-system.kube-proxy
  resources:
  - podsecuritypolicies
  verbs:
  - use
`

			roleBindingPSPYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  name: gardener.cloud:psp:kube-proxy
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener.cloud:psp:kube-system:kube-proxy
subjects:
- kind: ServiceAccount
  name: kube-proxy
  namespace: kube-system
`

			daemonSetNameFor = func(pool WorkerPool) string {
				return "kube-proxy-" + pool.Name + "-v" + pool.KubernetesVersion.String()
			}
			daemonSetYAMLFor = func(pool WorkerPool, ipvsEnabled, vpaEnabled bool) string {
				referenceAnnotations := func() string {
					var annotations []string

					if ipvsEnabled {
						annotations = []string{
							references.AnnotationKey(references.KindConfigMap, configMapConntrackFixScriptName) + `: ` + configMapConntrackFixScriptName,
							references.AnnotationKey(references.KindConfigMap, configMapCleanupScriptName) + `: ` + configMapCleanupScriptName,
							references.AnnotationKey(references.KindConfigMap, configMapNameFor(ipvsEnabled)) + `: ` + configMapNameFor(ipvsEnabled),
							references.AnnotationKey(references.KindSecret, secretName) + `: ` + secretName,
						}
					} else {
						annotations = []string{
							references.AnnotationKey(references.KindConfigMap, configMapConntrackFixScriptName) + `: ` + configMapConntrackFixScriptName,
							references.AnnotationKey(references.KindConfigMap, configMapCleanupScriptName) + `: ` + configMapCleanupScriptName,
							references.AnnotationKey(references.KindConfigMap, configMapNameFor(ipvsEnabled)) + `: ` + configMapNameFor(ipvsEnabled),
							references.AnnotationKey(references.KindSecret, secretName) + `: ` + secretName,
						}
					}

					return strings.Join(annotations, "\n")
				}

				out := `apiVersion: apps/v1
kind: DaemonSet
metadata:
  annotations:
    ` + utils.Indent(referenceAnnotations(), 4) + `
  creationTimestamp: null
  labels:
    gardener.cloud/role: system-component
    node.gardener.cloud/critical-component: "true"
    origin: gardener
  name: ` + daemonSetNameFor(pool) + `
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: kubernetes
      pool: ` + pool.Name + `
      role: proxy
      version: ` + pool.KubernetesVersion.String() + `
  template:
    metadata:
      annotations:
        ` + utils.Indent(referenceAnnotations(), 8) + `
      creationTimestamp: null
      labels:
        app: kubernetes
        gardener.cloud/role: system-component
        node.gardener.cloud/critical-component: "true"
        origin: gardener
        pool: ` + pool.Name + `
        role: proxy
        version: ` + pool.KubernetesVersion.String() + `
    spec:
      containers:
      - command:
        - /usr/local/bin/kube-proxy
        - --config=/var/lib/kube-proxy-config/config.yaml
        - --v=2
        image: ` + pool.Image + `
        imagePullPolicy: IfNotPresent
        name: kube-proxy
        ports:
        - containerPort: 10249
          hostPort: 10249
          name: metrics
          protocol: TCP
        resources:`

				if vpaEnabled {
					out += `
          limits:
            memory: 2Gi`
				}

				out += `
          requests:
            cpu: 20m
            memory: 64Mi
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /var/lib/kube-proxy-kubeconfig
          name: kubeconfig
        - mountPath: /var/lib/kube-proxy-config
          name: kube-proxy-config
        - mountPath: /etc/ssl/certs
          name: ssl-certs-hosts
          readOnly: true
        - mountPath: /var/run/dbus/system_bus_socket
          name: systembussocket
        - mountPath: /lib/modules
          name: kernel-modules
      - command:
        - /bin/sh
        - /script/conntrack_fix.sh
        image: ` + imageAlpine + `
        imagePullPolicy: IfNotPresent
        name: conntrack-fix
        resources: {}
        securityContext:
          capabilities:
            add:
            - NET_ADMIN
        volumeMounts:
        - mountPath: /script
          name: conntrack-fix-script
      hostNetwork: true
      initContainers:
      - command:
        - sh
        - -c
        - /script/cleanup.sh /var/lib/kube-proxy/mode
        env:
        - name: KUBE_PROXY_MODE`

				if ipvsEnabled {
					out += `
          value: ipvs`
				} else {
					out += `
          value: iptables`
				}

				out += `
        - name: EXECUTE_WORKAROUND_FOR_K8S_ISSUE_109286
          value: "` + strconv.FormatBool(version.ConstraintK8sLess125.Check(pool.KubernetesVersion)) + `"
        image: ` + pool.Image + `
        imagePullPolicy: IfNotPresent
        name: cleanup
        resources: {}
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /script
          name: kube-proxy-cleanup-script
        - mountPath: /lib/modules
          name: kernel-modules
        - mountPath: /var/lib/kube-proxy
          name: kube-proxy-dir
        - mountPath: /var/lib/kube-proxy/mode
          name: kube-proxy-mode
        - mountPath: /var/lib/kube-proxy-kubeconfig
          name: kubeconfig
        - mountPath: /var/lib/kube-proxy-config
          name: kube-proxy-config
      nodeSelector:
        worker.gardener.cloud/kubernetes-version: ` + pool.KubernetesVersion.String() + `
        worker.gardener.cloud/pool: ` + pool.Name + `
      priorityClassName: system-node-critical
      securityContext:
        seccompProfile:
          type: RuntimeDefault
      serviceAccountName: kube-proxy
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - effect: NoExecute
        operator: Exists
      volumes:
      - name: kubeconfig
        secret:
          secretName: ` + secretName + `
      - configMap:
          name: ` + configMapNameFor(ipvsEnabled) + `
        name: kube-proxy-config
      - hostPath:
          path: /usr/share/ca-certificates
        name: ssl-certs-hosts
      - hostPath:
          path: /var/run/dbus/system_bus_socket
        name: systembussocket
      - hostPath:
          path: /lib/modules
        name: kernel-modules
      - configMap:
          defaultMode: 511
          name: ` + configMapCleanupScriptName + `
        name: kube-proxy-cleanup-script
      - hostPath:
          path: /var/lib/kube-proxy
          type: DirectoryOrCreate
        name: kube-proxy-dir
      - hostPath:
          path: /var/lib/kube-proxy/mode
          type: FileOrCreate
        name: kube-proxy-mode
      - configMap:
          name: ` + configMapConntrackFixScriptName + `
        name: conntrack-fix-script
  updateStrategy:
    type: RollingUpdate
status:
  currentNumberScheduled: 0
  desiredNumberScheduled: 0
  numberMisscheduled: 0
  numberReady: 0
`
				return out
			}

			vpaNameFor = func(pool WorkerPool) string {
				return fmt.Sprintf("kube-proxy-%s-v%d.%d", pool.Name, pool.KubernetesVersion.Major(), pool.KubernetesVersion.Minor())
			}
			vpaYAMLFor = func(pool WorkerPool) string {
				return `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: ` + vpaNameFor(pool) + `
  namespace: kube-system
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      controlledValues: RequestsOnly
      maxAllowed:
        cpu: "4"
        memory: 10G
  targetRef:
    apiVersion: apps/v1
    kind: DaemonSet
    name: ` + daemonSetNameFor(pool) + `
  updatePolicy:
    updateMode: Auto
status: {}
`
			}
		)

		Context("IPVS Enabled", func() {
			JustBeforeEach(func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceCentral), managedResourceCentral)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretCentral), managedResourceSecretCentral)).To(BeNotFoundError())

				for _, pool := range values.WorkerPools {
					By(pool.Name)

					managedResource := managedResourceForPool(pool)
					managedResourceSecret := managedResourceSecretForPool(pool)

					Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
					Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
				}

				Expect(component.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceCentral), managedResourceCentral)).To(Succeed())
				expectedMr := &resourcesv1alpha1.ManagedResource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ManagedResource",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResourceCentral.Name,
						Namespace:       managedResourceCentral.Namespace,
						ResourceVersion: "1",
						Labels: map[string]string{
							"origin":    "gardener",
							"component": "kube-proxy",
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						KeepObjects:  pointer.Bool(false),
					},
				}

				managedResourceSecretCentral = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: managedResourceCentral.Spec.SecretRefs[0].Name, Namespace: namespace}}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretCentral), managedResourceSecretCentral)).To(Succeed())
				Expect(managedResourceSecretCentral.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecretCentral.Labels).To(Equal(map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
					"component": "kube-proxy",
					"origin":    "gardener",
				}))
				Expect(managedResourceSecretCentral.Immutable).To(Equal(pointer.Bool(true)))
				Expect(string(managedResourceSecretCentral.Data["serviceaccount__kube-system__kube-proxy.yaml"])).To(Equal(serviceAccountYAML))
				Expect(string(managedResourceSecretCentral.Data["clusterrolebinding____gardener.cloud_target_node-proxier.yaml"])).To(Equal(clusterRoleBindingYAML))
				Expect(string(managedResourceSecretCentral.Data["service__kube-system__kube-proxy.yaml"])).To(Equal(serviceYAML))
				Expect(string(managedResourceSecretCentral.Data["secret__kube-system__"+secretName+".yaml"])).To(Equal(secretYAML))
				Expect(string(managedResourceSecretCentral.Data["configmap__kube-system__"+configMapNameFor(values.IPVSEnabled)+".yaml"])).To(Equal(configMapYAMLFor(values.IPVSEnabled)))
				Expect(string(managedResourceSecretCentral.Data["configmap__kube-system__"+configMapConntrackFixScriptName+".yaml"])).To(Equal(configMapConntrackFixScriptYAML))
				Expect(string(managedResourceSecretCentral.Data["configmap__kube-system__"+configMapCleanupScriptName+".yaml"])).To(Equal(configMapCleanupScriptYAML))

				expectedMr.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: managedResourceSecretCentral.Name}}
				utilruntime.Must(references.InjectAnnotations(expectedMr))
				Expect(managedResourceCentral).To(DeepEqual(expectedMr))

				for _, pool := range values.WorkerPools {
					By(pool.Name)

					managedResource := managedResourceForPool(pool)

					Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					expectedPoolMr := &resourcesv1alpha1.ManagedResource{
						TypeMeta: metav1.TypeMeta{
							APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ManagedResource",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:            managedResource.Name,
							Namespace:       managedResource.Namespace,
							ResourceVersion: "1",
							Labels: map[string]string{
								"origin":             "gardener",
								"component":          "kube-proxy",
								"role":               "pool",
								"pool-name":          pool.Name,
								"kubernetes-version": pool.KubernetesVersion.String(),
							},
						},
						Spec: resourcesv1alpha1.ManagedResourceSpec{
							InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
							KeepObjects:  pointer.Bool(false),
						},
					}

					managedResourceSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: managedResource.Spec.SecretRefs[0].Name, Namespace: namespace}}
					Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
					Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
					Expect(managedResourceSecret.Data).To(HaveLen(1))
					Expect(managedResourceSecret.Immutable).To(Equal(pointer.Bool(true)))
					Expect(managedResourceSecret.Labels).To(Equal(map[string]string{
						"resources.gardener.cloud/garbage-collectable-reference": "true",
						"component":          "kube-proxy",
						"role":               "pool",
						"origin":             "gardener",
						"pool-name":          pool.Name,
						"kubernetes-version": pool.KubernetesVersion.String(),
					}))
					Expect(string(managedResourceSecret.Data["daemonset__kube-system__"+daemonSetNameFor(pool)+".yaml"])).To(Equal(daemonSetYAMLFor(pool, values.IPVSEnabled, values.VPAEnabled)))

					expectedPoolMr.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: managedResourceSecret.Name}}
					utilruntime.Must(references.InjectAnnotations(expectedPoolMr))
					Expect(managedResource).To(DeepEqual(expectedPoolMr))
				}
			})

			Context("PSP is not disabled", func() {
				BeforeEach(func() {
					values.PSPDisabled = false
					component = New(c, namespace, values)
				})

				It("should successfully deploy all resources when PSP is not disabled", func() {
					Expect(managedResourceSecretCentral.Data).To(HaveLen(10))
					Expect(string(managedResourceSecretCentral.Data["podsecuritypolicy____gardener.kube-system.kube-proxy.yaml"])).To(Equal(podSecurityPolicyYAML))
					Expect(string(managedResourceSecretCentral.Data["clusterrole____gardener.cloud_psp_kube-system_kube-proxy.yaml"])).To(Equal(clusterRolePSPYAML))
					Expect(string(managedResourceSecretCentral.Data["rolebinding__kube-system__gardener.cloud_psp_kube-proxy.yaml"])).To(Equal(roleBindingPSPYAML))
				})
			})

			Context("PSP is disabled", func() {
				BeforeEach(func() {
					values.PSPDisabled = true
					component = New(c, namespace, values)
				})

				It("should successfully deploy all resources when PSP is disabled", func() {
					Expect(managedResourceSecretCentral.Data).To(HaveLen(7))
				})
			})
		})

		It("should successfully deploy the expected config when IPVS is disabled", func() {
			values.IPVSEnabled = false
			values.PodNetworkCIDR = &podNetworkCIDR
			component = New(c, namespace, values)

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceCentral), managedResourceCentral)).To(Succeed())
			expectedMR := &resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceCentral.Name,
					Namespace:       managedResourceCentral.Namespace,
					ResourceVersion: "1",
					Labels: map[string]string{
						"origin":    "gardener",
						"component": "kube-proxy",
					},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					KeepObjects:  pointer.Bool(false),
				},
			}

			managedResourceSecretCentral = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceCentral.Spec.SecretRefs[0].Name,
				Namespace: namespace,
			}}

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretCentral), managedResourceSecretCentral)).To(Succeed())
			Expect(managedResourceSecretCentral.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecretCentral.Immutable).To(Equal(pointer.Bool(true)))
			Expect(managedResourceSecretCentral.Labels).To(Equal(map[string]string{
				"resources.gardener.cloud/garbage-collectable-reference": "true",
				"component": "kube-proxy",
				"origin":    "gardener",
			}))
			Expect(string(managedResourceSecretCentral.Data["configmap__kube-system__"+configMapNameFor(values.IPVSEnabled)+".yaml"])).To(Equal(configMapYAMLFor(values.IPVSEnabled)))

			expectedMR.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: managedResourceSecretCentral.Name}}
			utilruntime.Must(references.InjectAnnotations(expectedMR))
			Expect(managedResourceCentral).To(DeepEqual(expectedMR))

			for _, pool := range values.WorkerPools {
				By(pool.Name)

				managedResource := managedResourceForPool(pool)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				poolExpectedMr := &resourcesv1alpha1.ManagedResource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ManagedResource",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResource.Name,
						Namespace:       managedResource.Namespace,
						ResourceVersion: "1",
						Labels: map[string]string{
							"origin":             "gardener",
							"component":          "kube-proxy",
							"role":               "pool",
							"pool-name":          pool.Name,
							"kubernetes-version": pool.KubernetesVersion.String(),
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						KeepObjects:  pointer.Bool(false),
					},
				}

				managedResourceSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:      managedResource.Spec.SecretRefs[0].Name,
					Namespace: namespace,
				}}

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Data).To(HaveLen(1))
				Expect(managedResourceSecret.Labels).To(Equal(map[string]string{
					"component":          "kube-proxy",
					"kubernetes-version": pool.KubernetesVersion.String(),
					"origin":             "gardener",
					"pool-name":          pool.Name,
					"resources.gardener.cloud/garbage-collectable-reference": "true",
					"role": "pool",
				}))
				Expect(managedResourceSecret.Immutable).To(Equal(pointer.Bool(true)))
				Expect(string(managedResourceSecret.Data["daemonset__kube-system__"+daemonSetNameFor(pool)+".yaml"])).To(Equal(daemonSetYAMLFor(pool, values.IPVSEnabled, values.VPAEnabled)))

				poolExpectedMr.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: managedResourceSecret.Name}}
				utilruntime.Must(references.InjectAnnotations(poolExpectedMr))
				Expect(managedResource).To(DeepEqual(poolExpectedMr))
			}
		})

		It("should successfully deploy the expected resources when VPA is enabled", func() {
			values.VPAEnabled = true
			component = New(c, namespace, values)

			Expect(component.Deploy(ctx)).To(Succeed())

			for _, pool := range values.WorkerPools {
				By(pool.Name)

				// assertions for resources specific to the full Kubernetes version
				managedResource := managedResourceForPool(pool)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				managedResourceSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
					Name:      managedResource.Spec.SecretRefs[0].Name,
					Namespace: namespace,
				}}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Data).To(HaveLen(1))
				Expect(managedResourceSecret.Immutable).To(Equal(pointer.Bool(true)))
				Expect(managedResourceSecret.Labels).To(Equal(map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
					"component":          "kube-proxy",
					"role":               "pool",
					"origin":             "gardener",
					"pool-name":          pool.Name,
					"kubernetes-version": pool.KubernetesVersion.String(),
				}))
				Expect(string(managedResourceSecret.Data["daemonset__kube-system__"+daemonSetNameFor(pool)+".yaml"])).To(Equal(daemonSetYAMLFor(pool, values.IPVSEnabled, values.VPAEnabled)))

				expectedMr := &resourcesv1alpha1.ManagedResource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ManagedResource",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResource.Name,
						Namespace:       managedResource.Namespace,
						ResourceVersion: "1",
						Labels: map[string]string{
							"origin":             "gardener",
							"component":          "kube-proxy",
							"role":               "pool",
							"pool-name":          pool.Name,
							"kubernetes-version": pool.KubernetesVersion.String(),
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						SecretRefs:   []corev1.LocalObjectReference{{Name: managedResourceSecret.Name}},
						KeepObjects:  pointer.Bool(false),
					},
				}
				utilruntime.Must(references.InjectAnnotations(expectedMr))

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Immutable).To(Equal(pointer.Bool(true)))
				Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
				Expect(managedResourceSecret.Data).To(HaveLen(1))
				Expect(string(managedResourceSecret.Data["daemonset__kube-system__"+daemonSetNameFor(pool)+".yaml"])).To(Equal(daemonSetYAMLFor(pool, values.IPVSEnabled, values.VPAEnabled)))

				// assertions for resources specific to the major/minor parts only of the Kubernetes version
				managedResourceForMajorMinorVersionOnly := managedResourceForPoolForMajorMinorVersionOnly(pool)
				managedResourceSecretForMajorMinorVersionOnly := managedResourceSecretForPoolForMajorMinorVersionOnly(pool)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceForMajorMinorVersionOnly), managedResourceForMajorMinorVersionOnly)).To(Succeed())
				expectedMrForMajorMinorVersionOnly := &resourcesv1alpha1.ManagedResource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ManagedResource",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResourceForMajorMinorVersionOnly.Name,
						Namespace:       managedResourceForMajorMinorVersionOnly.Namespace,
						ResourceVersion: "1",
						Labels: map[string]string{
							"origin":             "gardener",
							"component":          "kube-proxy",
							"role":               "pool",
							"pool-name":          pool.Name,
							"kubernetes-version": fmt.Sprintf("%d.%d", pool.KubernetesVersion.Major(), pool.KubernetesVersion.Minor()),
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						SecretRefs: []corev1.LocalObjectReference{{
							Name: managedResourceForMajorMinorVersionOnly.Spec.SecretRefs[0].Name,
						}},
						KeepObjects: pointer.Bool(false),
					},
				}
				utilruntime.Must(references.InjectAnnotations(expectedMrForMajorMinorVersionOnly))
				Expect(managedResourceForMajorMinorVersionOnly).To(Equal(expectedMrForMajorMinorVersionOnly))

				managedResourceSecretForMajorMinorVersionOnly.Name = managedResourceForMajorMinorVersionOnly.Spec.SecretRefs[0].Name
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretForMajorMinorVersionOnly), managedResourceSecretForMajorMinorVersionOnly)).To(Succeed())
				Expect(managedResourceSecretForMajorMinorVersionOnly.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecretForMajorMinorVersionOnly.Immutable).To(Equal(pointer.Bool(true)))
				Expect(managedResourceSecretForMajorMinorVersionOnly.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
				Expect(managedResourceSecretForMajorMinorVersionOnly.Data).To(HaveLen(1))
				Expect(string(managedResourceSecretForMajorMinorVersionOnly.Data["verticalpodautoscaler__kube-system__"+vpaNameFor(pool)+".yaml"])).To(Equal(vpaYAMLFor(pool)))
				Expect(managedResource).To(DeepEqual(expectedMr))
			}
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources despite undesired managed resources", func() {
			Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())
			// TODO(dimityrmirchev): Remove this once mr secrets are turned into garbage-collectable, immutable secrets, after Gardener v1.90
			managedResourceSecretCentral = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedresource-" + managedResourceCentral.Name,
					Namespace: namespace,
					Labels:    map[string]string{"component": "kube-proxy"},
				},
			}
			Expect(c.Create(ctx, managedResourceSecretCentral)).To(Succeed())

			undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
			undesiredManagedResource := managedResourceForPool(undesiredPool)
			undesiredManagedResourceSecret := managedResourceSecretForPool(undesiredPool)

			Expect(c.Create(ctx, undesiredManagedResource)).To(Succeed())
			Expect(c.Create(ctx, undesiredManagedResourceSecret)).To(Succeed())

			for _, pool := range values.WorkerPools {
				By(pool.Name)

				managedResource := managedResourceForPool(pool)
				managedResourceSecret := managedResourceSecretForPool(pool)

				Expect(c.Create(ctx, managedResource)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			}

			Expect(component.Destroy(ctx)).To(Succeed())

			for _, pool := range values.WorkerPools {
				By(pool.Name)

				managedResource := managedResourceForPool(pool)
				managedResourceSecret := managedResourceSecretForPool(pool)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
			}

			Expect(c.Get(ctx, client.ObjectKeyFromObject(undesiredManagedResource), undesiredManagedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(undesiredManagedResourceSecret), undesiredManagedResourceSecret)).To(BeNotFoundError())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceCentral), managedResourceCentral)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretCentral), managedResourceSecretCentral)).To(BeNotFoundError())
		})
	})

	Describe("#DeleteStaleResources", func() {
		It("should successfully delete all stale resources", func() {
			Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())
			// TODO(dimityrmirchev): Remove this once mr secrets are turned into garbage-collectable, immutable secrets, after Gardener v1.90
			managedResourceSecretCentral = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedresource-" + managedResourceCentral.Name,
					Namespace: namespace,
					Labels:    map[string]string{"component": "kube-proxy"},
				},
			}
			Expect(c.Create(ctx, managedResourceSecretCentral)).To(Succeed())

			undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
			undesiredManagedResource := managedResourceForPool(undesiredPool)
			undesiredManagedResourceSecret := managedResourceSecretForPool(undesiredPool)
			undesiredManagedResourceForMajorMinorVersionOnly := managedResourceForPoolForMajorMinorVersionOnly(undesiredPool)
			undesiredManagedResourceSecretForMajorMinorVersionOnly := managedResourceSecretForPoolForMajorMinorVersionOnly(undesiredPool)

			Expect(c.Create(ctx, undesiredManagedResource)).To(Succeed())
			Expect(c.Create(ctx, undesiredManagedResourceSecret)).To(Succeed())
			Expect(c.Create(ctx, undesiredManagedResourceForMajorMinorVersionOnly)).To(Succeed())
			Expect(c.Create(ctx, undesiredManagedResourceSecretForMajorMinorVersionOnly)).To(Succeed())

			for _, pool := range values.WorkerPools {
				By(pool.Name)

				managedResource := managedResourceForPool(pool)
				managedResourceSecret := managedResourceSecretForPool(pool)
				managedResourceForMajorMinorVersionOnly := managedResourceForPoolForMajorMinorVersionOnly(pool)
				managedResourceSecretForMajorMinorVersionOnly := managedResourceSecretForPoolForMajorMinorVersionOnly(pool)

				Expect(c.Create(ctx, managedResource)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())
				Expect(c.Create(ctx, managedResourceForMajorMinorVersionOnly)).To(Succeed())
				Expect(c.Create(ctx, managedResourceSecretForMajorMinorVersionOnly)).To(Succeed())
			}

			Expect(component.DeleteStaleResources(ctx)).To(Succeed())

			for _, pool := range values.WorkerPools {
				By(pool.Name)

				managedResource := managedResourceForPool(pool)
				managedResourceSecret := managedResourceSecretForPool(pool)
				managedResourceForMajorMinorVersionOnly := managedResourceForPoolForMajorMinorVersionOnly(pool)
				managedResourceSecretForMajorMinorVersionOnly := managedResourceSecretForPoolForMajorMinorVersionOnly(pool)

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceForMajorMinorVersionOnly), managedResourceForMajorMinorVersionOnly)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretForMajorMinorVersionOnly), managedResourceSecretForMajorMinorVersionOnly)).To(Succeed())
			}

			Expect(c.Get(ctx, client.ObjectKeyFromObject(undesiredManagedResource), undesiredManagedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(undesiredManagedResourceSecret), undesiredManagedResourceSecret)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(undesiredManagedResourceForMajorMinorVersionOnly), undesiredManagedResourceForMajorMinorVersionOnly)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(undesiredManagedResourceSecretForMajorMinorVersionOnly), undesiredManagedResourceSecretForMajorMinorVersionOnly)).To(BeNotFoundError())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceCentral), managedResourceCentral)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecretCentral), managedResourceSecretCentral)).To(Succeed())
		})
	})

	Context("waiting functions", func() {
		var fakeOps *retryfake.Ops

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 2}
			DeferCleanup(test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			))
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the central ManagedResource doesn't become healthy", func() {
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceCentral.Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				for _, pool := range values.WorkerPools {
					By(pool.Name)

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPool(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())
				}

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should fail because a pool-specific ManagedResource doesn't become healthy", func() {
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceCentral.Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				for _, pool := range values.WorkerPools {
					By(pool.Name)

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPool(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: unhealthyManagedResourceStatus,
					})).To(Succeed())
				}

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should fail because a pool-specific ManagedResource for major/minor version only doesn't become healthy", func() {
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceCentral.Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				for _, pool := range values.WorkerPools {
					By(pool.Name)

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPool(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPoolForMajorMinorVersionOnly(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: unhealthyManagedResourceStatus,
					})).To(Succeed())
				}

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceCentral.Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				for _, pool := range values.WorkerPools {
					By(pool.Name)

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPool(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPoolForMajorMinorVersionOnly(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())
				}

				Expect(component.Wait(ctx)).To(Succeed())
			})

			It("should successfully wait for the managed resource to become healthy despite undesired managed resource unhealthy", func() {
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceCentral.Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceForPool(undesiredPool).Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				for _, pool := range values.WorkerPools {
					By(pool.Name)

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPool(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPoolForMajorMinorVersionOnly(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())
				}

				Expect(component.Wait(ctx)).To(Succeed())
			})

			It("should successfully wait for the managed resource to become healthy despite undesired managed resource for major/minor version only unhealthy", func() {
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceCentral.Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceForPool(undesiredPool).Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceForPoolForMajorMinorVersionOnly(undesiredPool).Name,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				for _, pool := range values.WorkerPools {
					By(pool.Name)

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPool(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())

					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       managedResourceForPoolForMajorMinorVersionOnly(pool).Name,
							Namespace:  namespace,
							Generation: 1,
						},
						Status: healthyManagedResourceStatus,
					})).To(Succeed())
				}

				Expect(component.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out because of central resource", func() {
				Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())

				for _, pool := range values.WorkerPools {
					Expect(c.Create(ctx, managedResourceForPool(pool))).To(Succeed())
					Expect(c.Delete(ctx, managedResourceForPool(pool))).To(Succeed())
				}

				Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should fail when the wait for the managed resource deletion times out because of pool-specific resource", func() {
				Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())
				Expect(c.Delete(ctx, managedResourceCentral)).To(Succeed())

				for _, pool := range values.WorkerPools {
					Expect(c.Create(ctx, managedResourceForPool(pool))).To(Succeed())
				}

				Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should successfully wait for the deletion", func() {
				for _, pool := range values.WorkerPools {
					managedResource := managedResourceForPool(pool)
					Expect(c.Create(ctx, managedResource)).To(Succeed())
					Expect(c.Delete(ctx, managedResource)).To(Succeed())
				}

				Expect(component.WaitCleanup(ctx)).To(Succeed())
			})

			It("should successfully wait for the deletion despite undesired still existing managed resources", func() {
				Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())
				Expect(c.Delete(ctx, managedResourceCentral)).To(Succeed())

				undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
				undesiredManagedResource := managedResourceForPool(undesiredPool)
				Expect(c.Create(ctx, undesiredManagedResource)).To(Succeed())
				Expect(c.Delete(ctx, undesiredManagedResource)).To(Succeed())

				for _, pool := range values.WorkerPools {
					managedResource := managedResourceForPool(pool)

					Expect(c.Create(ctx, managedResource)).To(Succeed())
					Expect(c.Delete(ctx, managedResource)).To(Succeed())
				}

				Expect(component.WaitCleanup(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanupStaleResources", func() {
			It("should succeed when there is nothing to wait for", func() {
				Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())

				for _, pool := range values.WorkerPools {
					Expect(c.Create(ctx, managedResourceForPool(pool))).To(Succeed())
				}

				Expect(component.WaitCleanupStaleResources(ctx)).To(Succeed())
			})

			It("should fail when the wait for the managed resource deletion times out", func() {
				Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())

				undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
				Expect(c.Create(ctx, managedResourceForPool(undesiredPool))).To(Succeed())

				Expect(component.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should fail when the wait for the managed resource for major/minor version only deletion times out", func() {
				Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())

				undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
				Expect(c.Create(ctx, managedResourceForPoolForMajorMinorVersionOnly(undesiredPool))).To(Succeed())

				Expect(component.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should successfully wait for the deletion", func() {
				Expect(c.Create(ctx, managedResourceCentral)).To(Succeed())

				undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
				undesiredManagedResource := managedResourceForPool(undesiredPool)
				undesiredManagedResourceForMajorMinorVersionOnly := managedResourceForPoolForMajorMinorVersionOnly(undesiredPool)

				Expect(c.Create(ctx, undesiredManagedResource)).To(Succeed())
				Expect(c.Create(ctx, undesiredManagedResourceForMajorMinorVersionOnly)).To(Succeed())
				Expect(component.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("still exists")))

				Expect(c.Delete(ctx, undesiredManagedResource)).To(Succeed())
				Expect(component.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("still exists")))

				Expect(c.Delete(ctx, undesiredManagedResourceForMajorMinorVersionOnly)).To(Succeed())
				Expect(component.WaitCleanupStaleResources(ctx)).To(Succeed())
			})

			It("should successfully wait for the deletion despite desired existing managed resources", func() {
				for _, pool := range values.WorkerPools {
					Expect(c.Create(ctx, managedResourceForPool(pool))).To(Succeed())
				}

				undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
				undesiredManagedResource := managedResourceForPool(undesiredPool)

				Expect(c.Create(ctx, undesiredManagedResource)).To(Succeed())
				Expect(component.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("still exists")))

				Expect(c.Delete(ctx, undesiredManagedResource)).To(Succeed())
				Expect(component.WaitCleanupStaleResources(ctx)).To(Succeed())
			})

			It("should successfully wait for the deletion despite desired existing managed resources for major/minor version only", func() {
				for _, pool := range values.WorkerPools {
					Expect(c.Create(ctx, managedResourceForPoolForMajorMinorVersionOnly(pool))).To(Succeed())
				}

				undesiredPool := WorkerPool{Name: "foo", KubernetesVersion: semver.MustParse("1.1.1")}
				undesiredManagedResource := managedResourceForPool(undesiredPool)

				Expect(c.Create(ctx, undesiredManagedResource)).To(Succeed())
				Expect(component.WaitCleanupStaleResources(ctx)).To(MatchError(ContainSubstring("still exists")))

				Expect(c.Delete(ctx, undesiredManagedResource)).To(Succeed())
				Expect(component.WaitCleanupStaleResources(ctx)).To(Succeed())
			})
		})
	})
})

var (
	unhealthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
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
	}
	healthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
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
	}
)
