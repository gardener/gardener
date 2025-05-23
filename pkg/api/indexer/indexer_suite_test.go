// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package indexer_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestIndexer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Indexer Suite")
}

type fakeFieldIndexer struct {
	obj          client.Object
	field        string
	extractValue client.IndexerFunc
}

func (f *fakeFieldIndexer) IndexField(_ context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	f.obj = obj
	f.field = field
	f.extractValue = extractValue
	return nil
}
