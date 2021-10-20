// Copyright (c) 2011 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
