// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	extensionsmockworker "github.com/gardener/gardener/extensions/pkg/controller/worker/mock"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockmanager "github.com/gardener/gardener/pkg/mock/controller-runtime/manager"
)

var _ = Describe("Worker Reconcile", func() {
	type fields struct {
		logger   logr.Logger
		actuator func(ctrl *gomock.Controller) worker.Actuator
		ctx      context.Context
		client   client.Client
	}
	type args struct {
		request reconcile.Request
	}
	type test struct {
		fields  fields
		args    args
		want    reconcile.Result
		wantErr bool
	}

	// Immutable through the function calls
	arguments := args{
		request: reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      "workerTestReconcile",
				Namespace: "test",
			},
		},
	}

	var (
		ctrl   *gomock.Controller
		ctx    context.Context
		logger logr.Logger
		mgr    *mockmanager.MockManager
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		ctx = context.TODO()
		logger = log.Log.WithName("Reconcile-Test-Controller")

		// Create fake manager
		mgr = mockmanager.NewMockManager(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	var (
		newMockActuator = func(op string, err error) func(ctrl *gomock.Controller) worker.Actuator {
			return func(ctrl *gomock.Controller) worker.Actuator {
				actuator := extensionsmockworker.NewMockActuator(ctrl)
				switch op {
				case "reconcile":
					actuator.EXPECT().Reconcile(ctx, gomock.AssignableToTypeOf(logr.Logger{}), gomock.AssignableToTypeOf(&extensionsv1alpha1.Worker{}), gomock.AssignableToTypeOf(&extensionscontroller.Cluster{})).Return(err)
				case "delete":
					actuator.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(logr.Logger{}), gomock.AssignableToTypeOf(&extensionsv1alpha1.Worker{}), gomock.AssignableToTypeOf(&extensionscontroller.Cluster{})).Return(err)
				case "restore":
					actuator.EXPECT().Restore(ctx, gomock.AssignableToTypeOf(logr.Logger{}), gomock.AssignableToTypeOf(&extensionsv1alpha1.Worker{}), gomock.AssignableToTypeOf(&extensionscontroller.Cluster{})).Return(err)
				case "migrate":
					actuator.EXPECT().Migrate(ctx, gomock.AssignableToTypeOf(logr.Logger{}), gomock.AssignableToTypeOf(&extensionsv1alpha1.Worker{}), gomock.AssignableToTypeOf(&extensionscontroller.Cluster{})).Return(err)
				}
				return actuator
			}
		}
	)

	DescribeTable("Reconcile function", func(t *test) {
		reconciler := worker.NewReconciler(t.fields.actuator(ctrl))

		got, err := reconciler.Reconcile(ctx, t.args.request)
		Expect(err != nil).To(Equal(t.wantErr))
		Expect(reflect.DeepEqual(got, t.want)).To(BeTrue())
	},
		Entry("test reconcile", &test{
			fields: fields{
				logger:   logger,
				actuator: newMockActuator("reconcile", nil),
				ctx:      context.TODO(),
				client: fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithObjects(
					addOperationAnnotationToWorker(
						getWorker(),
						v1beta1constants.GardenerOperationReconcile),
					getCluster()).WithStatusSubresource(getWorker()).Build(),
			},
			args:    arguments,
			want:    reconcile.Result{},
			wantErr: false,
		}),
		Entry("test after successful migrate", &test{
			fields: fields{
				logger:   logger,
				actuator: newMockActuator("", nil),
				ctx:      context.TODO(),
				client: fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithObjects(
					addFinalizerToWorker(
						addDeletionTimestampToWorker(
							addOperationAnnotationToWorker(
								addLastOperationToWorker(
									getWorker(),
									gardencorev1beta1.LastOperationTypeMigrate,
									gardencorev1beta1.LastOperationStateSucceeded,
									"Migrate worker"),
								v1beta1constants.GardenerOperationReconcile)),
						worker.FinalizerName),
					getCluster()).WithStatusSubresource(getWorker()).Build(),
			},
			args:    arguments,
			want:    reconcile.Result{},
			wantErr: false,
		}),
		Entry("test migrate when operationAnnotation Migrate occurs", &test{
			fields: fields{
				logger:   logger,
				actuator: newMockActuator("migrate", nil),
				ctx:      context.TODO(),
				client: fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithObjects(
					addOperationAnnotationToWorker(
						getWorker(),
						v1beta1constants.GardenerOperationMigrate),
					getCluster()).WithStatusSubresource(getWorker()).Build(),
			},
			args:    arguments,
			want:    reconcile.Result{},
			wantErr: false,
		}),
		Entry("test error during migrate when operationAnnotation Migrate occurs", &test{
			fields: fields{
				logger:   logger,
				actuator: newMockActuator("migrate", errors.New("test")),
				ctx:      context.TODO(),
				client: fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithObjects(
					addOperationAnnotationToWorker(
						getWorker(),
						v1beta1constants.GardenerOperationMigrate),
					getCluster()).WithStatusSubresource(getWorker()).Build(),
			},
			args:    arguments,
			want:    reconcile.Result{},
			wantErr: true,
		}),
		Entry("test Migrate after unssuccesful Migrate", &test{
			fields: fields{
				logger:   logger,
				actuator: newMockActuator("migrate", nil),
				ctx:      context.TODO(),
				client: fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithObjects(
					addLastOperationToWorker(
						getWorker(),
						gardencorev1beta1.LastOperationTypeMigrate,
						gardencorev1beta1.LastOperationStateFailed,
						"Migrate worker"),
					getCluster()).WithStatusSubresource(getWorker()).Build(),
			},
			args:    arguments,
			want:    reconcile.Result{},
			wantErr: false,
		}),
		Entry("test error during Migrate after unssuccesful Migrate", &test{
			fields: fields{
				logger:   logger,
				actuator: newMockActuator("migrate", errors.New("test")),
				ctx:      context.TODO(),
				client: fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithObjects(
					addLastOperationToWorker(
						getWorker(),
						gardencorev1beta1.LastOperationTypeMigrate,
						gardencorev1beta1.LastOperationStateFailed,
						"Migrate worker"),
					getCluster()).WithStatusSubresource(getWorker()).Build(),
			},
			args:    arguments,
			want:    reconcile.Result{},
			wantErr: true,
		}),
		Entry("test Delete Worker", &test{
			fields: fields{
				logger:   logger,
				actuator: newMockActuator("delete", nil),
				ctx:      context.TODO(),
				client: fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithObjects(
					addFinalizerToWorker(addDeletionTimestampToWorker(getWorker()), worker.FinalizerName),
					getCluster()).WithStatusSubresource(getWorker()).Build(),
			},
			args:    arguments,
			want:    reconcile.Result{},
			wantErr: false,
		}),
		Entry("test error when Delete Worker", &test{
			fields: fields{
				logger:   logger,
				actuator: newMockActuator("delete", errors.New("test")),
				ctx:      context.TODO(),
				client: fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithObjects(
					addFinalizerToWorker(addDeletionTimestampToWorker(getWorker()), worker.FinalizerName),
					getCluster()).WithStatusSubresource(getWorker()).Build(),
			},
			args:    arguments,
			want:    reconcile.Result{},
			wantErr: true,
		}),
		Entry("test restore when operationAnnotation Restore occurs", &test{
			fields: fields{
				logger:   logger,
				actuator: newMockActuator("restore", nil),
				ctx:      context.TODO(),
				client: fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithObjects(
					addOperationAnnotationToWorker(
						getWorker(),
						v1beta1constants.GardenerOperationRestore),
					getCluster()).WithStatusSubresource(getWorker()).Build(),
			},
			args:    arguments,
			want:    reconcile.Result{},
			wantErr: false,
		}),
		Entry("test error restore when operationAnnotation Restore occurs", &test{
			fields: fields{
				logger:   logger,
				actuator: newMockActuator("restore", errors.New("test")),
				ctx:      context.TODO(),
				client: fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithObjects(
					addOperationAnnotationToWorker(
						getWorker(),
						v1beta1constants.GardenerOperationRestore),
					getCluster()).WithStatusSubresource(getWorker()).Build(),
			},
			args:    arguments,
			want:    reconcile.Result{},
			wantErr: true,
		}),
		Entry("test reconcile after failed reconcilation", &test{
			fields: fields{
				logger:   logger,
				actuator: newMockActuator("reconcile", nil),
				ctx:      context.TODO(),
				client: fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithObjects(
					addLastOperationToWorker(
						getWorker(),
						gardencorev1beta1.LastOperationTypeReconcile,
						gardencorev1beta1.LastOperationStateFailed,
						"Reconcile worker"),
					getCluster()).WithStatusSubresource(getWorker()).Build(),
			},
			args:    arguments,
			want:    reconcile.Result{},
			wantErr: false,
		}),
		Entry("test reconcile after successful restoration reconcilation", &test{
			fields: fields{
				logger:   logger,
				actuator: newMockActuator("reconcile", nil),
				ctx:      context.TODO(),
				client: fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithObjects(
					addLastOperationToWorker(
						getWorker(),
						gardencorev1beta1.LastOperationTypeReconcile,
						gardencorev1beta1.LastOperationStateProcessing,
						"Processs worker reconcilation"),
					getCluster()).WithStatusSubresource(getWorker()).Build(),
			},
			args:    arguments,
			want:    reconcile.Result{},
			wantErr: false,
		}),
		Entry("test error while reconciliation after failed reconcilation", &test{
			fields: fields{
				logger:   logger,
				actuator: newMockActuator("reconcile", errors.New("test")),
				ctx:      context.TODO(),
				client: fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithObjects(
					addLastOperationToWorker(
						getWorker(),
						gardencorev1beta1.LastOperationTypeReconcile,
						gardencorev1beta1.LastOperationStateFailed,
						"Reconcile worker"),
					getCluster()).WithStatusSubresource(getWorker()).Build(),
			},
			args:    arguments,
			want:    reconcile.Result{},
			wantErr: true,
		}),
		Entry("test error while reconciliation after successful restoration reconcilation", &test{
			fields: fields{
				logger:   logger,
				actuator: newMockActuator("reconcile", errors.New("test")),
				ctx:      context.TODO(),
				client: fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithObjects(
					addLastOperationToWorker(
						getWorker(),
						gardencorev1beta1.LastOperationTypeReconcile,
						gardencorev1beta1.LastOperationStateProcessing,
						"Processs worker reconcilation"),
					getCluster()).WithStatusSubresource(getWorker()).Build(),
			},
			args:    arguments,
			want:    reconcile.Result{},
			wantErr: true,
		}),
	)
})

