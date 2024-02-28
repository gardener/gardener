// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package hostnamecheck_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/nodeagent"
	. "github.com/gardener/gardener/pkg/nodeagent/controller/hostnamecheck"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx           = context.Background()
		cancelContext cancel
		reconciler    *Reconciler
	)

	BeforeEach(func() {
		cancelContext = cancel{}
		reconciler = &Reconciler{CancelContext: cancelContext.Cancel}
	})

	Describe("#Reconcile", func() {
		It("should do nothing because hostname did not change", func() {
			hostName, err := nodeagent.GetHostName()
			Expect(err).NotTo(HaveOccurred())
			reconciler.HostName = hostName

			result, err := reconciler.Reconcile(ctx, reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{RequeueAfter: 30 * time.Second}))
			Expect(cancelContext.called).To(BeFalse())
		})

		It("should cancel the context because hostname changed", func() {
			reconciler.HostName = "foobartest"

			result, err := reconciler.Reconcile(ctx, reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(cancelContext.called).To(BeTrue())
		})
	})
})

type cancel struct {
	called bool
}

func (c *cancel) Cancel() {
	c.called = true
}
