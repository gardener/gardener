// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_suite_test

import (
	"flag"
	"fmt"
	"os"

	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/config"
	"github.com/gardener/gardener/test/framework/reporter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	_ "github.com/gardener/gardener/test/integration/plants"
	_ "github.com/gardener/gardener/test/integration/shoots/applications"
	_ "github.com/gardener/gardener/test/integration/shoots/care"
	_ "github.com/gardener/gardener/test/integration/shoots/logging"
	_ "github.com/gardener/gardener/test/integration/shoots/operatingsystem"
	_ "github.com/gardener/gardener/test/integration/shoots/operations"
)

var (
	configFilePath = flag.String("config", "", "Specify the configuration file")
	esIndex        = flag.String("es-index", "gardener-testsuite", "Specify the elastic search index where the report should be ingested")
	reportFilePath = flag.String("report-file", "/tmp/shoot_res.json", "Specify the file to write the test results")
)

func TestMain(m *testing.M) {
	framework.RegisterShootFrameworkFlags()
	flag.Parse()

	if err := config.ParseConfigForFlags(*configFilePath, flag.CommandLine); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	RegisterFailHandler(Fail)

	AfterSuite(func() {
		framework.CommonAfterSuite()
	})

	os.Exit(m.Run())
}

func TestGardenerSuite(t *testing.T) {
	RunSpecsWithDefaultAndCustomReporters(t, "Shoot Test Suite", []Reporter{reporter.NewGardenerESReporter(*reportFilePath, *esIndex)})
}
