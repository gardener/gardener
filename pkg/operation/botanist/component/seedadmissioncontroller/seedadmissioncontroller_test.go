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

package seedadmissioncontroller_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/seedadmissioncontroller"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("SeedAdmission", func() {
	var (
		ctrl       *gomock.Controller
		c          *mockclient.MockClient
		fakeClient client.Client
		sm         secretsmanager.Interface

		seedAdmission component.DeployWaiter

		ctx       = context.TODO()
		fakeErr   = fmt.Errorf("fake error")
		namespace = "shoot--foo--bar"
		image     = "gsac:v1.2.3"

		clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  labels:
    app: gardener
    role: seed-admission-controller
  name: gardener-seed-admission-controller
rules:
- apiGroups:
  - apiextensions.k8s.io
  resources:
  - customresourcedefinitions
  verbs:
  - get
  - list
- apiGroups:
  - druid.gardener.cloud
  resources:
  - etcds
  verbs:
  - get
  - list
- apiGroups:
  - extensions.gardener.cloud
  resources:
  - backupbuckets
  - backupentries
  - bastions
  - containerruntimes
  - controlplanes
  - dnsrecords
  - extensions
  - infrastructures
  - networks
  - operatingsystemconfigs
  - workers
  - clusters
  verbs:
  - get
  - list
`
		clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  labels:
    app: gardener
    role: seed-admission-controller
  name: gardener-seed-admission-controller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener-seed-admission-controller
subjects:
- kind: ServiceAccount
  name: gardener-seed-admission-controller
  namespace: shoot--foo--bar
`
		deploymentYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    app: gardener
    role: seed-admission-controller
  name: gardener-seed-admission-controller
  namespace: shoot--foo--bar
spec:
  replicas: 3
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      app: gardener
      role: seed-admission-controller
  strategy:
    rollingUpdate:
      maxUnavailable: 1
    type: RollingUpdate
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: gardener
        role: seed-admission-controller
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - podAffinityTerm:
              labelSelector:
                matchLabels:
                  app: gardener
                  role: seed-admission-controller
              topologyKey: kubernetes.io/hostname
            weight: 100
      containers:
      - command:
        - /gardener-seed-admission-controller
        - --port=10250
        - --tls-cert-dir=/srv/gardener-seed-admission-controller
        - --metrics-bind-address=:8080
        - --health-bind-address=:8081
        image: ` + image + `
        imagePullPolicy: IfNotPresent
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
            scheme: HTTP
          initialDelaySeconds: 5
        name: gardener-seed-admission-controller
        ports:
        - containerPort: 8080
          name: metrics
          protocol: TCP
        - containerPort: 10250
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
            scheme: HTTP
          initialDelaySeconds: 10
        resources:
          limits:
            memory: 100Mi
          requests:
            cpu: 20m
            memory: 50Mi
        volumeMounts:
        - mountPath: /srv/gardener-seed-admission-controller
          name: gardener-seed-admission-controller-tls
          readOnly: true
      serviceAccountName: gardener-seed-admission-controller
      volumes:
      - name: gardener-seed-admission-controller-tls
        secret:
          secretName: gardener-seed-admission-controller-server
status: {}
`
		pdbYAML = `apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  creationTimestamp: null
  labels:
    app: gardener
    role: seed-admission-controller
  name: gardener-seed-admission-controller
  namespace: shoot--foo--bar
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app: gardener
      role: seed-admission-controller
status:
  currentHealthy: 0
  desiredHealthy: 0
  disruptionsAllowed: 0
  expectedPods: 0
`
		serviceYAML = `apiVersion: v1
kind: Service
metadata:
  creationTimestamp: null
  labels:
    app: gardener
    role: seed-admission-controller
  name: gardener-seed-admission-controller
  namespace: shoot--foo--bar
spec:
  ports:
  - name: metrics
    port: 8080
    protocol: TCP
    targetPort: 8080
  - name: health
    port: 8081
    protocol: TCP
    targetPort: 8081
  - name: web
    port: 443
    protocol: TCP
    targetPort: 10250
  selector:
    app: gardener
    role: seed-admission-controller
  type: ClusterIP
status:
  loadBalancer: {}
`
		serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  labels:
    app: gardener
    role: seed-admission-controller
  name: gardener-seed-admission-controller
  namespace: shoot--foo--bar
`
		validatingWebhookConfigurationYAML = `apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  creationTimestamp: null
  labels:
    app: gardener
    role: seed-admission-controller
  name: gardener-seed-admission-controller
webhooks:
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    service:
      name: gardener-seed-admission-controller
      namespace: shoot--foo--bar
      path: /webhooks/validate-extension-crd-deletion
  failurePolicy: Fail
  matchPolicy: Exact
  name: crds.seed.admission.core.gardener.cloud
  namespaceSelector: {}
  objectSelector:
    matchLabels:
      gardener.cloud/deletion-protected: "true"
  rules:
  - apiGroups:
    - apiextensions.k8s.io
    apiVersions:
    - v1beta1
    - v1
    operations:
    - DELETE
    resources:
    - customresourcedefinitions
  sideEffects: None
  timeoutSeconds: 10
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    service:
      name: gardener-seed-admission-controller
      namespace: shoot--foo--bar
      path: /webhooks/validate-extension-crd-deletion
  failurePolicy: Fail
  matchPolicy: Exact
  name: crs.seed.admission.core.gardener.cloud
  namespaceSelector: {}
  rules:
  - apiGroups:
    - druid.gardener.cloud
    apiVersions:
    - v1alpha1
    operations:
    - DELETE
    resources:
    - etcds
  - apiGroups:
    - extensions.gardener.cloud
    apiVersions:
    - v1alpha1
    operations:
    - DELETE
    resources:
    - backupbuckets
    - backupentries
    - bastions
    - containerruntimes
    - controlplanes
    - dnsrecords
    - extensions
    - infrastructures
    - networks
    - operatingsystemconfigs
    - workers
  sideEffects: None
  timeoutSeconds: 10
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    service:
      name: gardener-seed-admission-controller
      namespace: shoot--foo--bar
      path: /validate-extensions-gardener-cloud-v1alpha1-backupbucket
  failurePolicy: Fail
  matchPolicy: Exact
  name: validation.extensions.backupbucket.admission.core.gardener.cloud
  namespaceSelector: {}
  rules:
  - apiGroups:
    - extensions.gardener.cloud
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - backupbuckets
  sideEffects: None
  timeoutSeconds: 10
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMrakNDQWVLZ0F3SUJBZ0lVVHAzWHZocldPVk04WkdlODZZb1hNVi9VSjdBd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0ZURVRNQkVHQTFVRUF4TUthM1ZpWlhKdVpYUmxjekFlRncweE9UQXlNamN4TlRNME1EQmFGdzB5TkRBeQpNall4TlRNME1EQmFNQlV4RXpBUkJnTlZCQU1UQ210MVltVnlibVYwWlhNd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDeWkwUUdPY3YyYlRmM044T0xOOTdSd3NnSDZRQXI4d1NwQU9ydHRCSmcKRm5mblUyVDFSSGd4bTdxZDE5MFdMOERDaHYwZFpmNzZkNmVTUTRacmpqeUFyVHp1ZmI0RHRQd2crVldxN1h2RgpCTnluKzJoZjRTeVNrd2Q2azdYTGhVVFJ4MDQ4SWJCeUM0ditGRXZtb0xBd3JjMGQwRzE0ZWM2c25EKzdqTzdlCmt5a1EvTmdBT0w3UDZrRHM5ejYrYk9mZ0YwbkdOK2JtZVdRcUplalIwdCtPeVFEQ3g1L0ZNdFVmRVZSNVFYODAKYWVlZmdwM0pGWmI2ZkF3OUtoTHRkUlYzRlAwdHo2aFMrZTRTZzBtd0FBT3FpalpzVjg3a1A1R1l6anRjZkExMgpsRFlsL25iMUd0VnZ2a1FENDlWblY3bURubDZtRzNMQ01OQ05INldsWk52M0FnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQkJqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCU0ZBM0x2Sk0yMWQ4cXMKWlZWQ2U2UnJUVDl3aVRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQW5zL0VKM3lLc2p0SVNvdGVRNzE0cjJVbQpCTVB5VVlUVGRSSEQ4TFpNZDNSeWt2c2FjRjJsMnk4OE56NndKY0F1b1VqMWg4YUJEUDVvWFZ0Tm1GVDlqeWJTClRYclJ2V2krYWVZZGI1NTZuRUE1L2E5NGUrY2IrQ2szcXkvMXhnUW9TNDU3QVpRT0Rpc0RaTkJZV2tBRnMyTGMKdWNwY0F0WEp0SXRoVm03RmpvQUhZY3NyWTA0eUFpWUVKTEQwMlRqVURYZzRpR09HTWtWSGRtaGF3QkRCRjNBagplc2ZjcUZ3amk2SnlBS0ZSQUNQb3d5a1FPTkZ3VVNvbTg5dVlFU1NDSkZ2TkNrOU1KbWpKMlB6RFV0NkN5cFI0CmVwRmRkMWZYTHd1d243ZnZQTW1KcUQzSHRMYWxYMUFabVBrK0JJOGV6ZkFpVmNWcW5USlFNWGxZUHBZZTlBPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQ==
    service:
      name: gardener-seed-admission-controller
      namespace: shoot--foo--bar
      path: /validate-extensions-gardener-cloud-v1alpha1-backupentry
  failurePolicy: Fail
  matchPolicy: Exact
  name: validation.extensions.backupentry.admission.core.gardener.cloud
  namespaceSelector: {}
  rules:
  - apiGroups:
    - extensions.gardener.cloud
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - backupentries
  sideEffects: None
  timeoutSeconds: 10
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMrakNDQWVLZ0F3SUJBZ0lVVHAzWHZocldPVk04WkdlODZZb1hNVi9VSjdBd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0ZURVRNQkVHQTFVRUF4TUthM1ZpWlhKdVpYUmxjekFlRncweE9UQXlNamN4TlRNME1EQmFGdzB5TkRBeQpNall4TlRNME1EQmFNQlV4RXpBUkJnTlZCQU1UQ210MVltVnlibVYwWlhNd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDeWkwUUdPY3YyYlRmM044T0xOOTdSd3NnSDZRQXI4d1NwQU9ydHRCSmcKRm5mblUyVDFSSGd4bTdxZDE5MFdMOERDaHYwZFpmNzZkNmVTUTRacmpqeUFyVHp1ZmI0RHRQd2crVldxN1h2RgpCTnluKzJoZjRTeVNrd2Q2azdYTGhVVFJ4MDQ4SWJCeUM0ditGRXZtb0xBd3JjMGQwRzE0ZWM2c25EKzdqTzdlCmt5a1EvTmdBT0w3UDZrRHM5ejYrYk9mZ0YwbkdOK2JtZVdRcUplalIwdCtPeVFEQ3g1L0ZNdFVmRVZSNVFYODAKYWVlZmdwM0pGWmI2ZkF3OUtoTHRkUlYzRlAwdHo2aFMrZTRTZzBtd0FBT3FpalpzVjg3a1A1R1l6anRjZkExMgpsRFlsL25iMUd0VnZ2a1FENDlWblY3bURubDZtRzNMQ01OQ05INldsWk52M0FnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQkJqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCU0ZBM0x2Sk0yMWQ4cXMKWlZWQ2U2UnJUVDl3aVRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQW5zL0VKM3lLc2p0SVNvdGVRNzE0cjJVbQpCTVB5VVlUVGRSSEQ4TFpNZDNSeWt2c2FjRjJsMnk4OE56NndKY0F1b1VqMWg4YUJEUDVvWFZ0Tm1GVDlqeWJTClRYclJ2V2krYWVZZGI1NTZuRUE1L2E5NGUrY2IrQ2szcXkvMXhnUW9TNDU3QVpRT0Rpc0RaTkJZV2tBRnMyTGMKdWNwY0F0WEp0SXRoVm03RmpvQUhZY3NyWTA0eUFpWUVKTEQwMlRqVURYZzRpR09HTWtWSGRtaGF3QkRCRjNBagplc2ZjcUZ3amk2SnlBS0ZSQUNQb3d5a1FPTkZ3VVNvbTg5dVlFU1NDSkZ2TkNrOU1KbWpKMlB6RFV0NkN5cFI0CmVwRmRkMWZYTHd1d243ZnZQTW1KcUQzSHRMYWxYMUFabVBrK0JJOGV6ZkFpVmNWcW5USlFNWGxZUHBZZTlBPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQ==
    service:
      name: gardener-seed-admission-controller
      namespace: shoot--foo--bar
      path: /validate-extensions-gardener-cloud-v1alpha1-bastion
  failurePolicy: Fail
  matchPolicy: Exact
  name: validation.extensions.bastion.admission.core.gardener.cloud
  namespaceSelector: {}
  rules:
  - apiGroups:
    - extensions.gardener.cloud
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - bastions
  sideEffects: None
  timeoutSeconds: 10
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMrakNDQWVLZ0F3SUJBZ0lVVHAzWHZocldPVk04WkdlODZZb1hNVi9VSjdBd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0ZURVRNQkVHQTFVRUF4TUthM1ZpWlhKdVpYUmxjekFlRncweE9UQXlNamN4TlRNME1EQmFGdzB5TkRBeQpNall4TlRNME1EQmFNQlV4RXpBUkJnTlZCQU1UQ210MVltVnlibVYwWlhNd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDeWkwUUdPY3YyYlRmM044T0xOOTdSd3NnSDZRQXI4d1NwQU9ydHRCSmcKRm5mblUyVDFSSGd4bTdxZDE5MFdMOERDaHYwZFpmNzZkNmVTUTRacmpqeUFyVHp1ZmI0RHRQd2crVldxN1h2RgpCTnluKzJoZjRTeVNrd2Q2azdYTGhVVFJ4MDQ4SWJCeUM0ditGRXZtb0xBd3JjMGQwRzE0ZWM2c25EKzdqTzdlCmt5a1EvTmdBT0w3UDZrRHM5ejYrYk9mZ0YwbkdOK2JtZVdRcUplalIwdCtPeVFEQ3g1L0ZNdFVmRVZSNVFYODAKYWVlZmdwM0pGWmI2ZkF3OUtoTHRkUlYzRlAwdHo2aFMrZTRTZzBtd0FBT3FpalpzVjg3a1A1R1l6anRjZkExMgpsRFlsL25iMUd0VnZ2a1FENDlWblY3bURubDZtRzNMQ01OQ05INldsWk52M0FnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQkJqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCU0ZBM0x2Sk0yMWQ4cXMKWlZWQ2U2UnJUVDl3aVRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQW5zL0VKM3lLc2p0SVNvdGVRNzE0cjJVbQpCTVB5VVlUVGRSSEQ4TFpNZDNSeWt2c2FjRjJsMnk4OE56NndKY0F1b1VqMWg4YUJEUDVvWFZ0Tm1GVDlqeWJTClRYclJ2V2krYWVZZGI1NTZuRUE1L2E5NGUrY2IrQ2szcXkvMXhnUW9TNDU3QVpRT0Rpc0RaTkJZV2tBRnMyTGMKdWNwY0F0WEp0SXRoVm03RmpvQUhZY3NyWTA0eUFpWUVKTEQwMlRqVURYZzRpR09HTWtWSGRtaGF3QkRCRjNBagplc2ZjcUZ3amk2SnlBS0ZSQUNQb3d5a1FPTkZ3VVNvbTg5dVlFU1NDSkZ2TkNrOU1KbWpKMlB6RFV0NkN5cFI0CmVwRmRkMWZYTHd1d243ZnZQTW1KcUQzSHRMYWxYMUFabVBrK0JJOGV6ZkFpVmNWcW5USlFNWGxZUHBZZTlBPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQ==
    service:
      name: gardener-seed-admission-controller
      namespace: shoot--foo--bar
      path: /validate-extensions-gardener-cloud-v1alpha1-containerruntime
  failurePolicy: Fail
  matchPolicy: Exact
  name: validation.extensions.containerruntime.admission.core.gardener.cloud
  namespaceSelector: {}
  rules:
  - apiGroups:
    - extensions.gardener.cloud
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - containerruntimes
  sideEffects: None
  timeoutSeconds: 10
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMrakNDQWVLZ0F3SUJBZ0lVVHAzWHZocldPVk04WkdlODZZb1hNVi9VSjdBd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0ZURVRNQkVHQTFVRUF4TUthM1ZpWlhKdVpYUmxjekFlRncweE9UQXlNamN4TlRNME1EQmFGdzB5TkRBeQpNall4TlRNME1EQmFNQlV4RXpBUkJnTlZCQU1UQ210MVltVnlibVYwWlhNd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDeWkwUUdPY3YyYlRmM044T0xOOTdSd3NnSDZRQXI4d1NwQU9ydHRCSmcKRm5mblUyVDFSSGd4bTdxZDE5MFdMOERDaHYwZFpmNzZkNmVTUTRacmpqeUFyVHp1ZmI0RHRQd2crVldxN1h2RgpCTnluKzJoZjRTeVNrd2Q2azdYTGhVVFJ4MDQ4SWJCeUM0ditGRXZtb0xBd3JjMGQwRzE0ZWM2c25EKzdqTzdlCmt5a1EvTmdBT0w3UDZrRHM5ejYrYk9mZ0YwbkdOK2JtZVdRcUplalIwdCtPeVFEQ3g1L0ZNdFVmRVZSNVFYODAKYWVlZmdwM0pGWmI2ZkF3OUtoTHRkUlYzRlAwdHo2aFMrZTRTZzBtd0FBT3FpalpzVjg3a1A1R1l6anRjZkExMgpsRFlsL25iMUd0VnZ2a1FENDlWblY3bURubDZtRzNMQ01OQ05INldsWk52M0FnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQkJqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCU0ZBM0x2Sk0yMWQ4cXMKWlZWQ2U2UnJUVDl3aVRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQW5zL0VKM3lLc2p0SVNvdGVRNzE0cjJVbQpCTVB5VVlUVGRSSEQ4TFpNZDNSeWt2c2FjRjJsMnk4OE56NndKY0F1b1VqMWg4YUJEUDVvWFZ0Tm1GVDlqeWJTClRYclJ2V2krYWVZZGI1NTZuRUE1L2E5NGUrY2IrQ2szcXkvMXhnUW9TNDU3QVpRT0Rpc0RaTkJZV2tBRnMyTGMKdWNwY0F0WEp0SXRoVm03RmpvQUhZY3NyWTA0eUFpWUVKTEQwMlRqVURYZzRpR09HTWtWSGRtaGF3QkRCRjNBagplc2ZjcUZ3amk2SnlBS0ZSQUNQb3d5a1FPTkZ3VVNvbTg5dVlFU1NDSkZ2TkNrOU1KbWpKMlB6RFV0NkN5cFI0CmVwRmRkMWZYTHd1d243ZnZQTW1KcUQzSHRMYWxYMUFabVBrK0JJOGV6ZkFpVmNWcW5USlFNWGxZUHBZZTlBPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQ==
    service:
      name: gardener-seed-admission-controller
      namespace: shoot--foo--bar
      path: /validate-extensions-gardener-cloud-v1alpha1-controlplane
  failurePolicy: Fail
  matchPolicy: Exact
  name: validation.extensions.controlplane.admission.core.gardener.cloud
  namespaceSelector: {}
  rules:
  - apiGroups:
    - extensions.gardener.cloud
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - controlplanes
  sideEffects: None
  timeoutSeconds: 10
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMrakNDQWVLZ0F3SUJBZ0lVVHAzWHZocldPVk04WkdlODZZb1hNVi9VSjdBd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0ZURVRNQkVHQTFVRUF4TUthM1ZpWlhKdVpYUmxjekFlRncweE9UQXlNamN4TlRNME1EQmFGdzB5TkRBeQpNall4TlRNME1EQmFNQlV4RXpBUkJnTlZCQU1UQ210MVltVnlibVYwWlhNd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDeWkwUUdPY3YyYlRmM044T0xOOTdSd3NnSDZRQXI4d1NwQU9ydHRCSmcKRm5mblUyVDFSSGd4bTdxZDE5MFdMOERDaHYwZFpmNzZkNmVTUTRacmpqeUFyVHp1ZmI0RHRQd2crVldxN1h2RgpCTnluKzJoZjRTeVNrd2Q2azdYTGhVVFJ4MDQ4SWJCeUM0ditGRXZtb0xBd3JjMGQwRzE0ZWM2c25EKzdqTzdlCmt5a1EvTmdBT0w3UDZrRHM5ejYrYk9mZ0YwbkdOK2JtZVdRcUplalIwdCtPeVFEQ3g1L0ZNdFVmRVZSNVFYODAKYWVlZmdwM0pGWmI2ZkF3OUtoTHRkUlYzRlAwdHo2aFMrZTRTZzBtd0FBT3FpalpzVjg3a1A1R1l6anRjZkExMgpsRFlsL25iMUd0VnZ2a1FENDlWblY3bURubDZtRzNMQ01OQ05INldsWk52M0FnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQkJqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCU0ZBM0x2Sk0yMWQ4cXMKWlZWQ2U2UnJUVDl3aVRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQW5zL0VKM3lLc2p0SVNvdGVRNzE0cjJVbQpCTVB5VVlUVGRSSEQ4TFpNZDNSeWt2c2FjRjJsMnk4OE56NndKY0F1b1VqMWg4YUJEUDVvWFZ0Tm1GVDlqeWJTClRYclJ2V2krYWVZZGI1NTZuRUE1L2E5NGUrY2IrQ2szcXkvMXhnUW9TNDU3QVpRT0Rpc0RaTkJZV2tBRnMyTGMKdWNwY0F0WEp0SXRoVm03RmpvQUhZY3NyWTA0eUFpWUVKTEQwMlRqVURYZzRpR09HTWtWSGRtaGF3QkRCRjNBagplc2ZjcUZ3amk2SnlBS0ZSQUNQb3d5a1FPTkZ3VVNvbTg5dVlFU1NDSkZ2TkNrOU1KbWpKMlB6RFV0NkN5cFI0CmVwRmRkMWZYTHd1d243ZnZQTW1KcUQzSHRMYWxYMUFabVBrK0JJOGV6ZkFpVmNWcW5USlFNWGxZUHBZZTlBPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQ==
    service:
      name: gardener-seed-admission-controller
      namespace: shoot--foo--bar
      path: /validate-extensions-gardener-cloud-v1alpha1-dnsrecord
  failurePolicy: Fail
  matchPolicy: Exact
  name: validation.extensions.dnsrecord.admission.core.gardener.cloud
  namespaceSelector: {}
  rules:
  - apiGroups:
    - extensions.gardener.cloud
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - dnsrecords
  sideEffects: None
  timeoutSeconds: 10
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMrakNDQWVLZ0F3SUJBZ0lVVHAzWHZocldPVk04WkdlODZZb1hNVi9VSjdBd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0ZURVRNQkVHQTFVRUF4TUthM1ZpWlhKdVpYUmxjekFlRncweE9UQXlNamN4TlRNME1EQmFGdzB5TkRBeQpNall4TlRNME1EQmFNQlV4RXpBUkJnTlZCQU1UQ210MVltVnlibVYwWlhNd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDeWkwUUdPY3YyYlRmM044T0xOOTdSd3NnSDZRQXI4d1NwQU9ydHRCSmcKRm5mblUyVDFSSGd4bTdxZDE5MFdMOERDaHYwZFpmNzZkNmVTUTRacmpqeUFyVHp1ZmI0RHRQd2crVldxN1h2RgpCTnluKzJoZjRTeVNrd2Q2azdYTGhVVFJ4MDQ4SWJCeUM0ditGRXZtb0xBd3JjMGQwRzE0ZWM2c25EKzdqTzdlCmt5a1EvTmdBT0w3UDZrRHM5ejYrYk9mZ0YwbkdOK2JtZVdRcUplalIwdCtPeVFEQ3g1L0ZNdFVmRVZSNVFYODAKYWVlZmdwM0pGWmI2ZkF3OUtoTHRkUlYzRlAwdHo2aFMrZTRTZzBtd0FBT3FpalpzVjg3a1A1R1l6anRjZkExMgpsRFlsL25iMUd0VnZ2a1FENDlWblY3bURubDZtRzNMQ01OQ05INldsWk52M0FnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQkJqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCU0ZBM0x2Sk0yMWQ4cXMKWlZWQ2U2UnJUVDl3aVRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQW5zL0VKM3lLc2p0SVNvdGVRNzE0cjJVbQpCTVB5VVlUVGRSSEQ4TFpNZDNSeWt2c2FjRjJsMnk4OE56NndKY0F1b1VqMWg4YUJEUDVvWFZ0Tm1GVDlqeWJTClRYclJ2V2krYWVZZGI1NTZuRUE1L2E5NGUrY2IrQ2szcXkvMXhnUW9TNDU3QVpRT0Rpc0RaTkJZV2tBRnMyTGMKdWNwY0F0WEp0SXRoVm03RmpvQUhZY3NyWTA0eUFpWUVKTEQwMlRqVURYZzRpR09HTWtWSGRtaGF3QkRCRjNBagplc2ZjcUZ3amk2SnlBS0ZSQUNQb3d5a1FPTkZ3VVNvbTg5dVlFU1NDSkZ2TkNrOU1KbWpKMlB6RFV0NkN5cFI0CmVwRmRkMWZYTHd1d243ZnZQTW1KcUQzSHRMYWxYMUFabVBrK0JJOGV6ZkFpVmNWcW5USlFNWGxZUHBZZTlBPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQ==
    service:
      name: gardener-seed-admission-controller
      namespace: shoot--foo--bar
      path: /validate-druid-gardener-cloud-v1alpha1-etcd
  failurePolicy: Fail
  matchPolicy: Exact
  name: validation.extensions.etcd.admission.core.gardener.cloud
  namespaceSelector: {}
  rules:
  - apiGroups:
    - druid.gardener.cloud
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - etcds
  sideEffects: None
  timeoutSeconds: 10
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMrakNDQWVLZ0F3SUJBZ0lVVHAzWHZocldPVk04WkdlODZZb1hNVi9VSjdBd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0ZURVRNQkVHQTFVRUF4TUthM1ZpWlhKdVpYUmxjekFlRncweE9UQXlNamN4TlRNME1EQmFGdzB5TkRBeQpNall4TlRNME1EQmFNQlV4RXpBUkJnTlZCQU1UQ210MVltVnlibVYwWlhNd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDeWkwUUdPY3YyYlRmM044T0xOOTdSd3NnSDZRQXI4d1NwQU9ydHRCSmcKRm5mblUyVDFSSGd4bTdxZDE5MFdMOERDaHYwZFpmNzZkNmVTUTRacmpqeUFyVHp1ZmI0RHRQd2crVldxN1h2RgpCTnluKzJoZjRTeVNrd2Q2azdYTGhVVFJ4MDQ4SWJCeUM0ditGRXZtb0xBd3JjMGQwRzE0ZWM2c25EKzdqTzdlCmt5a1EvTmdBT0w3UDZrRHM5ejYrYk9mZ0YwbkdOK2JtZVdRcUplalIwdCtPeVFEQ3g1L0ZNdFVmRVZSNVFYODAKYWVlZmdwM0pGWmI2ZkF3OUtoTHRkUlYzRlAwdHo2aFMrZTRTZzBtd0FBT3FpalpzVjg3a1A1R1l6anRjZkExMgpsRFlsL25iMUd0VnZ2a1FENDlWblY3bURubDZtRzNMQ01OQ05INldsWk52M0FnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQkJqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCU0ZBM0x2Sk0yMWQ4cXMKWlZWQ2U2UnJUVDl3aVRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQW5zL0VKM3lLc2p0SVNvdGVRNzE0cjJVbQpCTVB5VVlUVGRSSEQ4TFpNZDNSeWt2c2FjRjJsMnk4OE56NndKY0F1b1VqMWg4YUJEUDVvWFZ0Tm1GVDlqeWJTClRYclJ2V2krYWVZZGI1NTZuRUE1L2E5NGUrY2IrQ2szcXkvMXhnUW9TNDU3QVpRT0Rpc0RaTkJZV2tBRnMyTGMKdWNwY0F0WEp0SXRoVm03RmpvQUhZY3NyWTA0eUFpWUVKTEQwMlRqVURYZzRpR09HTWtWSGRtaGF3QkRCRjNBagplc2ZjcUZ3amk2SnlBS0ZSQUNQb3d5a1FPTkZ3VVNvbTg5dVlFU1NDSkZ2TkNrOU1KbWpKMlB6RFV0NkN5cFI0CmVwRmRkMWZYTHd1d243ZnZQTW1KcUQzSHRMYWxYMUFabVBrK0JJOGV6ZkFpVmNWcW5USlFNWGxZUHBZZTlBPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQ==
    service:
      name: gardener-seed-admission-controller
      namespace: shoot--foo--bar
      path: /validate-extensions-gardener-cloud-v1alpha1-extension
  failurePolicy: Fail
  matchPolicy: Exact
  name: validation.extensions.extension.admission.core.gardener.cloud
  namespaceSelector: {}
  rules:
  - apiGroups:
    - extensions.gardener.cloud
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - extensions
  sideEffects: None
  timeoutSeconds: 10
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMrakNDQWVLZ0F3SUJBZ0lVVHAzWHZocldPVk04WkdlODZZb1hNVi9VSjdBd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0ZURVRNQkVHQTFVRUF4TUthM1ZpWlhKdVpYUmxjekFlRncweE9UQXlNamN4TlRNME1EQmFGdzB5TkRBeQpNall4TlRNME1EQmFNQlV4RXpBUkJnTlZCQU1UQ210MVltVnlibVYwWlhNd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDeWkwUUdPY3YyYlRmM044T0xOOTdSd3NnSDZRQXI4d1NwQU9ydHRCSmcKRm5mblUyVDFSSGd4bTdxZDE5MFdMOERDaHYwZFpmNzZkNmVTUTRacmpqeUFyVHp1ZmI0RHRQd2crVldxN1h2RgpCTnluKzJoZjRTeVNrd2Q2azdYTGhVVFJ4MDQ4SWJCeUM0ditGRXZtb0xBd3JjMGQwRzE0ZWM2c25EKzdqTzdlCmt5a1EvTmdBT0w3UDZrRHM5ejYrYk9mZ0YwbkdOK2JtZVdRcUplalIwdCtPeVFEQ3g1L0ZNdFVmRVZSNVFYODAKYWVlZmdwM0pGWmI2ZkF3OUtoTHRkUlYzRlAwdHo2aFMrZTRTZzBtd0FBT3FpalpzVjg3a1A1R1l6anRjZkExMgpsRFlsL25iMUd0VnZ2a1FENDlWblY3bURubDZtRzNMQ01OQ05INldsWk52M0FnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQkJqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCU0ZBM0x2Sk0yMWQ4cXMKWlZWQ2U2UnJUVDl3aVRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQW5zL0VKM3lLc2p0SVNvdGVRNzE0cjJVbQpCTVB5VVlUVGRSSEQ4TFpNZDNSeWt2c2FjRjJsMnk4OE56NndKY0F1b1VqMWg4YUJEUDVvWFZ0Tm1GVDlqeWJTClRYclJ2V2krYWVZZGI1NTZuRUE1L2E5NGUrY2IrQ2szcXkvMXhnUW9TNDU3QVpRT0Rpc0RaTkJZV2tBRnMyTGMKdWNwY0F0WEp0SXRoVm03RmpvQUhZY3NyWTA0eUFpWUVKTEQwMlRqVURYZzRpR09HTWtWSGRtaGF3QkRCRjNBagplc2ZjcUZ3amk2SnlBS0ZSQUNQb3d5a1FPTkZ3VVNvbTg5dVlFU1NDSkZ2TkNrOU1KbWpKMlB6RFV0NkN5cFI0CmVwRmRkMWZYTHd1d243ZnZQTW1KcUQzSHRMYWxYMUFabVBrK0JJOGV6ZkFpVmNWcW5USlFNWGxZUHBZZTlBPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQ==
    service:
      name: gardener-seed-admission-controller
      namespace: shoot--foo--bar
      path: /validate-extensions-gardener-cloud-v1alpha1-infrastructure
  failurePolicy: Fail
  matchPolicy: Exact
  name: validation.extensions.infrastructure.admission.core.gardener.cloud
  namespaceSelector: {}
  rules:
  - apiGroups:
    - extensions.gardener.cloud
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - infrastructures
  sideEffects: None
  timeoutSeconds: 10
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMrakNDQWVLZ0F3SUJBZ0lVVHAzWHZocldPVk04WkdlODZZb1hNVi9VSjdBd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0ZURVRNQkVHQTFVRUF4TUthM1ZpWlhKdVpYUmxjekFlRncweE9UQXlNamN4TlRNME1EQmFGdzB5TkRBeQpNall4TlRNME1EQmFNQlV4RXpBUkJnTlZCQU1UQ210MVltVnlibVYwWlhNd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDeWkwUUdPY3YyYlRmM044T0xOOTdSd3NnSDZRQXI4d1NwQU9ydHRCSmcKRm5mblUyVDFSSGd4bTdxZDE5MFdMOERDaHYwZFpmNzZkNmVTUTRacmpqeUFyVHp1ZmI0RHRQd2crVldxN1h2RgpCTnluKzJoZjRTeVNrd2Q2azdYTGhVVFJ4MDQ4SWJCeUM0ditGRXZtb0xBd3JjMGQwRzE0ZWM2c25EKzdqTzdlCmt5a1EvTmdBT0w3UDZrRHM5ejYrYk9mZ0YwbkdOK2JtZVdRcUplalIwdCtPeVFEQ3g1L0ZNdFVmRVZSNVFYODAKYWVlZmdwM0pGWmI2ZkF3OUtoTHRkUlYzRlAwdHo2aFMrZTRTZzBtd0FBT3FpalpzVjg3a1A1R1l6anRjZkExMgpsRFlsL25iMUd0VnZ2a1FENDlWblY3bURubDZtRzNMQ01OQ05INldsWk52M0FnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQkJqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCU0ZBM0x2Sk0yMWQ4cXMKWlZWQ2U2UnJUVDl3aVRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQW5zL0VKM3lLc2p0SVNvdGVRNzE0cjJVbQpCTVB5VVlUVGRSSEQ4TFpNZDNSeWt2c2FjRjJsMnk4OE56NndKY0F1b1VqMWg4YUJEUDVvWFZ0Tm1GVDlqeWJTClRYclJ2V2krYWVZZGI1NTZuRUE1L2E5NGUrY2IrQ2szcXkvMXhnUW9TNDU3QVpRT0Rpc0RaTkJZV2tBRnMyTGMKdWNwY0F0WEp0SXRoVm03RmpvQUhZY3NyWTA0eUFpWUVKTEQwMlRqVURYZzRpR09HTWtWSGRtaGF3QkRCRjNBagplc2ZjcUZ3amk2SnlBS0ZSQUNQb3d5a1FPTkZ3VVNvbTg5dVlFU1NDSkZ2TkNrOU1KbWpKMlB6RFV0NkN5cFI0CmVwRmRkMWZYTHd1d243ZnZQTW1KcUQzSHRMYWxYMUFabVBrK0JJOGV6ZkFpVmNWcW5USlFNWGxZUHBZZTlBPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQ==
    service:
      name: gardener-seed-admission-controller
      namespace: shoot--foo--bar
      path: /validate-extensions-gardener-cloud-v1alpha1-network
  failurePolicy: Fail
  matchPolicy: Exact
  name: validation.extensions.network.admission.core.gardener.cloud
  namespaceSelector: {}
  rules:
  - apiGroups:
    - extensions.gardener.cloud
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - networks
  sideEffects: None
  timeoutSeconds: 10
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMrakNDQWVLZ0F3SUJBZ0lVVHAzWHZocldPVk04WkdlODZZb1hNVi9VSjdBd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0ZURVRNQkVHQTFVRUF4TUthM1ZpWlhKdVpYUmxjekFlRncweE9UQXlNamN4TlRNME1EQmFGdzB5TkRBeQpNall4TlRNME1EQmFNQlV4RXpBUkJnTlZCQU1UQ210MVltVnlibVYwWlhNd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDeWkwUUdPY3YyYlRmM044T0xOOTdSd3NnSDZRQXI4d1NwQU9ydHRCSmcKRm5mblUyVDFSSGd4bTdxZDE5MFdMOERDaHYwZFpmNzZkNmVTUTRacmpqeUFyVHp1ZmI0RHRQd2crVldxN1h2RgpCTnluKzJoZjRTeVNrd2Q2azdYTGhVVFJ4MDQ4SWJCeUM0ditGRXZtb0xBd3JjMGQwRzE0ZWM2c25EKzdqTzdlCmt5a1EvTmdBT0w3UDZrRHM5ejYrYk9mZ0YwbkdOK2JtZVdRcUplalIwdCtPeVFEQ3g1L0ZNdFVmRVZSNVFYODAKYWVlZmdwM0pGWmI2ZkF3OUtoTHRkUlYzRlAwdHo2aFMrZTRTZzBtd0FBT3FpalpzVjg3a1A1R1l6anRjZkExMgpsRFlsL25iMUd0VnZ2a1FENDlWblY3bURubDZtRzNMQ01OQ05INldsWk52M0FnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQkJqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCU0ZBM0x2Sk0yMWQ4cXMKWlZWQ2U2UnJUVDl3aVRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQW5zL0VKM3lLc2p0SVNvdGVRNzE0cjJVbQpCTVB5VVlUVGRSSEQ4TFpNZDNSeWt2c2FjRjJsMnk4OE56NndKY0F1b1VqMWg4YUJEUDVvWFZ0Tm1GVDlqeWJTClRYclJ2V2krYWVZZGI1NTZuRUE1L2E5NGUrY2IrQ2szcXkvMXhnUW9TNDU3QVpRT0Rpc0RaTkJZV2tBRnMyTGMKdWNwY0F0WEp0SXRoVm03RmpvQUhZY3NyWTA0eUFpWUVKTEQwMlRqVURYZzRpR09HTWtWSGRtaGF3QkRCRjNBagplc2ZjcUZ3amk2SnlBS0ZSQUNQb3d5a1FPTkZ3VVNvbTg5dVlFU1NDSkZ2TkNrOU1KbWpKMlB6RFV0NkN5cFI0CmVwRmRkMWZYTHd1d243ZnZQTW1KcUQzSHRMYWxYMUFabVBrK0JJOGV6ZkFpVmNWcW5USlFNWGxZUHBZZTlBPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQ==
    service:
      name: gardener-seed-admission-controller
      namespace: shoot--foo--bar
      path: /validate-extensions-gardener-cloud-v1alpha1-operatingsystemconfig
  failurePolicy: Fail
  matchPolicy: Exact
  name: validation.extensions.operatingsystemconfig.admission.core.gardener.cloud
  namespaceSelector: {}
  rules:
  - apiGroups:
    - extensions.gardener.cloud
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - operatingsystemconfigs
  sideEffects: None
  timeoutSeconds: 10
- admissionReviewVersions:
  - v1beta1
  - v1
  clientConfig:
    caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMrakNDQWVLZ0F3SUJBZ0lVVHAzWHZocldPVk04WkdlODZZb1hNVi9VSjdBd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0ZURVRNQkVHQTFVRUF4TUthM1ZpWlhKdVpYUmxjekFlRncweE9UQXlNamN4TlRNME1EQmFGdzB5TkRBeQpNall4TlRNME1EQmFNQlV4RXpBUkJnTlZCQU1UQ210MVltVnlibVYwWlhNd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDeWkwUUdPY3YyYlRmM044T0xOOTdSd3NnSDZRQXI4d1NwQU9ydHRCSmcKRm5mblUyVDFSSGd4bTdxZDE5MFdMOERDaHYwZFpmNzZkNmVTUTRacmpqeUFyVHp1ZmI0RHRQd2crVldxN1h2RgpCTnluKzJoZjRTeVNrd2Q2azdYTGhVVFJ4MDQ4SWJCeUM0ditGRXZtb0xBd3JjMGQwRzE0ZWM2c25EKzdqTzdlCmt5a1EvTmdBT0w3UDZrRHM5ejYrYk9mZ0YwbkdOK2JtZVdRcUplalIwdCtPeVFEQ3g1L0ZNdFVmRVZSNVFYODAKYWVlZmdwM0pGWmI2ZkF3OUtoTHRkUlYzRlAwdHo2aFMrZTRTZzBtd0FBT3FpalpzVjg3a1A1R1l6anRjZkExMgpsRFlsL25iMUd0VnZ2a1FENDlWblY3bURubDZtRzNMQ01OQ05INldsWk52M0FnTUJBQUdqUWpCQU1BNEdBMVVkCkR3RUIvd1FFQXdJQkJqQVBCZ05WSFJNQkFmOEVCVEFEQVFIL01CMEdBMVVkRGdRV0JCU0ZBM0x2Sk0yMWQ4cXMKWlZWQ2U2UnJUVDl3aVRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQW5zL0VKM3lLc2p0SVNvdGVRNzE0cjJVbQpCTVB5VVlUVGRSSEQ4TFpNZDNSeWt2c2FjRjJsMnk4OE56NndKY0F1b1VqMWg4YUJEUDVvWFZ0Tm1GVDlqeWJTClRYclJ2V2krYWVZZGI1NTZuRUE1L2E5NGUrY2IrQ2szcXkvMXhnUW9TNDU3QVpRT0Rpc0RaTkJZV2tBRnMyTGMKdWNwY0F0WEp0SXRoVm03RmpvQUhZY3NyWTA0eUFpWUVKTEQwMlRqVURYZzRpR09HTWtWSGRtaGF3QkRCRjNBagplc2ZjcUZ3amk2SnlBS0ZSQUNQb3d5a1FPTkZ3VVNvbTg5dVlFU1NDSkZ2TkNrOU1KbWpKMlB6RFV0NkN5cFI0CmVwRmRkMWZYTHd1d243ZnZQTW1KcUQzSHRMYWxYMUFabVBrK0JJOGV6ZkFpVmNWcW5USlFNWGxZUHBZZTlBPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQ==
    service:
      name: gardener-seed-admission-controller
      namespace: shoot--foo--bar
      path: /validate-extensions-gardener-cloud-v1alpha1-worker
  failurePolicy: Fail
  matchPolicy: Exact
  name: validation.extensions.worker.admission.core.gardener.cloud
  namespaceSelector: {}
  rules:
  - apiGroups:
    - extensions.gardener.cloud
    apiVersions:
    - v1alpha1
    operations:
    - CREATE
    - UPDATE
    resources:
    - workers
  sideEffects: None
  timeoutSeconds: 10
`

		vpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  labels:
    app: gardener
    role: seed-admission-controller
  name: gardener-seed-admission-controller-vpa
  namespace: shoot--foo--bar
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: gardener-seed-admission-controller
  updatePolicy:
    updateMode: Auto
status: {}
`

		managedResourceName       = "gardener-seed-admission-controller"
		managedResourceSecretName = "managedresource-gardener-seed-admission-controller"

		managedResourceSecret *corev1.Secret
		managedResource       *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)

		seedAdmission = New(c, namespace, sm, image)

		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceSecretName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"clusterrole____gardener-seed-admission-controller.yaml":                              []byte(clusterRoleYAML),
				"clusterrolebinding____gardener-seed-admission-controller.yaml":                       []byte(clusterRoleBindingYAML),
				"deployment__shoot--foo--bar__gardener-seed-admission-controller.yaml":                []byte(deploymentYAML),
				"poddisruptionbudget__shoot--foo--bar__gardener-seed-admission-controller.yaml":       []byte(pdbYAML),
				"service__shoot--foo--bar__gardener-seed-admission-controller.yaml":                   []byte(serviceYAML),
				"serviceaccount__shoot--foo--bar__gardener-seed-admission-controller.yaml":            []byte(serviceAccountYAML),
				"validatingwebhookconfiguration____gardener-seed-admission-controller.yaml":           []byte(validatingWebhookConfigurationYAML),
				"verticalpodautoscaler__shoot--foo--bar__gardener-seed-admission-controller-vpa.yaml": []byte(vpaYAML),
			},
		}
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs: []corev1.LocalObjectReference{
					{Name: managedResourceSecretName},
				},
				KeepObjects: pointer.Bool(false),
				Class:       pointer.String("seed"),
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		It("should fail because the managed resource secret cannot be updated", func() {
			gomock.InOrder(
				c.EXPECT().List(ctx, gomock.Any(), client.Limit(3)).DoAndReturn(
					func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						Expect(list).To(BeAssignableToTypeOf(&metav1.PartialObjectMetadataList{}))
						list.(*metav1.PartialObjectMetadataList).Items = make([]metav1.PartialObjectMetadata, 3)
						return nil
					}),
				c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
			)

			Expect(seedAdmission.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the managed resource cannot be updated", func() {
			gomock.InOrder(
				c.EXPECT().List(ctx, gomock.Any(), client.Limit(3)).DoAndReturn(
					func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						Expect(list).To(BeAssignableToTypeOf(&metav1.PartialObjectMetadataList{}))
						list.(*metav1.PartialObjectMetadataList).Items = make([]metav1.PartialObjectMetadata, 3)
						return nil
					}),
				c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr),
			)

			Expect(seedAdmission.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully deploy all resources", func() {
			gomock.InOrder(
				c.EXPECT().List(ctx, gomock.Any(), client.Limit(3)).DoAndReturn(
					func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						Expect(list).To(BeAssignableToTypeOf(&metav1.PartialObjectMetadataList{}))
						list.(*metav1.PartialObjectMetadataList).Items = make([]metav1.PartialObjectMetadata, 3)
						return nil
					}),
				c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
					Expect(obj).To(DeepEqual(managedResourceSecret))
				}),
				c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
					Expect(obj).To(DeepEqual(managedResource))
				}),
			)

			Expect(seedAdmission.Deploy(ctx)).To(Succeed())
		})

		It("should reduce replicas for seed clusters smaller than three nodes", func() {
			managedResourceSecret.Data["deployment__shoot--foo--bar__gardener-seed-admission-controller.yaml"] = []byte(strings.Replace(deploymentYAML, "replicas: 3", "replicas: 1", -1))

			gomock.InOrder(
				c.EXPECT().List(ctx, gomock.Any(), client.Limit(3)).DoAndReturn(
					func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
						Expect(list).To(BeAssignableToTypeOf(&metav1.PartialObjectMetadataList{}))
						list.(*metav1.PartialObjectMetadataList).Items = make([]metav1.PartialObjectMetadata, 1)
						return nil
					}),
				c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
					Expect(obj).To(DeepEqual(managedResourceSecret))
				}),
				c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
					Expect(obj).To(DeepEqual(managedResource))
				}),
			)

			Expect(seedAdmission.Deploy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		It("should fail because it cannot be checked if the managed resource became healthy", func() {
			oldTimeout := TimeoutWaitForManagedResource
			defer func() { TimeoutWaitForManagedResource = oldTimeout }()
			TimeoutWaitForManagedResource = time.Millisecond

			c.EXPECT().Get(gomock.Any(), kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr)

			Expect(seedAdmission.Wait(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the managed resource doesn't become healthy", func() {
			oldTimeout := TimeoutWaitForManagedResource
			defer func() { TimeoutWaitForManagedResource = oldTimeout }()
			TimeoutWaitForManagedResource = time.Millisecond

			c.EXPECT().Get(gomock.Any(), kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(
				func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
					(&resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
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
					}).DeepCopyInto(obj.(*resourcesv1alpha1.ManagedResource))
					return nil
				},
			).AnyTimes()

			Expect(seedAdmission.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
		})

		It("should successfully wait for all resources to be ready", func() {
			c.EXPECT().Get(gomock.Any(), kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(
				func(ctx context.Context, _ client.ObjectKey, obj client.Object) error {
					(&resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
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
					}).DeepCopyInto(obj.(*resourcesv1alpha1.ManagedResource))
					return nil
				},
			)

			Expect(seedAdmission.Wait(ctx)).To(Succeed())
		})
	})

	Context("cleanup", func() {
		var (
			secret          = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceName,
					Namespace: namespace,
				},
			}
		)

		Describe("#Destroy", func() {
			It("should fail when the managed resource deletion fails", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, managedResource).Return(fakeErr),
				)

				Expect(seedAdmission.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the secret deletion fails", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, managedResource),
					c.EXPECT().Delete(ctx, secret).Return(fakeErr),
				)

				Expect(seedAdmission.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully delete all resources", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, managedResource),
					c.EXPECT().Delete(ctx, secret),
				)

				Expect(seedAdmission.Destroy(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion fails", func() {
				c.EXPECT().Get(gomock.Any(), kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr)

				Expect(seedAdmission.WaitCleanup(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the wait for the managed resource deletion times out", func() {
				oldTimeout := TimeoutWaitForManagedResource
				defer func() { TimeoutWaitForManagedResource = oldTimeout }()
				TimeoutWaitForManagedResource = time.Millisecond

				c.EXPECT().Get(gomock.Any(), kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).AnyTimes()

				Expect(seedAdmission.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should successfully wait for all resources to be cleaned up", func() {
				c.EXPECT().Get(gomock.Any(), kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

				Expect(seedAdmission.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
