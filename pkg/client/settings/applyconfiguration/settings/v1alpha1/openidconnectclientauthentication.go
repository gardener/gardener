/*
Copyright SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1alpha1

// OpenIDConnectClientAuthenticationApplyConfiguration represents an declarative configuration of the OpenIDConnectClientAuthentication type for use
// with apply.
type OpenIDConnectClientAuthenticationApplyConfiguration struct {
	Secret      *string           `json:"secret,omitempty"`
	ExtraConfig map[string]string `json:"extraConfig,omitempty"`
}

// OpenIDConnectClientAuthenticationApplyConfiguration constructs an declarative configuration of the OpenIDConnectClientAuthentication type for use with
// apply.
func OpenIDConnectClientAuthentication() *OpenIDConnectClientAuthenticationApplyConfiguration {
	return &OpenIDConnectClientAuthenticationApplyConfiguration{}
}

// WithSecret sets the Secret field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Secret field is set to the value of the last call.
func (b *OpenIDConnectClientAuthenticationApplyConfiguration) WithSecret(value string) *OpenIDConnectClientAuthenticationApplyConfiguration {
	b.Secret = &value
	return b
}

// WithExtraConfig puts the entries into the ExtraConfig field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, the entries provided by each call will be put on the ExtraConfig field,
// overwriting an existing map entries in ExtraConfig field with the same key.
func (b *OpenIDConnectClientAuthenticationApplyConfiguration) WithExtraConfig(entries map[string]string) *OpenIDConnectClientAuthenticationApplyConfiguration {
	if b.ExtraConfig == nil && len(entries) > 0 {
		b.ExtraConfig = make(map[string]string, len(entries))
	}
	for k, v := range entries {
		b.ExtraConfig[k] = v
	}
	return b
}
