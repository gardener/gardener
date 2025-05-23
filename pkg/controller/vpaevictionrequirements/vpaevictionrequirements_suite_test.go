// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpaevictionrequirements_test

import (
	"io"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/pkg/logger"
)

func TestVPAEvictionRequirements(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller VPAEvictionRequirements Suite")
}

var logBuffer *gbytes.Buffer

var _ = BeforeSuite(func() {
	logBuffer = gbytes.NewBuffer()
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(io.MultiWriter(GinkgoWriter, logBuffer))))
})
