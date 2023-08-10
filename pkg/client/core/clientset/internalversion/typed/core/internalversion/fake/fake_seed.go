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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	core "github.com/gardener/gardener/pkg/apis/core"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeSeeds implements SeedInterface
type FakeSeeds struct {
	Fake *FakeCore
}

var seedsResource = core.SchemeGroupVersion.WithResource("seeds")

var seedsKind = core.SchemeGroupVersion.WithKind("Seed")

// Get takes name of the seed, and returns the corresponding seed object, and an error if there is any.
func (c *FakeSeeds) Get(ctx context.Context, name string, options v1.GetOptions) (result *core.Seed, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(seedsResource, name), &core.Seed{})
	if obj == nil {
		return nil, err
	}
	return obj.(*core.Seed), err
}

// List takes label and field selectors, and returns the list of Seeds that match those selectors.
func (c *FakeSeeds) List(ctx context.Context, opts v1.ListOptions) (result *core.SeedList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(seedsResource, seedsKind, opts), &core.SeedList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &core.SeedList{ListMeta: obj.(*core.SeedList).ListMeta}
	for _, item := range obj.(*core.SeedList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested seeds.
func (c *FakeSeeds) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(seedsResource, opts))
}

// Create takes the representation of a seed and creates it.  Returns the server's representation of the seed, and an error, if there is any.
func (c *FakeSeeds) Create(ctx context.Context, seed *core.Seed, opts v1.CreateOptions) (result *core.Seed, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(seedsResource, seed), &core.Seed{})
	if obj == nil {
		return nil, err
	}
	return obj.(*core.Seed), err
}

// Update takes the representation of a seed and updates it. Returns the server's representation of the seed, and an error, if there is any.
func (c *FakeSeeds) Update(ctx context.Context, seed *core.Seed, opts v1.UpdateOptions) (result *core.Seed, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(seedsResource, seed), &core.Seed{})
	if obj == nil {
		return nil, err
	}
	return obj.(*core.Seed), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeSeeds) UpdateStatus(ctx context.Context, seed *core.Seed, opts v1.UpdateOptions) (*core.Seed, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(seedsResource, "status", seed), &core.Seed{})
	if obj == nil {
		return nil, err
	}
	return obj.(*core.Seed), err
}

// Delete takes name of the seed and deletes it. Returns an error if one occurs.
func (c *FakeSeeds) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(seedsResource, name, opts), &core.Seed{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeSeeds) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(seedsResource, listOpts)

	_, err := c.Fake.Invokes(action, &core.SeedList{})
	return err
}

// Patch applies the patch and returns the patched seed.
func (c *FakeSeeds) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *core.Seed, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(seedsResource, name, pt, data, subresources...), &core.Seed{})
	if obj == nil {
		return nil, err
	}
	return obj.(*core.Seed), err
}
