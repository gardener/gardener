// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ApplyOption is some configuration that modifies options for a apply request.
type ApplyOption interface {
	// MutateApplyOptions applies this configuration to the given apply options.
	MutateApplyOptions(opts *ApplyOptions)
}

// ApplyOptions contains options for apply requests
type ApplyOptions struct {
	// Values to pass to chart.
	Values any

	// Additional MergeFunctions.
	MergeFuncs MergeFuncs

	// Forces the namespace for chart objects when applying the chart, this is because sometimes native chart
	// objects do not come with a Release.Namespace option and leave the namespace field empty
	ForceNamespace bool
}

// Values applies values to ApplyOptions or DeleteOptions.
var Values = func(values any) ValueOption { return &withValue{values} }

type withValue struct {
	values any
}

func (v withValue) MutateApplyOptions(opts *ApplyOptions) {
	opts.Values = v.values
}

func (v withValue) MutateDeleteOptions(opts *DeleteOptions) {
	opts.Values = v.values
}

// MergeFuncs can be used modify the default merge functions for ApplyOptions:
//
//	Apply(ctx, "chart", "my-ns", "my-release", MergeFuncs{
//			corev1.SchemeGroupVersion.WithKind("Service").GroupKind(): func(newObj, oldObj *unstructured.Unstructured) {
//				newObj.SetAnnotations(map[string]string{"foo":"bar"})
//			}
//	})
type MergeFuncs map[schema.GroupKind]MergeFunc

// MutateApplyOptions applies this configuration to the given apply options.
func (m MergeFuncs) MutateApplyOptions(opts *ApplyOptions) {
	opts.MergeFuncs = m
}

// ForceNamespace can be used for native chart objects do not come with
// a Release.Namespace option and leave the namespace field empty.
var ForceNamespace = forceNamespace{}

type forceNamespace struct{}

func (forceNamespace) MutateApplyOptions(opts *ApplyOptions) {
	opts.ForceNamespace = true
}

func (forceNamespace) MutateDeleteOptions(opts *DeleteOptions) {
	opts.ForceNamespace = true
}

// ValueOption contains value options for Apply and Delete.
type ValueOption interface {
	ApplyOption
	DeleteOption
}

// DeleteOption is some configuration that modifies options for a delete request.
type DeleteOption interface {
	// MutateDeleteOptions applies this configuration to the given delete options.
	MutateDeleteOptions(opts *DeleteOptions)
}

// DeleteOptions contains options for delete requests
type DeleteOptions struct {
	// Values to pass to chart.
	Values any

	// Forces the namespace for chart objects when applying the chart, this is because sometimes native chart
	// objects do not come with a Release.Namespace option and leave the namespace field empty
	ForceNamespace bool

	// TolerateErrorFuncs are functions for which errors are tolerated.
	TolerateErrorFuncs []TolerateErrorFunc
}

// TolerateErrorFunc is a function for which err is tolerated.
type TolerateErrorFunc func(err error) bool

// MutateDeleteOptions applies this configuration to the given delete options.
func (t TolerateErrorFunc) MutateDeleteOptions(opts *DeleteOptions) {
	if opts.TolerateErrorFuncs == nil {
		opts.TolerateErrorFuncs = []TolerateErrorFunc{}
	}

	opts.TolerateErrorFuncs = append(opts.TolerateErrorFuncs, t)
}

// MutateDeleteManifestOptions applies this configuration to the given delete manifest options.
func (t TolerateErrorFunc) MutateDeleteManifestOptions(opts *DeleteManifestOptions) {
	if opts.TolerateErrorFuncs == nil {
		opts.TolerateErrorFuncs = []TolerateErrorFunc{}
	}

	opts.TolerateErrorFuncs = append(opts.TolerateErrorFuncs, t)
}
