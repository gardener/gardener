// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package client

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CleanOption is a configuration that modifies options for a clean operation.
type CleanOption interface {
	ApplyToClean(*CleanOptions)
}

// TolerateErrorFunc is a function for tolerating errors.
type TolerateErrorFunc func(err error) bool

// CleanOptions are options to clean certain resources.
// If FinalizeGracePeriodSeconds is set, the finalizers of the resources are removed if the resources still
// exist after their targeted termination date plus the FinalizeGracePeriodSeconds amount.
type CleanOptions struct {
	ListOptions                []client.ListOption
	DeleteOptions              []client.DeleteOption
	FinalizeGracePeriodSeconds *int64
	ErrorToleration            []TolerateErrorFunc
}

var _ CleanOption = &CleanOptions{}

// ApplyToClean implements CleanOption for CleanOptions.
func (o *CleanOptions) ApplyToClean(co *CleanOptions) {
	if o.ListOptions != nil {
		co.ListOptions = o.ListOptions
	}
	if o.DeleteOptions != nil {
		co.DeleteOptions = o.DeleteOptions
	}
	if o.FinalizeGracePeriodSeconds != nil {
		co.FinalizeGracePeriodSeconds = o.FinalizeGracePeriodSeconds
	}
	if o.ErrorToleration != nil {
		co.ErrorToleration = o.ErrorToleration
	}
}

// ApplyOptions applies the OptFuncs to the CleanOptions.
func (o *CleanOptions) ApplyOptions(opts []CleanOption) *CleanOptions {
	for _, opt := range opts {
		opt.ApplyToClean(o)
	}
	return o
}

// ListWith uses the given list options for a clean operation.
type ListWith []client.ListOption

// ApplyToClean specifies list options for a clean operation.
func (d ListWith) ApplyToClean(opts *CleanOptions) {
	opts.ListOptions = append(opts.ListOptions, d...)
}

// DeleteWith uses the given delete options for a clean operation.
type DeleteWith []client.DeleteOption

// ApplyToClean specifies deletion options for a clean operation.
func (d DeleteWith) ApplyToClean(opts *CleanOptions) {
	opts.DeleteOptions = append(opts.DeleteOptions, d...)
}

// FinalizeGracePeriodSeconds specifies that a resource shall be finalized if it's been deleting
// without being gone beyond the deletion timestamp for the given seconds.
type FinalizeGracePeriodSeconds int64

// ApplyToClean specifies a finalize grace period for a clean operation.
func (s FinalizeGracePeriodSeconds) ApplyToClean(opts *CleanOptions) {
	secs := int64(s)
	opts.FinalizeGracePeriodSeconds = &secs
}

// TolerateErrors uses the given toleration funcs for a clean operation.
type TolerateErrors []TolerateErrorFunc

// ApplyToClean specifies a errors to be tolerated for a clean operation.
func (m TolerateErrors) ApplyToClean(opts *CleanOptions) {
	opts.ErrorToleration = append(opts.ErrorToleration, m...)
}
