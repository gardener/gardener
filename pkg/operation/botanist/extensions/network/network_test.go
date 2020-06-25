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

package network_test

import (
	"context"
	"fmt"
	"net"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/network"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/test/gomega"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("#Network", func() {
	const (
		networkNs          = "test-namespace"
		networkName        = "test-deploy"
		networkType        = "calico"
		networkPodIp       = "100.96.0.0"
		networkPodMask     = 11
		networkServiceIp   = "100.64.0.0"
		networkServiceMask = 13
	)
	var (
		ctrl *gomock.Controller

		ctx              context.Context
		c                client.Client
		expected         *extensionsv1alpha1.Network
		values           *network.Values
		log              *logrus.Entry
		defaultDepWaiter component.DeployWaiter

		mockNow *mocktime.MockNow
		now     time.Time

		networkPodCIDR     = fmt.Sprintf("%s/%d", networkPodIp, networkPodMask)
		networkServiceCIDR = fmt.Sprintf("%s/%d", networkServiceIp, networkServiceMask)
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		mockNow = mocktime.NewMockNow(ctrl)

		ctx = context.TODO()
		log = logrus.NewEntry(logger.NewNopLogger())

		s := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(s)).NotTo(HaveOccurred())

		c = fake.NewFakeClientWithScheme(s)

		podCIDR := net.IPNet{
			IP:   net.ParseIP(networkPodIp),
			Mask: net.CIDRMask(networkPodMask, 32),
		}
		serviceCIDR := net.IPNet{
			IP:   net.ParseIP(networkServiceIp),
			Mask: net.CIDRMask(networkServiceMask, 32),
		}

		values = &network.Values{
			Name:                                    "test-deploy",
			Namespace:                               networkNs,
			IsInRestorePhaseOfControlPlaneMigration: false,
			Type:                                    networkType,
			ProviderConfig:                          nil,
			PodCIDR:                                 &podCIDR,
			ServiceCIDR:                             &serviceCIDR,
		}

		expected = &extensionsv1alpha1.Network{
			ObjectMeta: metav1.ObjectMeta{
				Name:      networkName,
				Namespace: networkNs,
				Annotations: map[string]string{
					v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
					v1beta1constants.GardenerTimestamp: now.UTC().String(),
				},
			},
			Spec: extensionsv1alpha1.NetworkSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type:           networkType,
					ProviderConfig: nil,
				},
				PodCIDR:     networkPodCIDR,
				ServiceCIDR: networkServiceCIDR,
			},
		}

		defaultDepWaiter = network.New(log, c, values, time.Second, 2*time.Second, 3*time.Second)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		DescribeTable("correct Network is created", func(mutator func()) {
			defer test.WithVars(
				&network.TimeNow, mockNow.Do,
			)()

			mutator()
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			Expect(defaultDepWaiter.Deploy(ctx)).ToNot(HaveOccurred())

			actual := &extensionsv1alpha1.Network{}
			err := c.Get(ctx, client.ObjectKey{Name: networkName, Namespace: networkNs}, actual)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(DeepDerivativeEqual(expected))
		},
			Entry("with no modification", func() {}),
			Entry("during restore phase", func() {
				values.IsInRestorePhaseOfControlPlaneMigration = true
				expected.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationWaitForState
			}),
		)
	})

	Describe("#Wait", func() {
		It("should return error when it's not found", func() {
			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred())
		})

		It("should return error when it's not ready", func() {
			expected.Status.LastError = &gardencorev1beta1.LastError{
				Description: "Some error",
			}

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating network succeeds")
			Expect(defaultDepWaiter.Wait(ctx)).To(HaveOccurred(), "network indicates error")
		})

		It("should return no error when is ready", func() {
			expected.Status.LastError = nil
			// remove operation annotation
			expected.ObjectMeta.Annotations = map[string]string{}
			// set last operation
			expected.Status.LastOperation = &gardencorev1beta1.LastOperation{
				State: gardencorev1beta1.LastOperationStateSucceeded,
			}

			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "creating network succeeds")
			Expect(defaultDepWaiter.Wait(ctx)).ToNot(HaveOccurred(), "network is ready, should not return an error")
		})
	})

	Describe("#Destroy", func() {
		It("should not return error when it's not found", func() {
			Expect(defaultDepWaiter.Destroy(ctx)).ToNot(HaveOccurred())
		})

		It("should not return error when it's deleted successfully", func() {
			Expect(c.Create(ctx, expected)).ToNot(HaveOccurred(), "adding pre-existing network succeeds")

			Expect(defaultDepWaiter.Destroy(ctx)).ToNot(HaveOccurred())
		})

		It("should return error when it's not deleted successfully", func() {
			defer test.WithVars(
				&common.TimeNow, mockNow.Do,
			)()

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			expected := extensionsv1alpha1.Network{
				ObjectMeta: metav1.ObjectMeta{
					Name:      networkName,
					Namespace: networkNs,
					Annotations: map[string]string{
						common.ConfirmationDeletion:        "true",
						v1beta1constants.GardenerTimestamp: now.UTC().String(),
					},
				}}

			mc := mockclient.NewMockClient(ctrl)
			// check if the Network exist
			mc.EXPECT().Get(ctx, kutil.Key(networkNs, networkName), gomock.AssignableToTypeOf(&extensionsv1alpha1.Network{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, n *extensionsv1alpha1.Network) error {
				return nil
			})

			// add deletion confirmation and Timestamp annotation
			mc.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Network{})).Return(nil)

			mc.EXPECT().Delete(ctx, &expected).Times(1).Return(fmt.Errorf("some random error"))

			defaultDepWaiter = network.New(log, mc, &network.Values{
				Namespace: networkNs,
				Name:      networkName,
			}, time.Second, 2*time.Second, 3*time.Second)

			err := defaultDepWaiter.Destroy(ctx)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should not return error when it's already removed", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).ToNot(HaveOccurred())
		})
	})
})
