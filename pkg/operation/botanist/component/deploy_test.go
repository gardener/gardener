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

package component_test

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/operation/botanist/component"
	mockcomponent "github.com/gardener/gardener/pkg/operation/botanist/component/mock"
)

var _ = Describe("Helper functions", func() {
	var (
		ctrl                         *gomock.Controller
		deployer1, deployer2         *mockcomponent.MockDeployer
		deployWaiter1, deployWaiter2 *mockcomponent.MockDeployWaiter
		err                          = fmt.Errorf("some error")
		ctx                          = context.TODO()
		deployer                     Deployer
		waiter                       DeployWaiter
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		deployer1 = mockcomponent.NewMockDeployer(ctrl)
		deployer2 = mockcomponent.NewMockDeployer(ctrl)
		deployWaiter1 = mockcomponent.NewMockDeployWaiter(ctrl)
		deployWaiter2 = mockcomponent.NewMockDeployWaiter(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#OpDestroy", func() {
		Context("when Deployer is nil", func() {
			JustBeforeEach(func() {
				deployer = OpDestroy(nil)
			})

			It("should do nothing when called Deploy", func() {
				Expect(deployer.Deploy(ctx)).To(Succeed())
			})

			It("should do nothing when called Destroy", func() {
				Expect(deployer.Destroy(ctx)).To(Succeed())
			})
		})

		Context("when Deployer is not nil", func() {
			JustBeforeEach(func() {
				deployer = OpDestroy(deployWaiter1)
				deployWaiter1.EXPECT().Destroy(ctx).Return(err)
			})

			It("error is returned when calling Deploy", func() {
				Expect(deployer.Deploy(ctx)).To(MatchError(err))
			})

			It("error is returned when calling Destroy", func() {
				Expect(deployer.Destroy(ctx)).To(MatchError(err))
			})
		})

		It("succeeds when multiple Deployers are passed", func() {
			deployer = OpDestroy(deployer1, deployer2)

			gomock.InOrder(
				deployer1.EXPECT().Destroy(ctx),
				deployer2.EXPECT().Destroy(ctx),
			)

			Expect(deployer.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#OpWait", func() {
		Context("when DeployWaiter is nil", func() {
			JustBeforeEach(func() {
				waiter = OpWait(nil)
			})

			It("should do nothing when called Deploy", func() {
				Expect(waiter.Deploy(ctx)).To(Succeed())
			})

			It("should do nothing when called Destroy", func() {
				Expect(waiter.Destroy(ctx)).To(Succeed())
			})
		})

		Context("when DeployWaiter is not nil", func() {
			JustBeforeEach(func() {
				waiter = OpWait(deployWaiter1)
			})

			Context("error is returned when calling Deploy", func() {
				It("when underlying Deploy fails", func() {
					deployWaiter1.EXPECT().Deploy(ctx).Return(err)

					Expect(waiter.Deploy(ctx)).To(MatchError(err))
				})

				It("when underlying Wait fails", func() {
					gomock.InOrder(
						deployWaiter1.EXPECT().Deploy(ctx),
						deployWaiter1.EXPECT().Wait(ctx).Return(err),
					)

					Expect(waiter.Deploy(ctx)).To(MatchError(err))
				})
			})

			Context("error is returned when calling Destroy", func() {
				It("when underlying Destroy fails", func() {
					deployWaiter1.EXPECT().Destroy(ctx).Return(err)

					Expect(waiter.Destroy(ctx)).To(MatchError(err))
				})

				It("when underlying WaitCleanup fails", func() {
					gomock.InOrder(
						deployWaiter1.EXPECT().Destroy(ctx),
						deployWaiter1.EXPECT().WaitCleanup(ctx).Return(err),
					)

					Expect(waiter.Destroy(ctx)).To(MatchError(err))
				})
			})

			It("no error is returned when calling Deploy", func() {
				gomock.InOrder(
					deployWaiter1.EXPECT().Deploy(ctx),
					deployWaiter1.EXPECT().Wait(ctx),
				)

				Expect(waiter.Deploy(ctx)).To(Succeed())
			})

			It("no error is returned when calling Destroy", func() {
				gomock.InOrder(
					deployWaiter1.EXPECT().Destroy(ctx),
					deployWaiter1.EXPECT().WaitCleanup(ctx),
				)

				Expect(waiter.Destroy(ctx)).To(Succeed())
			})
		})

		It("succeeds when multiple DeployWaiter are passed", func() {
			waiter = OpWait(deployWaiter1, deployWaiter2)

			gomock.InOrder(
				deployWaiter1.EXPECT().Destroy(ctx),
				deployWaiter1.EXPECT().WaitCleanup(ctx),
				deployWaiter2.EXPECT().Destroy(ctx),
				deployWaiter2.EXPECT().WaitCleanup(ctx),
			)

			Expect(waiter.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#OpDestroyAndWait", func() {
		Context("when DeployWaiter is nil", func() {
			JustBeforeEach(func() {
				waiter = OpDestroyAndWait(nil)
			})

			It("should do nothing when called Deploy", func() {
				Expect(waiter.Deploy(ctx)).To(Succeed())
			})

			It("should do nothing when called Destroy", func() {
				Expect(waiter.Destroy(ctx)).To(Succeed())
			})
		})

		Context("when DeployWaiter is not nil", func() {
			JustBeforeEach(func() {
				waiter = OpDestroyAndWait(deployWaiter1)
			})

			Context("error is returned when calling Deploy", func() {
				It("when underlying Destroy fails", func() {
					deployWaiter1.EXPECT().Destroy(ctx).Return(err)

					Expect(waiter.Deploy(ctx)).To(MatchError(err))
				})

				It("when underlying WaitCleanup fails", func() {
					gomock.InOrder(
						deployWaiter1.EXPECT().Destroy(ctx),
						deployWaiter1.EXPECT().WaitCleanup(ctx).Return(err),
					)

					Expect(waiter.Deploy(ctx)).To(MatchError(err))
				})
			})

			Context("error is returned when calling Destroy", func() {
				It("when underlying Destroy fails", func() {
					deployWaiter1.EXPECT().Destroy(ctx).Return(err)

					Expect(waiter.Destroy(ctx)).To(MatchError(err))
				})

				It("when underlying WaitCleanup fails", func() {
					gomock.InOrder(
						deployWaiter1.EXPECT().Destroy(ctx),
						deployWaiter1.EXPECT().WaitCleanup(ctx).Return(err),
					)

					Expect(waiter.Destroy(ctx)).To(MatchError(err))
				})
			})

			It("no error is returned when calling Deploy", func() {
				gomock.InOrder(
					deployWaiter1.EXPECT().Destroy(ctx),
					deployWaiter1.EXPECT().WaitCleanup(ctx),
				)

				Expect(waiter.Deploy(ctx)).To(Succeed())
			})

			It("no error is returned when calling Destroy", func() {
				gomock.InOrder(
					deployWaiter1.EXPECT().Destroy(ctx),
					deployWaiter1.EXPECT().WaitCleanup(ctx),
				)

				Expect(waiter.Destroy(ctx)).To(Succeed())
			})
		})

		It("succeeds when multiple DeployWaiter are passed", func() {
			waiter = OpDestroyAndWait(deployWaiter1, deployWaiter2)

			gomock.InOrder(
				deployWaiter1.EXPECT().Destroy(ctx),
				deployWaiter1.EXPECT().WaitCleanup(ctx),
				deployWaiter2.EXPECT().Destroy(ctx),
				deployWaiter2.EXPECT().WaitCleanup(ctx),
			)

			Expect(waiter.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#OpDestroyWithoutWait", func() {
		Context("when DeployWaiter is nil", func() {
			JustBeforeEach(func() {
				waiter = OpDestroyWithoutWait(nil)
			})

			It("should do nothing when called Deploy", func() {
				Expect(waiter.Deploy(ctx)).To(Succeed())
			})

			It("should do nothing when called Destroy", func() {
				Expect(waiter.Destroy(ctx)).To(Succeed())
			})
		})

		Context("when DeployWaiter is not nil", func() {
			JustBeforeEach(func() {
				waiter = OpDestroyWithoutWait(deployWaiter1)
			})

			Context("error is returned when calling Deploy", func() {
				It("when underlying Destroy fails", func() {
					deployWaiter1.EXPECT().Destroy(ctx).Return(err)

					Expect(waiter.Deploy(ctx)).To(MatchError(err))
				})
			})

			Context("error is returned when calling Destroy", func() {
				It("when underlying Destroy fails", func() {
					deployWaiter1.EXPECT().Destroy(ctx).Return(err)

					Expect(waiter.Destroy(ctx)).To(MatchError(err))
				})
			})

			It("no error is returned when calling Deploy", func() {
				deployWaiter1.EXPECT().Destroy(ctx)

				Expect(waiter.Deploy(ctx)).To(Succeed())
			})

			It("no error is returned when calling Destroy", func() {
				deployWaiter1.EXPECT().Destroy(ctx)

				Expect(waiter.Destroy(ctx)).To(Succeed())
			})
		})

		It("succeeds when multiple DeployWaiter are passed", func() {
			waiter = OpDestroyWithoutWait(deployWaiter1, deployWaiter2)

			gomock.InOrder(
				deployWaiter1.EXPECT().Destroy(ctx),
				deployWaiter2.EXPECT().Destroy(ctx),
			)

			Expect(waiter.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#NoOp", func() {
		BeforeEach(func() {
			waiter = NoOp()
		})

		It("should do nothing", func() {
			Expect(waiter.Deploy(ctx)).To(Succeed())
			Expect(waiter.Destroy(ctx)).To(Succeed())
			Expect(waiter.Wait(ctx)).To(Succeed())
			Expect(waiter.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
