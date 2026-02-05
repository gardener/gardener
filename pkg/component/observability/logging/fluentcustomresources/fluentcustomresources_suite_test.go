// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fluentcustomresources_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardener/pkg/features"
)

var _ = BeforeSuite(func() {
	utilruntime.Must(features.DefaultFeatureGate.Add(features.GetFeatures(
		features.OpenTelemetryCollector,
	)))
})

func TestFluentCustomresources(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component Observability Logging FluentCustomResources Suite")
}
