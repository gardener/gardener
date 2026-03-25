// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"encoding/json"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UnmarshalJSON implements custom JSON unmarshaling for TokenRequestorWorkloadIdentityControllerConfiguration.
// It handles backward compatibility for the TokenExpirationDuration field which was changed from time.Duration
// (serialized as nanoseconds integer) to metav1.Duration (serialized as string like "6h").
// TODO(oliver-goetz): Remove this custom unmarshaling logic once Gardener v1.142 has been released.
func (c *TokenRequestorWorkloadIdentityControllerConfiguration) UnmarshalJSON(data []byte) error {
	// Define a type alias to avoid infinite recursion when calling json.Unmarshal.
	type alias TokenRequestorWorkloadIdentityControllerConfiguration

	// First, try to unmarshal with the new format (metav1.Duration as string).
	aux := &alias{}
	if err := json.Unmarshal(data, aux); err == nil {
		*c = TokenRequestorWorkloadIdentityControllerConfiguration(*aux)
		return nil
	}

	// If that fails, try the old format with TokenExpirationDuration as a number (nanoseconds).
	type oldFormat struct {
		ConcurrentSyncs         *int   `json:"concurrentSyncs,omitempty"`
		TokenExpirationDuration *int64 `json:"tokenExpirationDuration,omitempty"`
	}

	old := &oldFormat{}
	if err := json.Unmarshal(data, old); err != nil {
		return err
	}

	c.ConcurrentSyncs = old.ConcurrentSyncs
	if old.TokenExpirationDuration != nil {
		c.TokenExpirationDuration = &metav1.Duration{Duration: time.Duration(*old.TokenExpirationDuration)}
	}

	return nil
}
