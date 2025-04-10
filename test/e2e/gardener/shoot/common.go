// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	. "github.com/onsi/ginkgo/v2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal"
)

// TODO(timebertt): delete this file when finishing https://github.com/gardener/gardener/issues/11379

var _ = BeforeEach(func() {
	logf.SetLogger(logger.MustNewZapLogger(logger.InfoLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)).WithName("shoot-test"))

	internal.LoadLegacyFlags()
})
