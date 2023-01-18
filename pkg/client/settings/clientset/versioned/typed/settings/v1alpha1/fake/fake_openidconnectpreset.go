/*
Copyright (c) SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"

	v1alpha1 "github.com/gardener/gardener/pkg/apis/settings/v1alpha1"
)

// FakeOpenIDConnectPresets implements OpenIDConnectPresetInterface
type FakeOpenIDConnectPresets struct {
	Fake *FakeSettingsV1alpha1
	ns   string
}

var openidconnectpresetsResource = schema.GroupVersionResource{Group: "settings.gardener.cloud", Version: "v1alpha1", Resource: "openidconnectpresets"}

var openidconnectpresetsKind = schema.GroupVersionKind{Group: "settings.gardener.cloud", Version: "v1alpha1", Kind: "OpenIDConnectPreset"}

// Get takes name of the openIDConnectPreset, and returns the corresponding openIDConnectPreset object, and an error if there is any.
func (c *FakeOpenIDConnectPresets) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.OpenIDConnectPreset, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(openidconnectpresetsResource, c.ns, name), &v1alpha1.OpenIDConnectPreset{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.OpenIDConnectPreset), err
}

// List takes label and field selectors, and returns the list of OpenIDConnectPresets that match those selectors.
func (c *FakeOpenIDConnectPresets) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.OpenIDConnectPresetList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(openidconnectpresetsResource, openidconnectpresetsKind, c.ns, opts), &v1alpha1.OpenIDConnectPresetList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.OpenIDConnectPresetList{ListMeta: obj.(*v1alpha1.OpenIDConnectPresetList).ListMeta}
	for _, item := range obj.(*v1alpha1.OpenIDConnectPresetList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested openIDConnectPresets.
func (c *FakeOpenIDConnectPresets) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(openidconnectpresetsResource, c.ns, opts))

}

// Create takes the representation of a openIDConnectPreset and creates it.  Returns the server's representation of the openIDConnectPreset, and an error, if there is any.
func (c *FakeOpenIDConnectPresets) Create(ctx context.Context, openIDConnectPreset *v1alpha1.OpenIDConnectPreset, opts v1.CreateOptions) (result *v1alpha1.OpenIDConnectPreset, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(openidconnectpresetsResource, c.ns, openIDConnectPreset), &v1alpha1.OpenIDConnectPreset{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.OpenIDConnectPreset), err
}

// Update takes the representation of a openIDConnectPreset and updates it. Returns the server's representation of the openIDConnectPreset, and an error, if there is any.
func (c *FakeOpenIDConnectPresets) Update(ctx context.Context, openIDConnectPreset *v1alpha1.OpenIDConnectPreset, opts v1.UpdateOptions) (result *v1alpha1.OpenIDConnectPreset, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(openidconnectpresetsResource, c.ns, openIDConnectPreset), &v1alpha1.OpenIDConnectPreset{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.OpenIDConnectPreset), err
}

// Delete takes name of the openIDConnectPreset and deletes it. Returns an error if one occurs.
func (c *FakeOpenIDConnectPresets) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(openidconnectpresetsResource, c.ns, name, opts), &v1alpha1.OpenIDConnectPreset{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeOpenIDConnectPresets) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(openidconnectpresetsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.OpenIDConnectPresetList{})
	return err
}

// Patch applies the patch and returns the patched openIDConnectPreset.
func (c *FakeOpenIDConnectPresets) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.OpenIDConnectPreset, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(openidconnectpresetsResource, c.ns, name, pt, data, subresources...), &v1alpha1.OpenIDConnectPreset{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.OpenIDConnectPreset), err
}
