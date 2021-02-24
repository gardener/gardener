// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package graph

import (
	"context"

	"github.com/gardener/gardener/pkg/client/kubernetes"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ = Describe("graph", func() {
	var (
		ctx = context.TODO()

		fakeInformers *informertest.FakeInformers

		logger logr.Logger
		graph  *graph
	)

	BeforeEach(func() {
		scheme := kubernetes.GardenScheme
		Expect(metav1.AddMetaToScheme(scheme)).To(Succeed())

		fakeInformers = &informertest.FakeInformers{
			Scheme:         scheme,
			InformersByGVK: map[schema.GroupVersionKind]toolscache.SharedIndexInformer{},
		}

		logger = logzap.New(logzap.WriteTo(GinkgoWriter))
		graph = New(logger)
		Expect(graph.Setup(ctx, fakeInformers)).To(Succeed())
	})
})
