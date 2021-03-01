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

package component_test

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/operation/botanist/component"
	mockcomponent "github.com/gardener/gardener/pkg/operation/botanist/component/mock"
)

var _ = Describe("Helper functions", func() {
	var (
		ctrl     *gomock.Controller
		c, c2    *mockcomponent.MockDeployWaiter
		err      = fmt.Errorf("some error")
		ctx      = context.TODO()
		deployer Deployer
		waiter   DeployWaiter
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockcomponent.NewMockDeployWaiter(ctrl)
		c2 = mockcomponent.NewMockDeployWaiter(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
		c.EXPECT().Deploy(ctx).Times(0)
		c.EXPECT().Destroy(ctx).Times(0)
		c.EXPECT().Wait(ctx).Times(0)
		c.EXPECT().WaitCleanup(ctx).Times(0)

		c2.EXPECT().Deploy(ctx).Times(0)
		c2.EXPECT().Destroy(ctx).Times(0)
		c2.EXPECT().Wait(ctx).Times(0)
		c2.EXPECT().WaitCleanup(ctx).Times(0)
	})

	Describe("OpDestroy", func() {
		Context("when DeployWaiter is nil", func() {
			JustBeforeEach(func() {
				waiter = OpDestroy(nil)
			})

			It("should do nothing when called Deploy", func() {
				Expect(waiter.Deploy(ctx)).ToNot(HaveOccurred())
			})
			It("should do nothing when called Destroy", func() {
				Expect(waiter.Destroy(ctx)).ToNot(HaveOccurred())
			})
			It("should do nothing when called Wait", func() {
				Expect(waiter.Wait(ctx)).ToNot(HaveOccurred())
			})
			It("should do nothing when called WaitCleanup", func() {
				Expect(waiter.WaitCleanup(ctx)).ToNot(HaveOccurred())
			})
		})

		Context("when DeployWaiter is not nil", func() {
			JustBeforeEach(func() {
				waiter = OpDestroy(c)
			})

			It("error is returned when calling Deploy", func() {
				c.EXPECT().Destroy(ctx).Return(err)
				Expect(waiter.Deploy(ctx)).To(HaveOccurred())
			})

			It("error is returned when calling Destroy", func() {
				c.EXPECT().Destroy(ctx).Return(err)
				Expect(waiter.Destroy(ctx)).To(HaveOccurred())
			})

			It("no error is returned when calling Wait", func() {
				c.EXPECT().WaitCleanup(ctx).Times(1)
				Expect(waiter.Wait(ctx)).ToNot(HaveOccurred())
			})

			It("no error is returned when calling WaitCleanup", func() {
				c.EXPECT().WaitCleanup(ctx).Times(1)
				Expect(waiter.WaitCleanup(ctx)).ToNot(HaveOccurred())
			})
		})

		It("succeeds when multiple DeployWaiter are passed", func() {
			waiter = OpDestroy(c, c2)
			gomock.InOrder(
				c.EXPECT().Destroy(ctx).Times(1),
				c2.EXPECT().Destroy(ctx).Times(1),
			)

			Expect(waiter.Destroy(ctx)).ToNot(HaveOccurred())
		})
	})

	Describe("OpDestroyAndWait", func() {
		Context("when DeployWaiter is nil", func() {
			JustBeforeEach(func() {
				deployer = OpDestroyAndWait(nil)
			})

			It("should do nothing when called Deploy", func() {
				Expect(deployer.Deploy(ctx)).ToNot(HaveOccurred())
			})
			It("should do nothing when called Destroy", func() {
				Expect(deployer.Destroy(ctx)).ToNot(HaveOccurred())
			})
		})

		Context("when DeployWaiter is not nil", func() {
			JustBeforeEach(func() {
				deployer = OpDestroyAndWait(c)
			})

			Context("error is returned when calling Deploy", func() {
				AfterEach(func() {
					Expect(deployer.Deploy(ctx)).To(HaveOccurred())
				})

				It("when underlying Destroy fails", func() {
					c.EXPECT().Destroy(ctx).Return(err)
				})

				It("when underlying WaitCleanup fails", func() {
					gomock.InOrder(
						c.EXPECT().Destroy(ctx).Times(1),
						c.EXPECT().WaitCleanup(ctx).Return(err),
					)
				})
			})
			Context("error is returned when calling Destroy", func() {
				AfterEach(func() {
					Expect(deployer.Destroy(ctx)).To(HaveOccurred())
				})

				It("when underlying Destroy fails", func() {
					c.EXPECT().Destroy(ctx).Return(err)
				})

				It("when underlying WaitCleanup fails", func() {
					gomock.InOrder(
						c.EXPECT().Destroy(ctx).Times(1),
						c.EXPECT().WaitCleanup(ctx).Return(err),
					)
				})
			})

			It("no error is returned when calling Deploy", func() {
				gomock.InOrder(
					c.EXPECT().Destroy(ctx).Times(1),
					c.EXPECT().WaitCleanup(ctx).Times(1),
				)
				Expect(deployer.Deploy(ctx)).ToNot(HaveOccurred())
			})

			It("no error is returned when calling Destroy", func() {

				gomock.InOrder(
					c.EXPECT().Destroy(ctx).Times(1),
					c.EXPECT().WaitCleanup(ctx).Times(1),
				)
				Expect(deployer.Destroy(ctx)).ToNot(HaveOccurred())
			})
		})

		It("succeeds when multiple DeployWaiter are passed", func() {
			deployer = OpWaiter(c, c2)

			gomock.InOrder(
				c.EXPECT().Destroy(ctx).Times(1),
				c.EXPECT().WaitCleanup(ctx).Times(1),
				c2.EXPECT().Destroy(ctx).Times(1),
				c2.EXPECT().WaitCleanup(ctx).Times(1),
			)

			Expect(deployer.Destroy(ctx)).ToNot(HaveOccurred())
		})
	})

	Describe("OpWaiter", func() {
		Context("when DeployWaiter is nil", func() {
			JustBeforeEach(func() {
				deployer = OpWaiter(nil)
			})

			It("should do nothing when called Deploy", func() {
				Expect(deployer.Deploy(ctx)).ToNot(HaveOccurred())
			})
			It("should do nothing when called Destroy", func() {
				Expect(deployer.Destroy(ctx)).ToNot(HaveOccurred())
			})
		})

		Context("when DeployWaiter is not nil", func() {
			JustBeforeEach(func() {
				deployer = OpWaiter(c)
			})

			Context("error is returned when calling Deploy", func() {
				AfterEach(func() {
					Expect(deployer.Deploy(ctx)).To(HaveOccurred())
				})

				It("when underlying Deploy fails", func() {
					c.EXPECT().Deploy(ctx).Return(err)
				})

				It("when underlying Wait fails", func() {
					gomock.InOrder(
						c.EXPECT().Deploy(ctx).Times(1),
						c.EXPECT().Wait(ctx).Return(err),
					)
				})
			})
			Context("error is returned when calling Destroy", func() {
				AfterEach(func() {
					Expect(deployer.Destroy(ctx)).To(HaveOccurred())
				})

				It("when underlying Destroy fails", func() {
					c.EXPECT().Destroy(ctx).Return(err)
				})

				It("when underlying WaitCleanup fails", func() {
					gomock.InOrder(
						c.EXPECT().Destroy(ctx).Times(1),
						c.EXPECT().WaitCleanup(ctx).Return(err),
					)
				})
			})

			It("no error is returned when calling Deploy", func() {
				gomock.InOrder(
					c.EXPECT().Deploy(ctx).Times(1),
					c.EXPECT().Wait(ctx).Times(1),
				)
				Expect(deployer.Deploy(ctx)).ToNot(HaveOccurred())
			})

			It("no error is returned when calling Destroy", func() {
				gomock.InOrder(
					c.EXPECT().Destroy(ctx).Times(1),
					c.EXPECT().WaitCleanup(ctx).Times(1),
				)
				Expect(deployer.Destroy(ctx)).ToNot(HaveOccurred())
			})
		})

		It("succeeds when multiple DeployWaiter are passed", func() {
			deployer = OpWaiter(c, c2)
			gomock.InOrder(
				c.EXPECT().Destroy(ctx).Times(1),
				c.EXPECT().WaitCleanup(ctx).Times(1),
				c2.EXPECT().Destroy(ctx).Times(1),
				c2.EXPECT().WaitCleanup(ctx).Times(1),
			)
			Expect(deployer.Destroy(ctx)).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("NoOp", func() {
	var (
		ctx = context.TODO()
		dw  DeployWaiter
	)

	BeforeEach(func() {
		dw = NoOp()
	})

	It("should do nothing when callling Deploy", func() {
		Expect(dw.Deploy(ctx)).ToNot(HaveOccurred())
	})

	It("should do nothing when called Destroy", func() {
		Expect(dw.Destroy(ctx)).ToNot(HaveOccurred())
	})

	It("should do nothing when called Wait", func() {
		Expect(dw.Wait(ctx)).ToNot(HaveOccurred())
	})

	It("should do nothing when called WaitCleanup", func() {
		Expect(dw.WaitCleanup(ctx)).ToNot(HaveOccurred())
	})
})
