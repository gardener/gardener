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

package infrastructure_test

import (
	"context"
	"errors"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/infrastructure"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("#ExtensionInfrastructure", func() {
	const (
		namespace    = "test-namespace"
		name         = "test-deploy"
		providerType = "foo"
	)

	var (
		ctx context.Context
		log logrus.FieldLogger

		ctrl    *gomock.Controller
		c       *mockclient.MockClient
		mockNow *mocktime.MockNow
		now     time.Time

		region         string
		sshPublicKey   []byte
		providerConfig *runtime.RawExtension

		infra        *extensionsv1alpha1.Infrastructure
		values       *infrastructure.Values
		deployWaiter shoot.ExtensionInfrastructure
		waiter       *retryfake.Ops

		cleanupFunc func()
	)

	BeforeEach(func() {
		ctx = context.TODO()
		log = logger.NewNopLogger()

		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		mockNow = mocktime.NewMockNow(ctrl)

		region = "europe"
		sshPublicKey = []byte("secure")
		providerConfig = &runtime.RawExtension{
			Raw: []byte("very-provider-specific"),
		}

		values = &infrastructure.Values{
			Namespace:      namespace,
			Name:           name,
			Type:           providerType,
			ProviderConfig: providerConfig,
			Region:         region,
		}
		infra = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}

		waiter = &retryfake.Ops{MaxAttempts: 1}
		cleanupFunc = test.WithVars(
			&retry.Until, waiter.Until,
			&retry.UntilTimeout, waiter.UntilTimeout,
		)

		deployWaiter = infrastructure.New(log, c, values)
	})

	AfterEach(func() {
		ctrl.Finish()
		cleanupFunc()
	})

	Describe("#Deploy", func() {
		DescribeTable("correct Infrastructure is created", func(mutator func()) {
			defer test.WithVars(
				&infrastructure.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			c.
				EXPECT().
				Get(ctx, kutil.Key(namespace, name), infra.DeepCopy()).
				Return(apierrors.NewNotFound(schema.GroupResource{}, name))

			deployWaiter.SetSSHPublicKey(sshPublicKey)
			infra.Spec = extensionsv1alpha1.InfrastructureSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type:           providerType,
					ProviderConfig: providerConfig,
				},
				Region:       region,
				SSHPublicKey: sshPublicKey,
				SecretRef: corev1.SecretReference{
					Name:      v1beta1constants.SecretNameCloudProvider,
					Namespace: namespace,
				},
			}
			mutator()

			c.
				EXPECT().
				Create(ctx, infra)

			Expect(deployWaiter.Deploy(ctx)).To(Succeed())
		},
			Entry("with no modification", func() {}),
			Entry("without provider config", func() {
				values.ProviderConfig = nil
				infra.Spec.ProviderConfig = nil
			}),
			Entry("deployment task", func() {
				values.DeploymentRequested = true
				infra.Annotations = map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					v1beta1constants.GardenerTimestamp: now.UTC().String(),
				}
			}),
			Entry("restoration phase", func() {
				values.IsInRestorePhaseOfControlPlaneMigration = true
				infra.Annotations = map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationWaitForState,
					v1beta1constants.GardenerTimestamp: now.UTC().String(),
				}
			}),
		)
	})

	Describe("#Wait", func() {
		It("should return error when it's not found", func() {
			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				Return(apierrors.NewNotFound(schema.GroupResource{}, name)).
				AnyTimes()

			Expect(deployWaiter.Wait(ctx)).To(HaveOccurred())
		})

		It("should return unexpected errors", func() {
			waiter.MaxAttempts = 2

			fakeErr := errors.New("fake")

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{})).
				Return(fakeErr)

			Expect(deployWaiter.Wait(ctx)).To(MatchError(ContainSubstring(fakeErr.Error())))
		})

		It("should return error when it's not ready", func() {
			description := "some error"

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Infrastructure) error {
					obj.Status.LastError = &gardencorev1beta1.LastError{
						Description: description,
					}
					return nil
				}).
				AnyTimes()

			Expect(deployWaiter.Wait(ctx)).To(MatchError(ContainSubstring(description)))
		})

		It("should return no error when is ready", func() {
			nodesCIDR := "1.2.3.4/5"
			providerStatus := &runtime.RawExtension{
				Raw: []byte("foo"),
			}

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Infrastructure) error {
					obj.Status.LastError = nil
					obj.ObjectMeta.Annotations = map[string]string{}
					obj.Status.LastOperation = &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateSucceeded,
					}
					obj.Status.NodesCIDR = &nodesCIDR
					obj.Status.ProviderStatus = providerStatus
					return nil
				})

			Expect(deployWaiter.Wait(ctx)).To(Succeed())
			Expect(deployWaiter.ProviderStatus()).To(Equal(providerStatus))
			Expect(deployWaiter.NodesCIDR()).To(PointTo(Equal(nodesCIDR)))
		})

		It("should poll until it's ready", func() {
			waiter.MaxAttempts = 2

			var (
				description    = "some error"
				nodesCIDR      = "1.2.3.4/5"
				providerStatus = &runtime.RawExtension{
					Raw: []byte("foo"),
				}
			)

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Infrastructure) error {
					obj.Status.LastError = &gardencorev1beta1.LastError{
						Description: description,
					}
					return nil
				})

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Infrastructure) error {
					obj.Status.LastError = nil
					obj.ObjectMeta.Annotations = map[string]string{}
					obj.Status.LastOperation = &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateSucceeded,
					}
					obj.Status.NodesCIDR = &nodesCIDR
					obj.Status.ProviderStatus = providerStatus
					return nil
				})

			Expect(deployWaiter.Wait(ctx)).To(Succeed())
			Expect(deployWaiter.ProviderStatus()).To(Equal(providerStatus))
			Expect(deployWaiter.NodesCIDR()).To(PointTo(Equal(nodesCIDR)))
		})

		It("should poll until it times out", func() {
			waiter.MaxAttempts = 3

			description := "some error"

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Infrastructure) error {
					obj.Status.LastError = &gardencorev1beta1.LastError{
						Description: description,
					}
					return nil
				}).
				AnyTimes()

			Expect(deployWaiter.Wait(ctx)).To(MatchError(ContainSubstring(description)))
		})
	})

	Describe("#Destroy", func() {
		It("should not return error when it's not found", func() {
			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), infra).
				Return(apierrors.NewNotFound(schema.GroupResource{}, name))
			c.
				EXPECT().
				Delete(ctx, infra).
				Return(apierrors.NewNotFound(schema.GroupResource{}, name))

			Expect(deployWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should not return error when it's deleted successfully", func() {
			defer test.WithVars(
				&common.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			infraCopy := infra.DeepCopy()
			infraCopy.Annotations = map[string]string{
				common.ConfirmationDeletion:        "true",
				v1beta1constants.GardenerTimestamp: now.UTC().String(),
			}

			c.
				EXPECT().
				Get(ctx, kutil.Key(namespace, name), infra)
			c.
				EXPECT().
				Update(ctx, infraCopy)
			c.
				EXPECT().
				Delete(ctx, infraCopy)

			Expect(deployWaiter.Destroy(ctx)).To(Succeed())
		})

		It("should return error when it's not deleted successfully", func() {
			defer test.WithVars(
				&common.TimeNow, mockNow.Do,
			)()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			infraCopy := infra.DeepCopy()
			infraCopy.Annotations = map[string]string{
				common.ConfirmationDeletion:        "true",
				v1beta1constants.GardenerTimestamp: now.UTC().String(),
			}
			fakeErr := errors.New("some random error")

			c.
				EXPECT().
				Get(ctx, kutil.Key(namespace, name), infra)
			c.
				EXPECT().
				Update(ctx, infraCopy)
			c.
				EXPECT().
				Delete(ctx, infraCopy).
				Return(fakeErr)

			Expect(deployWaiter.Destroy(ctx)).To(MatchError(ContainSubstring(fakeErr.Error())))
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error when it's already removed", func() {
			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{})).
				Return(apierrors.NewNotFound(schema.GroupResource{}, name))

			Expect(deployWaiter.WaitCleanup(ctx)).To(Succeed())
		})

		It("should return error when it's not deleted successfully", func() {
			description := "some error"

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Infrastructure) error {
					obj.Status.LastError = &gardencorev1beta1.LastError{
						Description: description,
					}
					return nil
				})

			Expect(deployWaiter.WaitCleanup(ctx)).To(MatchError(ContainSubstring(description)))
		})

		It("should poll until it's removed", func() {
			waiter.MaxAttempts = 2
			description := "some error"

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Infrastructure) error {
					obj.Status.LastError = &gardencorev1beta1.LastError{
						Description: description,
					}
					return nil
				})
			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{})).
				Return(apierrors.NewNotFound(schema.GroupResource{}, name))

			Expect(deployWaiter.WaitCleanup(ctx)).To(Succeed())
		})

		It("should poll until it times out", func() {
			waiter.MaxAttempts = 3
			description := "some error"

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), &extensionsv1alpha1.Infrastructure{}).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *extensionsv1alpha1.Infrastructure) error {
					obj.Status.LastError = &gardencorev1beta1.LastError{
						Description: description,
					}
					return nil
				}).
				AnyTimes()

			Expect(deployWaiter.WaitCleanup(ctx)).To(MatchError(ContainSubstring(description)))
		})

		It("should return unexpected errors", func() {
			waiter.MaxAttempts = 2

			fakeErr := errors.New("fake")

			c.
				EXPECT().
				Get(gomock.Any(), kutil.Key(namespace, name), gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{})).
				Return(fakeErr)

			Expect(deployWaiter.WaitCleanup(ctx)).To(MatchError(ContainSubstring(fakeErr.Error())))
		})
	})
})
