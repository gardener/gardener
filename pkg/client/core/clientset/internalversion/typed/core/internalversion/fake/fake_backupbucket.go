// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

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

// FakeBackupBuckets implements BackupBucketInterface
type FakeBackupBuckets struct {
	Fake *FakeCore
}

var backupbucketsResource = core.SchemeGroupVersion.WithResource("backupbuckets")

var backupbucketsKind = core.SchemeGroupVersion.WithKind("BackupBucket")

// Get takes name of the backupBucket, and returns the corresponding backupBucket object, and an error if there is any.
func (c *FakeBackupBuckets) Get(ctx context.Context, name string, options v1.GetOptions) (result *core.BackupBucket, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(backupbucketsResource, name), &core.BackupBucket{})
	if obj == nil {
		return nil, err
	}
	return obj.(*core.BackupBucket), err
}

// List takes label and field selectors, and returns the list of BackupBuckets that match those selectors.
func (c *FakeBackupBuckets) List(ctx context.Context, opts v1.ListOptions) (result *core.BackupBucketList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(backupbucketsResource, backupbucketsKind, opts), &core.BackupBucketList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &core.BackupBucketList{ListMeta: obj.(*core.BackupBucketList).ListMeta}
	for _, item := range obj.(*core.BackupBucketList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested backupBuckets.
func (c *FakeBackupBuckets) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(backupbucketsResource, opts))
}

// Create takes the representation of a backupBucket and creates it.  Returns the server's representation of the backupBucket, and an error, if there is any.
func (c *FakeBackupBuckets) Create(ctx context.Context, backupBucket *core.BackupBucket, opts v1.CreateOptions) (result *core.BackupBucket, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(backupbucketsResource, backupBucket), &core.BackupBucket{})
	if obj == nil {
		return nil, err
	}
	return obj.(*core.BackupBucket), err
}

// Update takes the representation of a backupBucket and updates it. Returns the server's representation of the backupBucket, and an error, if there is any.
func (c *FakeBackupBuckets) Update(ctx context.Context, backupBucket *core.BackupBucket, opts v1.UpdateOptions) (result *core.BackupBucket, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(backupbucketsResource, backupBucket), &core.BackupBucket{})
	if obj == nil {
		return nil, err
	}
	return obj.(*core.BackupBucket), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeBackupBuckets) UpdateStatus(ctx context.Context, backupBucket *core.BackupBucket, opts v1.UpdateOptions) (*core.BackupBucket, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(backupbucketsResource, "status", backupBucket), &core.BackupBucket{})
	if obj == nil {
		return nil, err
	}
	return obj.(*core.BackupBucket), err
}

// Delete takes name of the backupBucket and deletes it. Returns an error if one occurs.
func (c *FakeBackupBuckets) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(backupbucketsResource, name, opts), &core.BackupBucket{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeBackupBuckets) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(backupbucketsResource, listOpts)

	_, err := c.Fake.Invokes(action, &core.BackupBucketList{})
	return err
}

// Patch applies the patch and returns the patched backupBucket.
func (c *FakeBackupBuckets) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *core.BackupBucket, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(backupbucketsResource, name, pt, data, subresources...), &core.BackupBucket{})
	if obj == nil {
		return nil, err
	}
	return obj.(*core.BackupBucket), err
}
