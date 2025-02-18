// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/pkg/logger"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal"
	"github.com/gardener/gardener/test/framework"
)

// TODO(timebertt): delete this file when finishing https://github.com/gardener/gardener/issues/11379

var parentCtx context.Context

var _ = BeforeEach(func() {
	parentCtx = context.Background()
	logf.SetLogger(logger.MustNewZapLogger(logger.InfoLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)).WithName("shoot-test"))

	internal.LoadLegacyFlags()
})

func defaultShootCreationFramework() *framework.ShootCreationFramework {
	return framework.NewShootCreationFramework(&framework.ShootCreationConfig{
		GardenerConfig: e2e.DefaultGardenConfig("garden-local"),
	})
}
