// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenadm_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/test/e2e/gardenadm/common"
	_ "github.com/gardener/gardener/test/e2e/gardenadm/hightouch"
	_ "github.com/gardener/gardener/test/e2e/gardenadm/mediumtouch"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test E2E gardenadm Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.InfoLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	SetupRuntimeClient()
})
