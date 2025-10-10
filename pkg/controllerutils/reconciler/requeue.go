// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reconciler

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// RequeueAfterError is an error that indicates that an actuator wants a reconcile operation
// to be requeued again after RequeueAfter has passed.
type RequeueAfterError struct {
	// Cause is an optional cause that may be returned together with a time for requeuing.
	Cause error
	// RequeueAfter is the duration after which the request should be enqueued again.
	RequeueAfter time.Duration
}

func (e *RequeueAfterError) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("requeue in %s", e.RequeueAfter)
	}

	return fmt.Sprintf("requeue in %s due to %+v", e.RequeueAfter, e.Cause)
}

// RequeueReconciler defines a reconciler interface that offers an alternative way
// to requeue apart from returning errors or requeuing after a specific time.
type RequeueReconciler interface {
	// Reconcile performs the reconciliation logic and returns whether reconciliation should be requeued a reconcile.Result and an error.
	Reconcile(ctx context.Context, request reconcile.Request) (requeue bool, result reconcile.Result, error error)
}

// RateLimitedRequeueReconcilerAdapter adapts a RequeueReconciler to work with controller-runtime's
// standard reconcile.Reconciler interface, adding rate limiting for requeue operations.
func RateLimitedRequeueReconcilerAdapter(requeuer RequeueReconciler, requeueRateLimiter workqueue.TypedRateLimiter[reconcile.Request]) reconcile.Reconciler {
	return reconcile.Func(
		func(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
			requeue, result, err := requeuer.Reconcile(ctx, request)

			switch {
			case err != nil:
				return reconcile.Result{}, err
			case result.RequeueAfter > 0:
				requeueRateLimiter.Forget(request)
				return result, nil
			case requeue:
				return reconcile.Result{RequeueAfter: requeueRateLimiter.When(request)}, nil
			default:
				requeueRateLimiter.Forget(request)
				return result, nil
			}
		},
	)
}
