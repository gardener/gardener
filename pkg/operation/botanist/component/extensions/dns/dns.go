// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dns

import (
	"fmt"
	"time"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// Object is an interface for accessing common fields across DNS objects.
// It is similar to extensionsv1alpha1.Object but special, as the DNS objects don't have the same
// structure as all other extension objects and thus don't implement extensionsv1alpha1.Object.
type Object interface {
	client.Object

	GetObservedGeneration() int64
	SetObservedGeneration(observedGeneration int64)
	GetState() string
	SetState(state string)
	GetMessage() *string
	SetMessage(message *string)
}

// dnsProviderAccessor implements Object for a DNSProvider object.
type dnsProviderAccessor struct {
	*dnsv1alpha1.DNSProvider
}

func (d dnsProviderAccessor) GetObservedGeneration() int64  { return d.Status.ObservedGeneration }
func (d dnsProviderAccessor) SetObservedGeneration(o int64) { d.Status.ObservedGeneration = o }
func (d dnsProviderAccessor) GetState() string              { return d.Status.State }
func (d dnsProviderAccessor) SetState(state string)         { d.Status.State = state }
func (d dnsProviderAccessor) GetMessage() *string           { return d.Status.Message }
func (d dnsProviderAccessor) SetMessage(message *string)    { d.Status.Message = message }

// dnsEntryAccessor implements Object for a DNSEntry object.
type dnsEntryAccessor struct {
	*dnsv1alpha1.DNSEntry
}

func (d dnsEntryAccessor) GetObservedGeneration() int64  { return d.Status.ObservedGeneration }
func (d dnsEntryAccessor) SetObservedGeneration(o int64) { d.Status.ObservedGeneration = o }
func (d dnsEntryAccessor) GetState() string              { return d.Status.State }
func (d dnsEntryAccessor) SetState(state string)         { d.Status.State = state }
func (d dnsEntryAccessor) GetMessage() *string           { return d.Status.Message }
func (d dnsEntryAccessor) SetMessage(message *string)    { d.Status.Message = message }

// Accessor returns an Object implementation for the given obj.
func Accessor(obj client.Object) (Object, error) {
	switch v := obj.(type) {
	case *dnsv1alpha1.DNSProvider:
		return dnsProviderAccessor{v}, nil
	case *dnsv1alpha1.DNSEntry:
		return dnsEntryAccessor{v}, nil
	default:
		return nil, fmt.Errorf("expected either *dnsv1alpha1.DNSProvider or *dnsv1alpha1.DNSEntry but got %T", obj)
	}
}

// CheckDNSObject is similar to health.CheckExtensionObject, but implements the special handling for DNS objects
// as they don't implement extensionsv1alpha1.Object.
func CheckDNSObject(obj client.Object) error {
	dnsObj, err := Accessor(obj)
	if err != nil {
		return err
	}

	generation := dnsObj.GetGeneration()
	observedGeneration := dnsObj.GetObservedGeneration()
	if observedGeneration != generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", observedGeneration, generation)
	}

	if state := dnsObj.GetState(); state != dnsv1alpha1.STATE_READY {
		var err error
		if msg := dnsObj.GetMessage(); msg != nil {
			err = fmt.Errorf("state %s: %s", state, *msg)
		} else {
			err = fmt.Errorf("state %s", state)
		}

		// TODO(timebertt): this should be the other way round: ErrorWithCodes should wrap the errorWithDNSState.
		// DetermineError first needs to be improved to properly wrap the given error, afterwards we can clean up this
		// code here
		if state == dnsv1alpha1.STATE_ERROR || state == dnsv1alpha1.STATE_INVALID {
			// return a retriable error for an Error or Invalid state (independent of the error code detection), which makes
			// WaitUntilObjectReadyWithHealthFunction not treat the error as severe immediately but still surface errors
			// faster, without retrying until the entire timeout is elapsed.
			// This is the same behavior as in other extension components which leverage health.CheckExtensionObject, where
			// ErrorWithCodes is returned if status.lastError is set (no matter if status.lastError.codes contains error codes).
			err = retry.RetriableError(gardencorev1beta1helper.DetermineError(err, ""))
		}
		return &errorWithDNSState{underlying: err, state: state}
	}

	return nil
}

// ErrorWithDNSState is an error annotated with the state of a DNS object.
type ErrorWithDNSState interface {
	error

	// DNSState returns the state of the DNS object this error is about.
	DNSState() string
}

var _ ErrorWithDNSState = (*errorWithDNSState)(nil)

type errorWithDNSState struct {
	underlying error
	state      string
}

// Error returns the error message of the underlying (wrapped) error.
func (e *errorWithDNSState) Error() string {
	return e.underlying.Error()
}

// DNSState returns the state of the DNS object this error is about.
func (e *errorWithDNSState) DNSState() string {
	return e.state
}

// Unwrap returns the underlying (wrapped) error.
func (e *errorWithDNSState) Unwrap() error {
	return e.underlying
}
