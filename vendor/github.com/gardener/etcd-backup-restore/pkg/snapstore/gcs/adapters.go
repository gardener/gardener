// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gcs

import (
	"context"

	"cloud.google.com/go/storage"
)

// AdaptClient adapts a storage.Client so that it satisfies the Client
// interface.
func AdaptClient(c *storage.Client) Client {
	return client{c}
}

type (
	client         struct{ *storage.Client }
	bucketHandle   struct{ *storage.BucketHandle }
	objectHandle   struct{ *storage.ObjectHandle }
	bucketIterator struct{ *storage.BucketIterator }
	objectIterator struct{ *storage.ObjectIterator }
	reader         struct{ *storage.Reader }
	writer         struct{ *storage.Writer }
	copier         struct{ *storage.Copier }
	composer       struct{ *storage.Composer }
	aclHandle      struct{ *storage.ACLHandle }
)

func (client) embedToIncludeNewMethods()         {}
func (bucketHandle) embedToIncludeNewMethods()   {}
func (objectHandle) embedToIncludeNewMethods()   {}
func (bucketIterator) embedToIncludeNewMethods() {}
func (objectIterator) embedToIncludeNewMethods() {}
func (writer) embedToIncludeNewMethods()         {}
func (reader) embedToIncludeNewMethods()         {}
func (copier) embedToIncludeNewMethods()         {}
func (composer) embedToIncludeNewMethods()       {}
func (aclHandle) embedToIncludeNewMethods()      {}

// Bucket ...
func (c client) Bucket(name string) BucketHandle {
	return bucketHandle{c.Client.Bucket(name)}
}

// Buckets ...
func (c client) Buckets(ctx context.Context, projectID string) BucketIterator {
	return bucketIterator{c.Client.Buckets(ctx, projectID)}
}

// Object ...
func (b bucketHandle) Object(name string) ObjectHandle {
	return objectHandle{b.BucketHandle.Object(name)}
}

// If ...
func (b bucketHandle) If(conds storage.BucketConditions) BucketHandle {
	return bucketHandle{b.BucketHandle.If(conds)}
}

// Objects ...
func (b bucketHandle) Objects(ctx context.Context, q *storage.Query) ObjectIterator {
	return objectIterator{b.BucketHandle.Objects(ctx, q)}
}

// DefaultObjectACL ...
func (b bucketHandle) DefaultObjectACL() ACLHandle {
	return aclHandle{b.BucketHandle.DefaultObjectACL()}
}

// ACL ...
func (b bucketHandle) ACL() ACLHandle {
	return aclHandle{b.BucketHandle.ACL()}
}

// UserProject ...
func (b bucketHandle) UserProject(projectID string) BucketHandle {
	return bucketHandle{b.BucketHandle.UserProject(projectID)}
}

// SetPrefix ...
func (bi bucketIterator) SetPrefix(s string) {
	bi.BucketIterator.Prefix = s
}

// ACL ...
func (o objectHandle) ACL() ACLHandle {
	return aclHandle{o.ObjectHandle.ACL()}
}

// Generation ...
func (o objectHandle) Generation(gen int64) ObjectHandle {
	return objectHandle{o.ObjectHandle.Generation(gen)}
}

// If ...
func (o objectHandle) If(conds storage.Conditions) ObjectHandle {
	return objectHandle{o.ObjectHandle.If(conds)}
}

// Key ...
func (o objectHandle) Key(encryptionKey []byte) ObjectHandle {
	return objectHandle{o.ObjectHandle.Key(encryptionKey)}
}

// ReadCompressed ...
func (o objectHandle) ReadCompressed(compressed bool) ObjectHandle {
	return objectHandle{o.ObjectHandle.ReadCompressed(compressed)}
}

// NewReader ...
func (o objectHandle) NewReader(ctx context.Context) (Reader, error) {
	r, err := o.ObjectHandle.NewReader(ctx)
	if err != nil {
		return nil, err
	}
	return reader{r}, nil
}

// NewRangeReader ...
func (o objectHandle) NewRangeReader(ctx context.Context, offset, length int64) (Reader, error) {
	r, err := o.ObjectHandle.NewRangeReader(ctx, offset, length)
	if err != nil {
		return nil, err
	}
	return reader{r}, nil
}

// NewWriter ...
func (o objectHandle) NewWriter(ctx context.Context) Writer {
	return writer{o.ObjectHandle.NewWriter(ctx)}
}

// CopierFrom ...
func (o objectHandle) CopierFrom(src ObjectHandle) Copier {
	return copier{o.ObjectHandle.CopierFrom(src.(objectHandle).ObjectHandle)}
}

// ComposetFrom ...
func (o objectHandle) ComposerFrom(srcs ...ObjectHandle) Composer {
	objs := make([]*storage.ObjectHandle, len(srcs))
	for i, s := range srcs {
		objs[i] = s.(objectHandle).ObjectHandle
	}
	return composer{o.ObjectHandle.ComposerFrom(objs...)}
}

// ObjectAttrs ...
func (w writer) ObjectAttrs() *storage.ObjectAttrs {
	return &w.Writer.ObjectAttrs
}

// SetChunkSize ...
func (w writer) SetChunkSize(s int) {
	w.ChunkSize = s
}

// SetProgressFunc ...
func (w writer) SetProgressFunc(f func(int64)) {
	w.ProgressFunc = f
}

// SetCRC32C ...
func (w writer) SetCRC32C(c uint32) {
	w.CRC32C = c
	w.SendCRC32C = true
}

// ObjectAttrs ...
func (c copier) ObjectAttrs() *storage.ObjectAttrs {
	return &c.Copier.ObjectAttrs
}

// SetRewriteToken ...
func (c copier) SetRewriteToken(t string) {
	c.RewriteToken = t
}

// SetProgressFunc ...
func (c copier) SetProgressFunc(f func(copiedBytes, totalBytes uint64)) {
	c.ProgressFunc = f
}

// SetDestinationKMSKeyName ...
func (c copier) SetDestinationKMSKeyName(k string) {
	c.DestinationKMSKeyName = k
}

// ObjectAttrs ...
func (c composer) ObjectAttrs() *storage.ObjectAttrs {
	return &c.Composer.ObjectAttrs
}
