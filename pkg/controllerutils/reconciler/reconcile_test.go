// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reconciler_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/pkg/controllerutils/reconciler"
)

var _ = Describe("Reconcile", func() {
	var (
		fakeErr           = errors.New("fake")
		cause             = errors.New("cause")
		requeueAfter      = time.Hour
		requeueAfterError = &RequeueAfterError{Cause: cause, RequeueAfter: requeueAfter}
	)

	Describe("#ReconcileErr", func() {
		It("should return the correct result if it's no RequeueAfterError", func() {
			res, err := ReconcileErr(fakeErr)
			Expect(err).To(MatchError(fakeErr))
			Expect(res).To(Equal(reconcile.Result{}))
		})

		It("should return the correct result if it's a RequeueAfterError", func() {
			res, err := ReconcileErr(requeueAfterError)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(Equal(reconcile.Result{Requeue: true, RequeueAfter: requeueAfter}))
		})
	})

	Describe("#ReconcileErrCause", func() {
		It("should return the correct result if it's no RequeueAfterError", func() {
			Expect(ReconcileErrCause(fakeErr)).To(MatchError(fakeErr))
		})

		It("should return the correct result if it's a RequeueAfterError", func() {
			Expect(ReconcileErrCause(requeueAfterError)).To(Equal(cause))
		})
	})

	Describe("#ReconcileErrCauseOrErr", func() {
		It("should return the correct result if it's no RequeueAfterError", func() {
			Expect(ReconcileErrCauseOrErr(fakeErr)).To(MatchError(fakeErr))
		})

		It("should return the correct result if it's a RequeueAfterError", func() {
			Expect(ReconcileErrCauseOrErr(requeueAfterError)).To(Equal(cause))
		})
	})
})
