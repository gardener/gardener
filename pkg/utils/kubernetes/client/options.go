// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
