// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package plutono_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardener/pkg/features"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
)

func TestPlutono(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component Observability Plutono Suite")
}

var _ = BeforeSuite(func() {
	DeferCleanup(test.WithVar(&secretsutils.GenerateRandomString, secretsutils.FakeGenerateRandomString))
	utilruntime.Must(features.DefaultFeatureGate.Add(features.GetFeatures(
		features.RemoveVali,
	)))
})
