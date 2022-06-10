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

package etcd_test

import (
	"testing"

	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEtcd(t *testing.T) {
	gardenletfeatures.RegisterFeatureGates()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Botanist Component Etcd Suite")
}

const (
	testNamespace = "shoot--test--test"
	testRole      = "test"
	testROLE      = "Test"
)

var _ = BeforeSuite(func() {
	DeferCleanup(test.WithVar(&secretutils.GenerateKey, secretutils.FakeGenerateKey))
})
