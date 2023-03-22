// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
