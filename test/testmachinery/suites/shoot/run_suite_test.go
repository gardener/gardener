// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_suite_test

import (
	"flag"
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/config"
	"github.com/gardener/gardener/test/framework/reporter"
	_ "github.com/gardener/gardener/test/testmachinery/shoots/applications"
	_ "github.com/gardener/gardener/test/testmachinery/shoots/care"
	_ "github.com/gardener/gardener/test/testmachinery/shoots/logging"
	_ "github.com/gardener/gardener/test/testmachinery/shoots/operatingsystem"
	_ "github.com/gardener/gardener/test/testmachinery/shoots/operations"
	_ "github.com/gardener/gardener/test/testmachinery/shoots/vpntunnel"
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
	RunSpecs(t, "Shoot Test Suite")
}

var _ = ReportAfterSuite("Report to Elasticsearch", func(report Report) {
	reporter.ReportResults(*reportFilePath, *esIndex, report)
})
