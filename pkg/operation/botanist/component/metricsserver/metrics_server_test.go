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

package metricsserver_test

import (
	"context"
	"fmt"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/metricsserver"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("MetricsServer", func() {
	var (
		ctrl          *gomock.Controller
		c             *mockclient.MockClient
		metricsServer Interface

		ctx               = context.TODO()
		fakeErr           = fmt.Errorf("fake error")
		namespace         = "shoot--foo--bar"
		image             = "k8s.gcr.io/metrics-server:v4.5.6"
		kubeAPIServerHost = "foo.bar"

		secretNameCA         = "ca-metrics-server"
		secretChecksumCA     = "1234"
		secretDataCA         = map[string][]byte{"ca.crt": []byte("bar")}
		secretNameServer     = "metrics-server"
		secretChecksumServer = "5678"
		secretDataServer     = map[string][]byte{"bar": []byte("baz")}
		secrets              = Secrets{
			CA:     component.Secret{Name: secretNameCA, Checksum: secretChecksumCA, Data: secretDataCA},
			Server: component.Secret{Name: secretNameServer, Checksum: secretChecksumServer, Data: secretDataServer},
		}

		secretName = "metrics-server-3a086058"
		secretYAML = `apiVersion: v1
data:
  bar: YmF6
immutable: true
kind: Secret
metadata:
  creationTimestamp: null
  labels:
    resources.gardener.cloud/garbage-collectable-reference: "true"
  name: ` + secretName + `
  namespace: kube-system
type: kubernetes.io/tls
`
		serviceYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    kubernetes.io/name: metrics-server
  name: metrics-server
  namespace: kube-system
spec:
  ports:
  - port: 443
    protocol: TCP
    targetPort: 8443
  selector:
    k8s-app: metrics-server
status:
  loadBalancer: {}
`
		apiServiceYAML = `apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  creationTimestamp: null
  name: v1beta1.metrics.k8s.io
spec:
  caBundle: YmFy
  group: metrics.k8s.io
  groupPriorityMinimum: 100
  service:
    name: metrics-server
    namespace: kube-system
  version: v1beta1
  versionPriority: 100
status: {}
`
		vpaYAML = `apiVersion: autoscaling.k8s.io/v1beta2
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  name: metrics-server
  namespace: kube-system
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      minAllowed:
        cpu: 50m
        memory: 150Mi
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: metrics-server
  updatePolicy:
    updateMode: Auto
status: {}
`
		clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: system:metrics-server
rules:
- apiGroups:
  - ""
  resources:
  - pods
  - nodes
  - nodes/stats
  - namespaces
  - configmaps
  verbs:
  - get
  - list
  - watch
`
		clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  name: system:metrics-server
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:metrics-server
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
`
		clusterRoleBindingAuthDelegatorYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  name: metrics-server:system:auth-delegator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
`
		roleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  annotations:
    resources.gardener.cloud/delete-on-invalid-update: "true"
  creationTimestamp: null
  name: metrics-server-auth-reader
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: extension-apiserver-authentication-reader
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
`
		serviceAccountYAML = `apiVersion: v1
kind: ServiceAccount
metadata:
  creationTimestamp: null
  name: metrics-server
  namespace: kube-system
`

		deploymentYAMLWithoutHostEnv = `apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    ` + references.AnnotationKey(references.KindSecret, secretName) + `: ` + secretName + `
  creationTimestamp: null
  labels:
    gardener.cloud/role: system-component
    k8s-app: metrics-server
    origin: gardener
  name: metrics-server
  namespace: kube-system
spec:
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      k8s-app: metrics-server
  strategy:
    rollingUpdate:
      maxUnavailable: 0
  template:
    metadata:
      annotations:
        ` + references.AnnotationKey(references.KindSecret, secretName) + `: ` + secretName + `
      creationTimestamp: null
      labels:
        gardener.cloud/role: system-component
        k8s-app: metrics-server
        networking.gardener.cloud/from-seed: allowed
        networking.gardener.cloud/to-apiserver: allowed
        networking.gardener.cloud/to-dns: allowed
        networking.gardener.cloud/to-kubelet: allowed
        origin: gardener
    spec:
      containers:
      - command:
        - /metrics-server
        - --authorization-always-allow-paths=/livez,/readyz
        - --profiling=false
        - --cert-dir=/home/certdir
        - --secure-port=8443
        - --kubelet-insecure-tls
        - --kubelet-preferred-address-types=Hostname,InternalDNS,InternalIP,ExternalDNS,ExternalIP
        - --tls-cert-file=/srv/metrics-server/tls/tls.crt
        - --tls-private-key-file=/srv/metrics-server/tls/tls.key
        image: ` + image + `
        imagePullPolicy: IfNotPresent
        livenessProbe:
          failureThreshold: 1
          httpGet:
            path: /livez
            port: 8443
            scheme: HTTPS
          initialDelaySeconds: 30
          periodSeconds: 30
        name: metrics-server
        readinessProbe:
          failureThreshold: 1
          httpGet:
            path: /readyz
            port: 8443
            scheme: HTTPS
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          limits:
            cpu: 500m
            memory: 1Gi
          requests:
            cpu: 50m
            memory: 150Mi
        volumeMounts:
        - mountPath: /srv/metrics-server/tls
          name: metrics-server
      dnsPolicy: Default
      nodeSelector:
        worker.gardener.cloud/system-components: "true"
      priorityClassName: system-cluster-critical
      securityContext:
        fsGroup: 65534
        runAsUser: 65534
      serviceAccountName: metrics-server
      tolerations:
      - key: CriticalAddonsOnly
        operator: Exists
      volumes:
      - name: metrics-server
        secret:
          secretName: ` + secretName + `
status: {}
`
		deploymentYAMLWithHostEnv = `apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    ` + references.AnnotationKey(references.KindSecret, secretName) + `: ` + secretName + `
  creationTimestamp: null
  labels:
    gardener.cloud/role: system-component
    k8s-app: metrics-server
    origin: gardener
  name: metrics-server
  namespace: kube-system
spec:
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      k8s-app: metrics-server
  strategy:
    rollingUpdate:
      maxUnavailable: 0
  template:
    metadata:
      annotations:
        ` + references.AnnotationKey(references.KindSecret, secretName) + `: ` + secretName + `
      creationTimestamp: null
      labels:
        gardener.cloud/role: system-component
        k8s-app: metrics-server
        networking.gardener.cloud/from-seed: allowed
        networking.gardener.cloud/to-apiserver: allowed
        networking.gardener.cloud/to-dns: allowed
        networking.gardener.cloud/to-kubelet: allowed
        origin: gardener
    spec:
      containers:
      - command:
        - /metrics-server
        - --authorization-always-allow-paths=/livez,/readyz
        - --profiling=false
        - --cert-dir=/home/certdir
        - --secure-port=8443
        - --kubelet-insecure-tls
        - --kubelet-preferred-address-types=Hostname,InternalDNS,InternalIP,ExternalDNS,ExternalIP
        - --tls-cert-file=/srv/metrics-server/tls/tls.crt
        - --tls-private-key-file=/srv/metrics-server/tls/tls.key
        env:
        - name: KUBERNETES_SERVICE_HOST
          value: ` + kubeAPIServerHost + `
        image: ` + image + `
        imagePullPolicy: IfNotPresent
        livenessProbe:
          failureThreshold: 1
          httpGet:
            path: /livez
            port: 8443
            scheme: HTTPS
          initialDelaySeconds: 30
          periodSeconds: 30
        name: metrics-server
        readinessProbe:
          failureThreshold: 1
          httpGet:
            path: /readyz
            port: 8443
            scheme: HTTPS
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          limits:
            cpu: 500m
            memory: 1Gi
          requests:
            cpu: 50m
            memory: 150Mi
        volumeMounts:
        - mountPath: /srv/metrics-server/tls
          name: metrics-server
      dnsPolicy: Default
      nodeSelector:
        worker.gardener.cloud/system-components: "true"
      priorityClassName: system-cluster-critical
      securityContext:
        fsGroup: 65534
        runAsUser: 65534
      serviceAccountName: metrics-server
      tolerations:
      - key: CriticalAddonsOnly
        operator: Exists
      volumes:
      - name: metrics-server
        secret:
          secretName: ` + secretName + `
status: {}
`

		managedResourceName       = "shoot-core-metrics-server"
		managedResourceSecretName = "managedresource-shoot-core-metrics-server"

		managedResourceSecret *corev1.Secret
		managedResource       *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		metricsServer = New(c, namespace, image, false, nil)

		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceSecretName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"apiservice____v1beta1.metrics.k8s.io.yaml":                       []byte(apiServiceYAML),
				"clusterrole____system_metrics-server.yaml":                       []byte(clusterRoleYAML),
				"clusterrolebinding____system_metrics-server.yaml":                []byte(clusterRoleBindingYAML),
				"clusterrolebinding____metrics-server_system_auth-delegator.yaml": []byte(clusterRoleBindingAuthDelegatorYAML),
				"deployment__kube-system__metrics-server.yaml":                    []byte(deploymentYAMLWithoutHostEnv),
				"rolebinding__kube-system__metrics-server-auth-reader.yaml":       []byte(roleBindingYAML),
				"secret__kube-system__" + secretName + ".yaml":                    []byte(secretYAML),
				"service__kube-system__metrics-server.yaml":                       []byte(serviceYAML),
				"serviceaccount__kube-system__metrics-server.yaml":                []byte(serviceAccountYAML),
			},
		}
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
				Labels:    map[string]string{"origin": "gardener"},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs: []corev1.LocalObjectReference{
					{Name: managedResourceSecretName},
				},
				InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
				KeepObjects:  pointer.Bool(false),
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		Context("missing secret information", func() {
			It("should return an error because the CA secret information is not provided", func() {
				Expect(metricsServer.Deploy(ctx)).To(MatchError(ContainSubstring("missing CA secret information")))
			})

			It("should return an error because the Server secret information is not provided", func() {
				metricsServer.SetSecrets(Secrets{CA: component.Secret{Name: secretNameCA, Checksum: secretChecksumCA}})
				Expect(metricsServer.Deploy(ctx)).To(MatchError(ContainSubstring("missing server secret information")))
			})
		})

		Context("secret information available", func() {
			BeforeEach(func() {
				metricsServer.SetSecrets(secrets)
			})

			It("should fail because the managed resource secret cannot be updated", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
				)

				Expect(metricsServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the managed resource cannot be updated", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr),
				)

				Expect(metricsServer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully deploy all resources (w/o VPA, w/o host env)", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(managedResourceSecret))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(managedResource))
					}),
				)

				Expect(metricsServer.Deploy(ctx)).To(Succeed())
			})

			It("should successfully deploy all resources (w/ VPA, w/ host env)", func() {
				metricsServer = New(c, namespace, image, true, &kubeAPIServerHost)
				metricsServer.SetSecrets(secrets)

				managedResourceSecret.Data["deployment__kube-system__metrics-server.yaml"] = []byte(deploymentYAMLWithHostEnv)
				managedResourceSecret.Data["verticalpodautoscaler__kube-system__metrics-server.yaml"] = []byte(vpaYAML)

				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(managedResourceSecret))
					}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
						Expect(obj).To(DeepEqual(managedResource))
					}),
				)

				Expect(metricsServer.Deploy(ctx)).To(Succeed())
			})
		})
	})

	Describe("#Destroy", func() {
		var managedResourceToDelete *resourcesv1alpha1.ManagedResource

		BeforeEach(func() {
			managedResourceToDelete = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceName,
					Namespace: namespace,
				},
			}
		})

		It("should fail because the managed resource cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResourceToDelete).Return(fakeErr),
			)

			Expect(metricsServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the managed resource secret cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResourceToDelete),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}).Return(fakeErr),
			)

			Expect(metricsServer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully delete all the resources", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResourceToDelete),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}),
			)

			Expect(metricsServer.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(metricsServer.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(metricsServer.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
