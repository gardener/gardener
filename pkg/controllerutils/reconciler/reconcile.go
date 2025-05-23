// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reconciler

import (
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ReconcileErr returns a reconcile.Result or an error, depending on whether the error is a
// RequeueAfterError or not.
func ReconcileErr(err error) (reconcile.Result, error) {
	if requeueAfter, ok := err.(*RequeueAfterError); ok {
		return reconcile.Result{Requeue: true, RequeueAfter: requeueAfter.RequeueAfter}, nil
	}
	return reconcile.Result{}, err
}

// ReconcileErrCause returns the cause in case the error is an RequeueAfterError. Otherwise,
// it returns the input error.
func ReconcileErrCause(err error) error {
	if requeueAfter, ok := err.(*RequeueAfterError); ok {
		return requeueAfter.Cause
	}
	return err
}

// ReconcileErrCauseOrErr returns the cause of the error or the error if the cause is nil.
func ReconcileErrCauseOrErr(err error) error {
	if cause := ReconcileErrCause(err); cause != nil {
		return cause
	}
	return err
}
