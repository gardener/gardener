// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

import "k8s.io/apimachinery/pkg/runtime/schema"

// ApplyOption is some configuration that modifies options for a apply request.
type ApplyOption interface {
	// MutateApplyOptions applies this configuration to the given apply options.
	MutateApplyOptions(opts *ApplyOptions)
}

// ApplyOptions contains options for apply requests
type ApplyOptions struct {
	// Values to pass to chart.
	Values interface{}

	// Additional MergeFunctions.
	MergeFuncs MergeFuncs

	// Forces the namespace for chart objects when applying the chart, this is because sometimes native chart
	// objects do not come with a Release.Namespace option and leave the namespace field empty
	ForceNamespace bool
}

// Values applies values to ApplyOptions or DeleteOptions.
var Values = func(values interface{}) ValueOption { return &withValue{values} }

type withValue struct {
	values interface{}
}

func (v withValue) MutateApplyOptions(opts *ApplyOptions) {
	opts.Values = v.values
}

func (v withValue) MutateDeleteOptions(opts *DeleteOptions) {
	opts.Values = v.values
}

// MergeFuncs can be used modify the default merge functions for ApplyOptions:
//
// Apply(ctx, "chart", "my-ns", "my-release", MergeFuncs{
// 		corev1.SchemeGroupVersion.WithKind("Service").GroupKind(): func(newObj, oldObj *unstructured.Unstructured) {
// 			newObj.SetAnnotations(map[string]string{"foo":"bar"})
// 		}
// })
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
	Values interface{}

	// Forces the namespace for chart objects when applying the chart, this is because sometimes native chart
	// objects do not come with a Release.Namespace option and leave the namespace field empty
	ForceNamespace bool
}
