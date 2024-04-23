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
	"github.com/gardener/gardener/test/framework"
)

const testID = "shoot-test"

var parentCtx context.Context

var _ = BeforeEach(func() {
	parentCtx = context.Background()
	logf.SetLogger(logger.MustNewZapLogger(logger.InfoLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)).WithName(testID))
})

func defaultShootCreationFramework() *framework.ShootCreationFramework {
	return framework.NewShootCreationFramework(&framework.ShootCreationConfig{
		GardenerConfig: e2e.DefaultGardenConfig("garden-local"),
	})
}
