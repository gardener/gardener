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

package etcd_test

import (
	"context"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Etcd", func() {
	var (
		ctrl         *gomock.Controller
		c            *mockclient.MockClient
		bootstrapper component.DeployWaiter
		conf         *config.GardenletConfiguration

		ctx                       = context.TODO()
		fakeErr                   = fmt.Errorf("fake error")
		namespace                 = "shoot--foo--bar"
		etcdDruidImage            = "etcd/druid:1.2.3"
		imageVectorOverwriteEmpty *string
		imageVectorOverwriteFull  = pointer.String("some overwrite")

		managedResourceName       = "etcd-druid"
		managedResourceSecretName = "managedresource-" + managedResourceName
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		conf = &config.GardenletConfiguration{
			ETCDConfig: &config.ETCDConfig{
				ETCDController: &config.ETCDController{
					Workers: pointer.Int64Ptr(50),
				},
				CustodianController: &config.CustodianController{
					Workers: pointer.Int64Ptr(3),
				},
				BackupCompactionController: &config.BackupCompactionController{
					Workers:                pointer.Int64Ptr(3),
					EnableBackupCompaction: pointer.Bool(true),
					EventsThreshold:        pointer.Int64(1000000),
					ActiveDeadlineDuration: &metav1.Duration{Duration: time.Hour * 3},
				},
			},
		}

		bootstrapper = NewBootstrapper(c, namespace, conf, etcdDruidImage, imageVectorOverwriteEmpty)
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
  - list
  - watch
  - delete
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
  - apps
  resources:
  - services
  - configmaps
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
  - batch
  resources:
  - cronjobs
  verbs:
  - get
  - list
  - watch
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
      controlledValues: RequestsOnly
      minAllowed:
        cpu: 50m
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
        security.gardener.cloud/trigger: rollout
      creationTimestamp: null
      labels:
        gardener.cloud/role: etcd-druid
    spec:
      containers:
      - command:
        - /bin/etcd-druid
        - --enable-leader-election=true
        - --ignore-operation-annotation=false
        - --disable-etcd-serviceaccount-automount=true
        - --workers=50
        - --custodian-workers=3
        - --compaction-workers=3
        - --enable-backup-compaction=true
        - --etcd-events-threshold=1000000
        - --active-deadline-duration=3h0m0s
        image: ` + etcdDruidImage + `
        imagePullPolicy: IfNotPresent
        name: etcd-druid
        ports:
        - containerPort: 9569
        resources:
          limits:
            memory: 512Mi
          requests:
            cpu: 50m
            memory: 128Mi
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
        security.gardener.cloud/trigger: rollout
      creationTimestamp: null
      labels:
        gardener.cloud/role: etcd-druid
    spec:
      containers:
      - command:
        - /bin/etcd-druid
        - --enable-leader-election=true
        - --ignore-operation-annotation=false
        - --disable-etcd-serviceaccount-automount=true
        - --workers=50
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
        - containerPort: 9569
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
      serviceAccountName: etcd-druid
      volumes:
      - configMap:
          name: ` + configMapName + `
        name: imagevector-overwrite
status: {}
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
					"serviceaccount__shoot--foo--bar__etcd-druid.yaml":            []byte(serviceAccountYAML),
					"clusterrole____gardener.cloud_system_etcd-druid.yaml":        []byte(clusterRoleYAML),
					"clusterrolebinding____gardener.cloud_system_etcd-druid.yaml": []byte(clusterRoleBindingYAML),
					"verticalpodautoscaler__shoot--foo--bar__etcd-druid-vpa.yaml": []byte(vpaYAML),
					"deployment__shoot--foo--bar__etcd-druid.yaml":                []byte(deploymentWithoutImageVectorOverwriteYAML),
					"crd.yaml":                    []byte(crd),
					"crdEtcdCopyBackupsTask.yaml": []byte(etcdCopyBackupsTaskCRDYaml),
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
				c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
			)

			Expect(bootstrapper.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the managed resource cannot be updated", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr),
			)

			Expect(bootstrapper.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully deploy all the resources (w/o image vector overwrite)", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
					Expect(obj).To(Equal(managedResourceSecret))
				}),
				c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
					Expect(obj).To(DeepEqual(managedResource))
				}),
			)

			Expect(bootstrapper.Deploy(ctx)).To(Succeed())
		})

		It("should successfully deploy all the resources (w/ image vector overwrite)", func() {
			bootstrapper = NewBootstrapper(c, namespace, conf, etcdDruidImage, imageVectorOverwriteFull)

			managedResourceSecret.Data["configmap__shoot--foo--bar__"+configMapName+".yaml"] = []byte(configMapImageVectorOverwriteYAML)
			managedResourceSecret.Data["deployment__shoot--foo--bar__etcd-druid.yaml"] = []byte(deploymentWithImageVectorOverwriteYAML)

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

			Expect(bootstrapper.Deploy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		It("should fail because it cannot be checked if the managed resource became healthy", func() {
			oldTimeout := TimeoutWaitForManagedResource
			defer func() { TimeoutWaitForManagedResource = oldTimeout }()
			TimeoutWaitForManagedResource = time.Millisecond

			c.EXPECT().Get(gomock.Any(), kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr)

			Expect(bootstrapper.Wait(ctx)).To(MatchError(fakeErr))
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

			Expect(bootstrapper.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
		})

		It("should successfully wait for the managed resource to become healthy", func() {
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
				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any())

				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdCopyBackupsTaskList{})).Return(noMatchError)
				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any())

				c.EXPECT().Delete(gomock.Any(), gomock.Any())
				c.EXPECT().Delete(gomock.Any(), gomock.Any())

				Expect(bootstrapper.Destroy(ctx)).To(Succeed())
			})

			It("should suceed when NotFoundError is returned", func() {
				notFoundError := apierrors.NewNotFound(schema.GroupResource{}, "etcd")
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{})).Return(notFoundError)
				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any())

				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdCopyBackupsTaskList{})).Return(notFoundError)
				c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any())

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
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any())
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

			It("should fail when the deletion confirmation for etcd CRDs fails", func() {
				gomock.InOrder(
					c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any()).Return(fakeErr),
				)

				Expect(bootstrapper.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the deletion confirmation for EtcdCopyBackupsTask CRDs fails", func() {
				gomock.InOrder(
					c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any()),
					c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdCopyBackupsTaskList{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any()).Return(fakeErr),
				)

				Expect(bootstrapper.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the managed resource deletion fails", func() {
				gomock.InOrder(
					c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any()),
					c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdCopyBackupsTaskList{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any()),
					c.EXPECT().Delete(ctx, managedResource).Return(fakeErr),
				)

				Expect(bootstrapper.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the secret deletion fails", func() {
				gomock.InOrder(
					c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any()),
					c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdCopyBackupsTaskList{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any()),
					c.EXPECT().Delete(ctx, managedResource),
					c.EXPECT().Delete(ctx, secret).Return(fakeErr),
				)

				Expect(bootstrapper.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should successfully delete all resources", func() {
				gomock.InOrder(
					c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdList{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any()),
					c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&druidv1alpha1.EtcdCopyBackupsTaskList{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&apiextensionsv1.CustomResourceDefinition{}), gomock.Any()),
					c.EXPECT().Delete(ctx, managedResource),
					c.EXPECT().Delete(ctx, secret),
				)

				Expect(bootstrapper.Destroy(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion fails", func() {
				oldTimeNow := gutil.TimeNow
				defer func() { gutil.TimeNow = oldTimeNow }()
				gutil.TimeNow = timeNowFunc

				c.EXPECT().Get(gomock.Any(), kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr)

				Expect(bootstrapper.WaitCleanup(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the wait for the managed resource deletion times out", func() {
				oldTimeNow := gutil.TimeNow
				defer func() { gutil.TimeNow = oldTimeNow }()
				gutil.TimeNow = timeNowFunc

				oldTimeout := TimeoutWaitForManagedResource
				defer func() { TimeoutWaitForManagedResource = oldTimeout }()
				TimeoutWaitForManagedResource = time.Millisecond

				c.EXPECT().Get(gomock.Any(), kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).AnyTimes()

				Expect(bootstrapper.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should successfully delete all resources", func() {
				oldTimeNow := gutil.TimeNow
				defer func() { gutil.TimeNow = oldTimeNow }()
				gutil.TimeNow = timeNowFunc

				c.EXPECT().Get(gomock.Any(), kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

				Expect(bootstrapper.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

const (
	crd = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: etcds.druid.gardener.cloud
  annotations:
    controller-gen.kubebuilder.io/version: v0.4.1
  labels:
    gardener.cloud/deletion-protected: "true"
spec:
  group: druid.gardener.cloud
  names:
    kind: Etcd
    listKind: EtcdList
    plural: etcds
    singular: etcd
  scope: Namespaced
  versions:
  - additionalPrinterColumns:
    - jsonPath: .status.ready
      name: Ready
      type: string
    - jsonPath: .metadata.creationTimestamp
      name: Age
      type: date
    name: v1alpha1
    schema:
      openAPIV3Schema:
        description: Etcd is the Schema for the etcds API
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            description: EtcdSpec defines the desired state of Etcd
            properties:
              annotations:
                additionalProperties:
                  type: string
                type: object
              backup:
                description: BackupSpec defines parameters associated with the full
                  and delta snapshots of etcd.
                properties:
                  compactionResources:
                    description: 'CompactionResources defines compute Resources required
                      by compaction job. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/'
                    properties:
                      limits:
                        additionalProperties:
                          anyOf:
                          - type: integer
                          - type: string
                          pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                          x-kubernetes-int-or-string: true
                        description: 'Limits describes the maximum amount of compute
                          resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/'
                        type: object
                      requests:
                        additionalProperties:
                          anyOf:
                          - type: integer
                          - type: string
                          pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                          x-kubernetes-int-or-string: true
                        description: 'Requests describes the minimum amount of compute
                          resources required. If Requests is omitted for a container,
                          it defaults to Limits if that is explicitly specified, otherwise
                          to an implementation-defined value. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/'
                        type: object
                    type: object
                  compression:
                    description: SnapshotCompression defines the specification for
                      compression of Snapshots.
                    properties:
                      enabled:
                        type: boolean
                      policy:
                        description: CompressionPolicy defines the type of policy
                          for compression of snapshots.
                        enum:
                        - gzip
                        - lzw
                        - zlib
                        type: string
                    type: object
                  deltaSnapshotMemoryLimit:
                    anyOf:
                    - type: integer
                    - type: string
                    description: DeltaSnapshotMemoryLimit defines the memory limit
                      after which delta snapshots will be taken
                    pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                    x-kubernetes-int-or-string: true
                  deltaSnapshotPeriod:
                    description: DeltaSnapshotPeriod defines the period after which
                      delta snapshots will be taken
                    type: string
                  enableProfiling:
                    description: EnableProfiling defines if profiling should be enabled
                      for the etcd-backup-restore-sidecar
                    type: boolean
                  etcdSnapshotTimeout:
                    description: EtcdSnapshotTimeout defines the timeout duration
                      for etcd FullSnapshot operation
                    type: string
                  fullSnapshotSchedule:
                    description: FullSnapshotSchedule defines the cron standard schedule
                      for full snapshots.
                    type: string
                  garbageCollectionPeriod:
                    description: GarbageCollectionPeriod defines the period for garbage
                      collecting old backups
                    type: string
                  garbageCollectionPolicy:
                    description: GarbageCollectionPolicy defines the policy for garbage
                      collecting old backups
                    enum:
                    - Exponential
                    - LimitBased
                    type: string
                  image:
                    description: Image defines the etcd container image and tag
                    type: string
                  leaderElection:
                    description: LeaderElection defines parameters related to the
                      LeaderElection configuration.
                    properties:
                      etcdConnectionTimeout:
                        description: EtcdConnectionTimeout defines the timeout duration
                          for etcd client connection during leader election.
                        type: string
                      reelectionPeriod:
                        description: ReelectionPeriod defines the Period after which
                          leadership status of corresponding etcd is checked.
                        type: string
                    type: object
                  ownerCheck:
                    description: OwnerCheck defines parameters related to checking
                      if the cluster owner, as specified in the owner DNS record,
                      is the expected one.
                    properties:
                      dnsCacheTTL:
                        description: DNSCacheTTL is the DNS cache TTL for owner checks.
                        type: string
                      id:
                        description: ID is the owner id value that is expected to
                          be found in the owner DNS record.
                        type: string
                      interval:
                        description: Interval is the time interval between owner checks.
                        type: string
                      name:
                        description: Name is the domain name of the owner DNS record.
                        type: string
                      timeout:
                        description: Timeout is the timeout for owner checks.
                        type: string
                    required:
                    - id
                    - name
                    type: object
                  port:
                    description: Port define the port on which etcd-backup-restore
                      server will be exposed.
                    format: int32
                    type: integer
                  resources:
                    description: 'Resources defines compute Resources required by
                      backup-restore container. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/'
                    properties:
                      limits:
                        additionalProperties:
                          anyOf:
                          - type: integer
                          - type: string
                          pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                          x-kubernetes-int-or-string: true
                        description: 'Limits describes the maximum amount of compute
                          resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/'
                        type: object
                      requests:
                        additionalProperties:
                          anyOf:
                          - type: integer
                          - type: string
                          pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                          x-kubernetes-int-or-string: true
                        description: 'Requests describes the minimum amount of compute
                          resources required. If Requests is omitted for a container,
                          it defaults to Limits if that is explicitly specified, otherwise
                          to an implementation-defined value. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/'
                        type: object
                    type: object
                  store:
                    description: Store defines the specification of object store provider
                      for storing backups.
                    properties:
                      container:
                        description: Container is the name of the container the backup
                          is stored at.
                        type: string
                      prefix:
                        description: Prefix is the prefix used for the store.
                        type: string
                      provider:
                        description: Provider is the name of the backup provider.
                        type: string
                      secretRef:
                        description: SecretRef is the reference to the secret which
                          used to connect to the backup store.
                        properties:
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                    required:
                    - prefix
                    type: object
                  tls:
                    description: TLSConfig hold the TLS configuration details.
                    properties:
                      clientTLSSecretRef:
                        description: SecretReference represents a Secret Reference.
                          It has enough information to retrieve secret in any namespace
                        properties:
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                      serverTLSSecretRef:
                        description: SecretReference represents a Secret Reference.
                          It has enough information to retrieve secret in any namespace
                        properties:
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                      tlsCASecretRef:
                        description: SecretReference defines a reference to a secret.
                        properties:
                          dataKey:
                            description: DataKey is the name of the key in the data
                              map containing the credentials.
                            type: string
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                    required:
                    - clientTLSSecretRef
                    - serverTLSSecretRef
                    - tlsCASecretRef
                    type: object
                type: object
              etcd:
                description: EtcdConfig defines parameters associated etcd deployed
                properties:
                  authSecretRef:
                    description: SecretReference represents a Secret Reference. It
                      has enough information to retrieve secret in any namespace
                    properties:
                      name:
                        description: Name is unique within a namespace to reference
                          a secret resource.
                        type: string
                      namespace:
                        description: Namespace defines the space within which the
                          secret name must be unique.
                        type: string
                    type: object
                  clientPort:
                    format: int32
                    type: integer
                  defragmentationSchedule:
                    description: DefragmentationSchedule defines the cron standard
                      schedule for defragmentation of etcd.
                    type: string
                  etcdDefragTimeout:
                    description: EtcdDefragTimeout defines the timeout duration for
                      etcd defrag call
                    type: string
                  heartbeatDuration:
                    description: HeartbeatDuration defines the duration for members
                      to send heartbeats. The default value is 10s.
                    type: string
                  image:
                    description: Image defines the etcd container image and tag
                    type: string
                  metrics:
                    description: Metrics defines the level of detail for exported
                      metrics of etcd, specify 'extensive' to include histogram metrics.
                    enum:
                    - basic
                    - extensive
                    type: string
                  quota:
                    anyOf:
                    - type: integer
                    - type: string
                    description: Quota defines the etcd DB quota.
                    pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                    x-kubernetes-int-or-string: true
                  resources:
                    description: 'Resources defines the compute Resources required
                      by etcd container. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/'
                    properties:
                      limits:
                        additionalProperties:
                          anyOf:
                          - type: integer
                          - type: string
                          pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                          x-kubernetes-int-or-string: true
                        description: 'Limits describes the maximum amount of compute
                          resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/'
                        type: object
                      requests:
                        additionalProperties:
                          anyOf:
                          - type: integer
                          - type: string
                          pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                          x-kubernetes-int-or-string: true
                        description: 'Requests describes the minimum amount of compute
                          resources required. If Requests is omitted for a container,
                          it defaults to Limits if that is explicitly specified, otherwise
                          to an implementation-defined value. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/'
                        type: object
                    type: object
                  serverPort:
                    format: int32
                    type: integer
                  tls:
                    description: TLSConfig hold the TLS configuration details.
                    properties:
                      clientTLSSecretRef:
                        description: SecretReference represents a Secret Reference.
                          It has enough information to retrieve secret in any namespace
                        properties:
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                      serverTLSSecretRef:
                        description: SecretReference represents a Secret Reference.
                          It has enough information to retrieve secret in any namespace
                        properties:
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                      tlsCASecretRef:
                        description: SecretReference defines a reference to a secret.
                        properties:
                          dataKey:
                            description: DataKey is the name of the key in the data
                              map containing the credentials.
                            type: string
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                    required:
                    - clientTLSSecretRef
                    - serverTLSSecretRef
                    - tlsCASecretRef
                    type: object
                type: object
              labels:
                additionalProperties:
                  type: string
                type: object
              priorityClassName:
                description: PriorityClassName is the name of a priority class that
                  shall be used for the etcd pods.
                type: string
              replicas:
                format: int32
                type: integer
              selector:
                description: 'selector is a label query over pods that should match
                  the replica count. It must match the pod template''s labels. More
                  info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors'
                properties:
                  matchExpressions:
                    description: matchExpressions is a list of label selector requirements.
                      The requirements are ANDed.
                    items:
                      description: A label selector requirement is a selector that
                        contains values, a key, and an operator that relates the key
                        and values.
                      properties:
                        key:
                          description: key is the label key that the selector applies
                            to.
                          type: string
                        operator:
                          description: operator represents a key's relationship to
                            a set of values. Valid operators are In, NotIn, Exists
                            and DoesNotExist.
                          type: string
                        values:
                          description: values is an array of string values. If the
                            operator is In or NotIn, the values array must be non-empty.
                            If the operator is Exists or DoesNotExist, the values
                            array must be empty. This array is replaced during a strategic
                            merge patch.
                          items:
                            type: string
                          type: array
                      required:
                      - key
                      - operator
                      type: object
                    type: array
                  matchLabels:
                    additionalProperties:
                      type: string
                    description: matchLabels is a map of {key,value} pairs. A single
                      {key,value} in the matchLabels map is equivalent to an element
                      of matchExpressions, whose key field is "key", the operator
                      is "In", and the values array contains only "value". The requirements
                      are ANDed.
                    type: object
                type: object
              sharedConfig:
                description: SharedConfig defines parameters shared and used by Etcd
                  as well as backup-restore sidecar.
                properties:
                  autoCompactionMode:
                    description: AutoCompactionMode defines the auto-compaction-mode:'periodic'
                      mode or 'revision' mode for etcd and embedded-Etcd of backup-restore
                      sidecar.
                    enum:
                    - periodic
                    - revision
                    type: string
                  autoCompactionRetention:
                    description: AutoCompactionRetention defines the auto-compaction-retention
                      length for etcd as well as for embedded-Etcd of backup-restore
                      sidecar.
                    type: string
                type: object
              storageCapacity:
                anyOf:
                - type: integer
                - type: string
                description: StorageCapacity defines the size of persistent volume.
                pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                x-kubernetes-int-or-string: true
              storageClass:
                description: 'StorageClass defines the name of the StorageClass required
                  by the claim. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1'
                type: string
              volumeClaimTemplate:
                description: VolumeClaimTemplate defines the volume claim template
                  to be created
                type: string
            required:
            - backup
            - etcd
            - labels
            - replicas
            - selector
            type: object
          status:
            description: EtcdStatus defines the observed state of Etcd.
            properties:
              clusterSize:
                description: Cluster size is the size of the etcd cluster.
                format: int32
                type: integer
              conditions:
                description: Conditions represents the latest available observations
                  of an etcd's current state.
                items:
                  description: Condition holds the information about the state of
                    a resource.
                  properties:
                    lastTransitionTime:
                      description: Last time the condition transitioned from one status
                        to another.
                      format: date-time
                      type: string
                    lastUpdateTime:
                      description: Last time the condition was updated.
                      format: date-time
                      type: string
                    message:
                      description: A human readable message indicating details about
                        the transition.
                      type: string
                    reason:
                      description: The reason for the condition's last transition.
                      type: string
                    status:
                      description: Status of the condition, one of True, False, Unknown.
                      type: string
                    type:
                      description: Type of the Etcd condition.
                      type: string
                  required:
                  - lastTransitionTime
                  - lastUpdateTime
                  - message
                  - reason
                  - status
                  - type
                  type: object
                type: array
              currentReplicas:
                description: CurrentReplicas is the current replica count for the
                  etcd cluster.
                format: int32
                type: integer
              etcd:
                description: CrossVersionObjectReference contains enough information
                  to let you identify the referred resource.
                properties:
                  apiVersion:
                    description: API version of the referent
                    type: string
                  kind:
                    description: Kind of the referent
                    type: string
                  name:
                    description: Name of the referent
                    type: string
                type: object
              labelSelector:
                description: LabelSelector is a label query over pods that should
                  match the replica count. It must match the pod template's labels.
                properties:
                  matchExpressions:
                    description: matchExpressions is a list of label selector requirements.
                      The requirements are ANDed.
                    items:
                      description: A label selector requirement is a selector that
                        contains values, a key, and an operator that relates the key
                        and values.
                      properties:
                        key:
                          description: key is the label key that the selector applies
                            to.
                          type: string
                        operator:
                          description: operator represents a key's relationship to
                            a set of values. Valid operators are In, NotIn, Exists
                            and DoesNotExist.
                          type: string
                        values:
                          description: values is an array of string values. If the
                            operator is In or NotIn, the values array must be non-empty.
                            If the operator is Exists or DoesNotExist, the values
                            array must be empty. This array is replaced during a strategic
                            merge patch.
                          items:
                            type: string
                          type: array
                      required:
                      - key
                      - operator
                      type: object
                    type: array
                  matchLabels:
                    additionalProperties:
                      type: string
                    description: matchLabels is a map of {key,value} pairs. A single
                      {key,value} in the matchLabels map is equivalent to an element
                      of matchExpressions, whose key field is "key", the operator
                      is "In", and the values array contains only "value". The requirements
                      are ANDed.
                    type: object
                type: object
              lastError:
                description: LastError represents the last occurred error.
                type: string
              members:
                description: Members represents the members of the etcd cluster
                items:
                  description: EtcdMemberStatus holds information about a etcd cluster
                    membership.
                  properties:
                    id:
                      description: ID is the ID of the etcd member.
                      type: string
                    lastTransitionTime:
                      description: LastTransitionTime is the last time the condition's
                        status changed.
                      format: date-time
                      type: string
                    name:
                      description: Name is the name of the etcd member. It is the
                        name of the backing ` + "`Pod`" + `.
                      type: string
                    reason:
                      description: The reason for the condition's last transition.
                      type: string
                    role:
                      description: Role is the role in the etcd cluster, either ` + "`Leader`" + `.
                        or ` + "`Member`" + `.
                      type: string
                    status:
                      description: Status of the condition, one of True, False, Unknown.
                      type: string
                  required:
                  - lastTransitionTime
                  - name
                  - reason
                  - status
                  type: object
                type: array
              observedGeneration:
                description: ObservedGeneration is the most recent generation observed
                  for this resource.
                format: int64
                type: integer
              ready:
                description: Ready represents the readiness of the etcd resource.
                type: boolean
              readyReplicas:
                description: ReadyReplicas is the count of replicas being ready in
                  the etcd cluster.
                format: int32
                type: integer
              replicas:
                description: Replicas is the replica count of the etcd resource.
                format: int32
                type: integer
              serviceName:
                description: ServiceName is the name of the etcd service.
                type: string
              updatedReplicas:
                description: UpdatedReplicas is the count of updated replicas in the
                  etcd cluster.
                format: int32
                type: integer
            type: object
        type: object
    served: true
    storage: true
    subresources:
      scale:
        labelSelectorPath: .status.labelSelector
        specReplicasPath: .spec.replicas
        statusReplicasPath: .status.replicas
      status: {}
`

	etcdCopyBackupsTaskCRDYaml = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.7.0
  creationTimestamp: null
  name: etcdcopybackupstasks.druid.gardener.cloud
  labels:
    gardener.cloud/deletion-protected: "true"
spec:
  group: druid.gardener.cloud
  names:
    kind: EtcdCopyBackupsTask
    listKind: EtcdCopyBackupsTaskList
    plural: etcdcopybackupstasks
    singular: etcdcopybackupstask
  scope: Namespaced
  versions:
  - additionalPrinterColumns:
    - jsonPath: .metadata.creationTimestamp
      name: Age
      type: date
    name: v1alpha1
    schema:
      openAPIV3Schema:
        description: EtcdCopyBackupsTask is a task for copying etcd backups from a
          source to a target store.
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            description: EtcdCopyBackupsTaskSpec defines the parameters for the copy
              backups task.
            properties:
              maxBackupAge:
                description: MaxBackupAge is the maximum age in days that a backup
                  must have in order to be copied. By default all backups will be
                  copied.
                format: int32
                type: integer
              maxBackups:
                description: MaxBackups is the maximum number of backups that will
                  be copied starting with the most recent ones.
                format: int32
                type: integer
              sourceStore:
                description: SourceStore defines the specification of the source object
                  store provider for storing backups.
                properties:
                  container:
                    description: Container is the name of the container the backup
                      is stored at.
                    type: string
                  prefix:
                    description: Prefix is the prefix used for the store.
                    type: string
                  provider:
                    description: Provider is the name of the backup provider.
                    type: string
                  secretRef:
                    description: SecretRef is the reference to the secret which used
                      to connect to the backup store.
                    properties:
                      name:
                        description: Name is unique within a namespace to reference
                          a secret resource.
                        type: string
                      namespace:
                        description: Namespace defines the space within which the
                          secret name must be unique.
                        type: string
                    type: object
                required:
                - prefix
                type: object
              targetStore:
                description: TargetStore defines the specification of the target object
                  store provider for storing backups.
                properties:
                  container:
                    description: Container is the name of the container the backup
                      is stored at.
                    type: string
                  prefix:
                    description: Prefix is the prefix used for the store.
                    type: string
                  provider:
                    description: Provider is the name of the backup provider.
                    type: string
                  secretRef:
                    description: SecretRef is the reference to the secret which used
                      to connect to the backup store.
                    properties:
                      name:
                        description: Name is unique within a namespace to reference
                          a secret resource.
                        type: string
                      namespace:
                        description: Namespace defines the space within which the
                          secret name must be unique.
                        type: string
                    type: object
                required:
                - prefix
                type: object
              waitForFinalSnapshot:
                description: WaitForFinalSnapshot defines the parameters for waiting
                  for a final full snapshot before copying backups.
                properties:
                  enabled:
                    description: Enabled specifies whether to wait for a final full
                      snapshot before copying backups.
                    type: boolean
                  timeout:
                    description: Timeout is the timeout for waiting for a final full
                      snapshot. When this timeout expires, the copying of backups
                      will be performed anyway. No timeout or 0 means wait forever.
                    type: string
                required:
                - enabled
                type: object
            required:
            - sourceStore
            - targetStore
            type: object
          status:
            description: EtcdCopyBackupsTaskStatus defines the observed state of the
              copy backups task.
            properties:
              conditions:
                description: Conditions represents the latest available observations
                  of an object's current state.
                items:
                  description: Condition holds the information about the state of
                    a resource.
                  properties:
                    lastTransitionTime:
                      description: Last time the condition transitioned from one status
                        to another.
                      format: date-time
                      type: string
                    lastUpdateTime:
                      description: Last time the condition was updated.
                      format: date-time
                      type: string
                    message:
                      description: A human readable message indicating details about
                        the transition.
                      type: string
                    reason:
                      description: The reason for the condition's last transition.
                      type: string
                    status:
                      description: Status of the condition, one of True, False, Unknown.
                      type: string
                    type:
                      description: Type of the Etcd condition.
                      type: string
                  required:
                  - lastTransitionTime
                  - lastUpdateTime
                  - message
                  - reason
                  - status
                  - type
                  type: object
                type: array
              lastError:
                description: LastError represents the last occurred error.
                type: string
              observedGeneration:
                description: ObservedGeneration is the most recent generation observed
                  for this resource.
                format: int64
                type: integer
            type: object
        type: object
    served: true
    storage: true
    subresources:
      status: {}
`
)
