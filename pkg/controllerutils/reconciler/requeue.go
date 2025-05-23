// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reconciler

import (
	"fmt"
	"time"
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