func getWorker() *extensionsv1alpha1.Worker {
	return &extensionsv1alpha1.Worker{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Worker",
			APIVersion: "extensions.gardener.cloud/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "workerTestReconcile",
			Namespace:       "test",
			ResourceVersion: "42",
		},
		Spec: extensionsv1alpha1.WorkerSpec{},
	}
}

func addOperationAnnotationToWorker(worker *extensionsv1alpha1.Worker, annotation string) *extensionsv1alpha1.Worker {
	worker.Annotations = make(map[string]string)
	worker.Annotations[v1beta1constants.GardenerOperation] = annotation
	return worker
}

func addLastOperationToWorker(worker *extensionsv1alpha1.Worker, lastOperationType gardencorev1beta1.LastOperationType, lastOperationState gardencorev1beta1.LastOperationState, description string) *extensionsv1alpha1.Worker {
	worker.Status.LastOperation = extensionscontroller.LastOperation(lastOperationType, lastOperationState, 1, description)
	return worker
}

func addDeletionTimestampToWorker(worker *extensionsv1alpha1.Worker) *extensionsv1alpha1.Worker {
	worker.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	return worker
}

func addFinalizerToWorker(worker *extensionsv1alpha1.Worker, finalizer string) *extensionsv1alpha1.Worker {
	worker.Finalizers = append(worker.Finalizers, finalizer)
	return worker
}

func getCluster() *extensionsv1alpha1.Cluster {
	return &extensionsv1alpha1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "extensions.gardener.cloud/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			Shoot: runtime.RawExtension{
				Raw: encode(&gardencorev1beta1.Shoot{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Shoot",
						APIVersion: "core.gardener.cloud/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				}),
			},
		},
	}
}

func encode(obj runtime.Object) []byte {
	bytes, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	return bytes
}
