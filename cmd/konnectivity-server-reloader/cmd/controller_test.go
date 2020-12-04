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

package cmd

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/utils/pointer"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller tests")
}

var _ = Describe("controller", func() {
	var (
		c          *controller
		ctx        context.Context
		cancelFunc context.CancelFunc
		deploy     *appsv1.Deployment
		getErr     error
	)

	BeforeEach(func() {
		deploy = &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: pointer.Int32Ptr(40),
			},
		}
		getErr = nil
		c = &controller{
			opts: &options{
				command:        []string{"sleep"},
				namespace:      "foo",
				deploymentName: "bar",
			},
			actualCount: 0,
			lastCommand: nil,
			getter:      func(name string) (*appsv1.Deployment, error) { return deploy, getErr },
		}
		ctx, cancelFunc = context.WithCancel(context.Background())
	})

	Context("get of deployment succeeds", func() {
		AfterEach(func() {
			cancelFunc()

			Eventually(func() error {
				_, err := c.lastCommand.Process.Wait()
				// when the process is killed it by the context above
				// Wait should return error
				return err
			}, time.Millisecond*40, time.Millisecond).Should(HaveOccurred(), "process should be killed by cancelFunc")
		})

		It("starts a sleep process", func() {
			Expect(c.reconcile(ctx)).ToNot(HaveOccurred())

			Expect(c.lastCommand).NotTo(BeNil())
			Expect(c.lastCommand.Args).To(ConsistOf("sleep", "40"))
		})

		It("restarts a sleep process when replica count is changed", func() {
			Expect(c.reconcile(ctx)).ToNot(HaveOccurred())

			Expect(c.lastCommand).NotTo(BeNil())
			Expect(c.lastCommand.Args).To(ConsistOf("sleep", "40"))

			oldProcess := c.lastCommand

			deploy.Spec.Replicas = pointer.Int32Ptr(60)
			Expect(c.reconcile(ctx)).ToNot(HaveOccurred())

			Eventually(func() error {
				_, err := oldProcess.Process.Wait()
				return err
			}, time.Millisecond*40, time.Millisecond).Should(HaveOccurred(), "old process should be interupted")

			Expect(c.lastCommand).NotTo(BeNil())
			Expect(c.lastCommand.Args).To(ConsistOf("sleep", "60"))
		})

		It("do not restart a sleep process when replica count is not changed", func() {
			Expect(c.reconcile(ctx)).ToNot(HaveOccurred())

			Expect(c.lastCommand).NotTo(BeNil())
			Expect(c.lastCommand.Args).To(ConsistOf("sleep", "40"))

			oldProcess := c.lastCommand

			Expect(c.reconcile(ctx)).ToNot(HaveOccurred())

			Expect(c.lastCommand).NotTo(BeNil())
			Expect(c.lastCommand).To(BeIdenticalTo(oldProcess), "old process must not be changed")
			Expect(c.lastCommand.Args).To(ConsistOf("sleep", "40"))
		})

		It("do not restart a process that was killed after reconcile", func() {
			Expect(c.reconcile(ctx)).ToNot(HaveOccurred())

			Expect(c.lastCommand).NotTo(BeNil())
			Expect(c.lastCommand.Args).To(ConsistOf("sleep", "40"))

			Expect(c.lastCommand.Process.Signal(os.Interrupt)).NotTo(HaveOccurred())

			Expect(c.reconcile(ctx)).ToNot(HaveOccurred())

			Expect(c.lastCommand).NotTo(BeNil())
			Expect(c.lastCommand.Args).To(ConsistOf("sleep", "40"))

			Eventually(func() error {
				_, err := c.lastCommand.Process.Wait()
				return err
			}, time.Millisecond*40, time.Millisecond).Should(HaveOccurred(), "process should be started again")
		})
	})

	Context("get of deployment fails", func() {
		BeforeEach(func() {
			getErr = errors.New("some error")
		})

		It("returns error", func() {
			Expect(c.reconcile(ctx)).Should(HaveOccurred())

			Expect(c.lastCommand).To(BeNil())
		})
	})
})
