// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package downloader_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/gardenlet/features"
)

func TestDownloader(t *testing.T) {
	features.RegisterFeatureGates()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Component Extensions OperatingSystemConfig Downloader Suite")
}
