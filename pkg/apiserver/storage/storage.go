// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/server/storage"
)

var _ storage.StorageFactory = (*GardenerStorageFactory)(nil)

// GardenerStorageFactory is the default storage factory for Gardener resources.
type GardenerStorageFactory struct {
	*storage.DefaultStorageFactory
}

// ResourcePrefix implements `storage.StorageFactory`.
// It is based on the `ResourcePrefix` of `options.SimpleRestOptionsFactory`.
func (g *GardenerStorageFactory) ResourcePrefix(groupResource schema.GroupResource) string {
	return groupResource.Group + "/" + groupResource.Resource
}
