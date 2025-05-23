// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
