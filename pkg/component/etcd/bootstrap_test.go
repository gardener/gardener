// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package etcd_test

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver"
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Etcd", func() {
	var (
		ctrl         *gomock.Controller
		c            *mockclient.MockClient
		bootstrapper component.DeployWaiter
		etcdConfig   *config.ETCDConfig

		ctx                       = context.TODO()
		fakeErr                   = fmt.Errorf("fake error")
		namespace                 = "shoot--foo--bar"
		kubernetesVersion         = semver.MustParse("1.25.0")
		etcdDruidImage            = "etcd/druid:1.2.3"
		imageVectorOverwriteEmpty *string
		imageVectorOverwriteFull  = pointer.String("some overwrite")
		priorityClassName         = "some-priority-class"

		managedResourceName       = "etcd-druid"
		managedResourceSecretName = "managedresource-" + managedResourceName
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		etcdConfig = &config.ETCDConfig{
			ETCDController: &config.ETCDController{
				Workers: pointer.Int64(25),
			},
			CustodianController: &config.CustodianController{
				Workers: pointer.Int64(3),
			},
			BackupCompactionController: &config.BackupCompactionController{
				Workers:                pointer.Int64(3),
				EnableBackupCompaction: pointer.Bool(true),
				EventsThreshold:        pointer.Int64(1000000),
				ActiveDeadlineDuration: &metav1.Duration{Duration: time.Hour * 3},
			},
		}

		bootstrapper = NewBootstrapper(c, namespace, kubernetesVersion, etcdConfig, etcdDruidImage, imageVectorOverwriteEmpty, priorityClassName)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		var (
			configMapName = "etcd-druid-imagevector-overwrite-4475dd36"

			serviceAccountYAML = `apiVersion: v1
automountServiceAccountToken: false
kind: ServiceAccount
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
  name: etcd-druid
  namespace: ` + namespace + `
`
			clusterRoleYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
  name: gardener.cloud:system:etcd-druid
rules:
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
  - list
  - watch
  - delete
  - deletecollection
- apiGroups:
  - ""
  resources:
  - secrets
  - endpoints
  verbs:
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - get
  - list
  - watch
  - patch
  - update
- apiGroups:
  - ""
  resources:
  - serviceaccounts
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - rbac.authorization.k8s.io
  resources:
  - roles
  - rolebindings
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - ""
  resources:
  - services
  - configmaps
  verbs:
  - get
  - list
  - patch
  - update
  - watch
  - create
  - delete
- apiGroups:
  - apps
  resources:
  - statefulsets
  verbs:
  - get
  - list
  - patch
  - update
  - watch
  - create
  - delete
- apiGroups:
  - batch
  resources:
  - jobs
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - druid.gardener.cloud
  resources:
  - etcds
  - etcdcopybackupstasks
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - druid.gardener.cloud
  resources:
  - etcds/status
  - etcds/finalizers
  - etcdcopybackupstasks/status
  - etcdcopybackupstasks/finalizers
  verbs:
  - get
  - update
  - patch
  - create
- apiGroups:
  - coordination.k8s.io
  resources:
  - leases
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
  - deletecollection
- apiGroups:
  - ""
  resources:
  - persistentvolumeclaims
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - policy
  resources:
  - poddisruptionbudgets
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
`
			clusterRoleBindingYAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
  name: gardener.cloud:system:etcd-druid
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gardener.cloud:system:etcd-druid
subjects:
- kind: ServiceAccount
  name: etcd-druid
  namespace: ` + namespace + `
`
			vpaYAML = `apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
  name: etcd-druid-vpa
  namespace: ` + namespace + `
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      minAllowed:
        memory: 100M
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: etcd-druid
  updatePolicy:
    updateMode: Auto
status: {}
`
			configMapImageVectorOverwriteYAML = `apiVersion: v1
data:
  images_overwrite.yaml: ` + *imageVectorOverwriteFull + `
immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
    resources.gardener.cloud/garbage-collectable-reference: "true"
  name: ` + configMapName + `
  namespace: ` + namespace + `
`

			deploymentWithoutImageVectorOverwriteYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
    high-availability-config.resources.gardener.cloud/type: controller
  name: etcd-druid
  namespace: ` + namespace + `
spec:
  replicas: 1
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      gardener.cloud/role: etcd-druid
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        gardener.cloud/role: etcd-druid
        networking.gardener.cloud/to-dns: allowed
        networking.gardener.cloud/to-runtime-apiserver: allowed
    spec:
      containers:
      - command:
        - /etcd-druid
        - --enable-leader-election=true
        - --ignore-operation-annotation=false
        - --disable-etcd-serviceaccount-automount=true
        - --workers=25
        - --custodian-workers=3
        - --compaction-workers=3
        - --enable-backup-compaction=true
        - --etcd-events-threshold=1000000
        - --active-deadline-duration=3h0m0s
        image: ` + etcdDruidImage + `
        imagePullPolicy: IfNotPresent
        name: etcd-druid
        ports:
        - containerPort: 8080
        resources:
          limits:
            memory: 512Mi
          requests:
            cpu: 50m
            memory: 128Mi
      priorityClassName: ` + priorityClassName + `
      serviceAccountName: etcd-druid
status: {}
`
			deploymentWithImageVectorOverwriteYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  annotations:
    ` + references.AnnotationKey(references.KindConfigMap, configMapName) + `: ` + configMapName + `
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
    high-availability-config.resources.gardener.cloud/type: controller
  name: etcd-druid
  namespace: ` + namespace + `
spec:
  replicas: 1
  revisionHistoryLimit: 1
  selector:
    matchLabels:
      gardener.cloud/role: etcd-druid
  strategy: {}
  template:
    metadata:
      annotations:
        ` + references.AnnotationKey(references.KindConfigMap, configMapName) + `: ` + configMapName + `
      creationTimestamp: null
      labels:
        gardener.cloud/role: etcd-druid
        networking.gardener.cloud/to-dns: allowed
        networking.gardener.cloud/to-runtime-apiserver: allowed
    spec:
      containers:
      - command:
        - /etcd-druid
        - --enable-leader-election=true
        - --ignore-operation-annotation=false
        - --disable-etcd-serviceaccount-automount=true
        - --workers=25
        - --custodian-workers=3
        - --compaction-workers=3
        - --enable-backup-compaction=true
        - --etcd-events-threshold=1000000
        - --active-deadline-duration=3h0m0s
        env:
        - name: IMAGEVECTOR_OVERWRITE
          value: /charts_overwrite/images_overwrite.yaml
        image: ` + etcdDruidImage + `
        imagePullPolicy: IfNotPresent
        name: etcd-druid
        ports:
        - containerPort: 8080
        resources:
          limits:
            memory: 512Mi
          requests:
            cpu: 50m
            memory: 128Mi
        volumeMounts:
        - mountPath: /charts_overwrite
          name: imagevector-overwrite
          readOnly: true
      priorityClassName: ` + priorityClassName + `
      serviceAccountName: etcd-druid
      volumes:
      - configMap:
          name: ` + configMapName + `
        name: imagevector-overwrite
status: {}
`
			serviceYAML = `apiVersion: v1
kind: Service
metadata:
  annotations:
    networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports: '[{"protocol":"TCP","port":8080}]'
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
    high-availability-config.resources.gardener.cloud/type: controller
  name: etcd-druid
  namespace: ` + namespace + `
spec:
  ports:
  - name: metrics
    port: 8080
    protocol: TCP
    targetPort: 8080
  selector:
    gardener.cloud/role: etcd-druid
  sessionAffinity: None
  type: ClusterIP
status:
  loadBalancer: {}
`
			podDisruptionYAML = `apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  creationTimestamp: null
  labels:
    gardener.cloud/role: etcd-druid
  name: etcd-druid
  namespace: ` + namespace + `
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      gardener.cloud/role: etcd-druid
status:
  currentHealthy: 0
  desiredHealthy: 0
  disruptionsAllowed: 0
  expectedPods: 0
`

			managedResourceSecret *corev1.Secret
			managedResource       *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			managedResourceSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceSecretName,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"serviceaccount__" + namespace + "__etcd-druid.yaml":            []byte(serviceAccountYAML),
					"clusterrole____gardener.cloud_system_etcd-druid.yaml":          []byte(clusterRoleYAML),
					"clusterrolebinding____gardener.cloud_system_etcd-druid.yaml":   []byte(clusterRoleBindingYAML),
					"verticalpodautoscaler__" + namespace + "__etcd-druid-vpa.yaml": []byte(vpaYAML),
					"service__" + namespace + "__etcd-druid.yaml":                   []byte(serviceYAML),
					"deployment__" + namespace + "__etcd-druid.yaml":                []byte(deploymentWithoutImageVectorOverwriteYAML),
					"poddisruptionbudget__" + namespace + "__etcd-druid.yaml":       []byte(podDisruptionYAML),
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
					Class:       pointer.String("seed"),
					KeepObjects: pointer.Bool(false),
				},
			}
		})

		It("should fail because the managed resource secret cannot be updated", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
			)

			Expect(bootstrapper.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the managed resource cannot be updated", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr),
			)

			Expect(bootstrapper.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully deploy all the resources (w/o image vector overwrite)", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
					Expect(obj).To(Equal(managedResourceSecret))
				}),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
					Expect(obj).To(DeepEqual(managedResource))
				}),
			)

			Expect(bootstrapper.Deploy(ctx)).To(Succeed())
		})

		It("should successfully deploy all the resources (w/ image vector overwrite)", func() {
			bootstrapper = NewBootstrapper(c, namespace, kubernetesVersion, etcdConfig, etcdDruidImage, imageVectorOverwriteFull, priorityClassName)

			managedResourceSecret.Data["configmap__"+namespace+"__"+configMapName+".yaml"] = []byte(configMapImageVectorOverwriteYAML)
			managedResourceSecret.Data["deployment__"+namespace+"__etcd-druid.yaml"] = []byte(deploymentWithImageVectorOverwriteYAML)

			gomock.InOrder(
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
					Expect(obj).To(DeepEqual(managedResourceSecret))
				}),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
					Expect(obj).To(DeepEqual(managedResource))
				}),
			)

			Expect(bootstrapper.Deploy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		It("should fail because it cannot be checked if the managed resource became healthy", func() {
			oldTimeout := TimeoutWaitForManagedResource
			defer func() { TimeoutWaitForManagedResource = oldTimeout }()
			TimeoutWaitForManagedResource = time.Millisecond

			c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr)

			Expect(bootstrapper.Wait(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the managed resource doesn't become healthy", func() {
			oldTimeout := TimeoutWaitForManagedResource
			defer func() { TimeoutWaitForManagedResource = oldTimeout }()
			TimeoutWaitForManagedResource = time.Millisecond

			c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(
				func(ctx context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
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

			Expect(bootstrapper.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
		})

		It("should successfully wait for the managed resource to become healthy", func() {
			oldTimeout := TimeoutWaitForManagedResource
			defer func() { TimeoutWaitForManagedResource = oldTimeout }()
			TimeoutWaitForManagedResource = time.Millisecond

			c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(
				func(ctx context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
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

			Expect(bootstrapper.Wait(ctx)).To(Succeed())
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

			timeNowFunc = func() time.Time { return time.Time{} }
		)

		Describe("#Destroy", func() {
			It("should fail when the etcd listing fails", func() {
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{})).Return(fakeErr)

				Expect(bootstrapper.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should succeed when isNoMatch error is returned", func() {
				noMatchError := &meta.NoKindMatchError{}
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{})).Return(noMatchError)

				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdCopyBackupsTaskList{})).Return(noMatchError)

				c.EXPECT().Delete(gomock.Any(), gomock.Any())
				c.EXPECT().Delete(gomock.Any(), gomock.Any())

				Expect(bootstrapper.Destroy(ctx)).To(Succeed())
			})

			It("should suceed when NotFoundError is returned", func() {
				notFoundError := apierrors.NewNotFound(schema.GroupResource{}, "etcd")
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{})).Return(notFoundError)

				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdCopyBackupsTaskList{})).Return(notFoundError)

				c.EXPECT().Delete(gomock.Any(), gomock.Any())
				c.EXPECT().Delete(gomock.Any(), gomock.Any())

				Expect(bootstrapper.Destroy(ctx)).To(Succeed())
			})

			It("should fail when there are etcd resources left", func() {
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{})).DoAndReturn(
					func(ctx context.Context, list client.ObjectList, _ ...client.ListOptions) error {
						(&druidv1alpha1.EtcdList{
							Items: []druidv1alpha1.Etcd{{}},
						}).DeepCopyInto(list.(*druidv1alpha1.EtcdList))
						return nil
					},
				)

				Expect(bootstrapper.Destroy(ctx)).To(MatchError(ContainSubstring("because there are still druidv1alpha1.Etcd resources left in the cluster")))
			})

			It("should fail when there are EtcdCopyBackupsTask resources left", func() {
				notFoundError := apierrors.NewNotFound(schema.GroupResource{}, "etcd")
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{})).Return(notFoundError)
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdCopyBackupsTaskList{})).DoAndReturn(
					func(ctx context.Context, list client.ObjectList, _ ...client.ListOptions) error {
						(&druidv1alpha1.EtcdCopyBackupsTaskList{
							Items: []druidv1alpha1.EtcdCopyBackupsTask{{}},
						}).DeepCopyInto(list.(*druidv1alpha1.EtcdCopyBackupsTaskList))
						return nil
					},
				)
				Expect(bootstrapper.Destroy(ctx)).To(MatchError(ContainSubstring("because there are still druidv1alpha1.EtcdCopyBackupsTask resources left in the cluster")))
			})

			It("should fail when the managed resource deletion fails", func() {
				gomock.InOrder(
					c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{})),
					c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdCopyBackupsTaskList{})),
					c.EXPECT().Delete(ctx, managedResource).Return(fakeErr),
				)

				Expect(bootstrapper.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the secret deletion fails", func() {
				gomock.InOrder(
					c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{})),
					c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdCopyBackupsTaskList{})),
					c.EXPECT().Delete(ctx, managedResource),
					c.EXPECT().Delete(ctx, secret).Return(fakeErr),
				)

				Expect(bootstrapper.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully delete all resources", func() {
				gomock.InOrder(
					c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{})),
					c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdCopyBackupsTaskList{})),
					c.EXPECT().Delete(ctx, managedResource),
					c.EXPECT().Delete(ctx, secret),
				)

				Expect(bootstrapper.Destroy(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion fails", func() {
				oldTimeNow := gardenerutils.TimeNow
				defer func() { gardenerutils.TimeNow = oldTimeNow }()
				gardenerutils.TimeNow = timeNowFunc

				c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr)

				Expect(bootstrapper.WaitCleanup(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the wait for the managed resource deletion times out", func() {
				oldTimeNow := gardenerutils.TimeNow
				defer func() { gardenerutils.TimeNow = oldTimeNow }()
				gardenerutils.TimeNow = timeNowFunc

				oldTimeout := TimeoutWaitForManagedResource
				defer func() { TimeoutWaitForManagedResource = oldTimeout }()
				TimeoutWaitForManagedResource = time.Millisecond

				c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).AnyTimes()

				Expect(bootstrapper.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should successfully delete all resources", func() {
				oldTimeNow := gardenerutils.TimeNow
				defer func() { gardenerutils.TimeNow = oldTimeNow }()
				gardenerutils.TimeNow = timeNowFunc

				c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

				Expect(bootstrapper.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
