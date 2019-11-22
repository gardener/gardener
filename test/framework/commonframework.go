// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package framework

import (
	"flag"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
)

var cfg *CommonConfig

// CommonConfig is the configuration for a common framework
type CommonConfig struct {
	logLevel string
}

// CommonFramework represents the common gardener test framework that consolidates all
// shared features of the specific test frameworks (system, garderner, shoot)
type CommonFramework struct {
	Logger *logrus.Logger

	// ResourcesDir is the absolute path to the resources directory
	ResourcesDir string

	ChartDir string
}

// NewCommonFramework creates a new common framework and registers its ginkgo BeforeEach setup
func NewCommonFramework() *CommonFramework {
	f := &CommonFramework{}
	ginkgo.BeforeEach(f.BeforeEach)
	return f
}

// BeforeEach should be called in ginkgo's BeforeEach.
// It sets up the common framework.
func (f *CommonFramework) BeforeEach() {
	var err error
	f.Logger = logger.AddWriter(logger.NewLogger(cfg.logLevel), ginkgo.GinkgoWriter)

	f.ResourcesDir, err = filepath.Abs(filepath.Join("..", "..", "integration", "resources"))
	ExpectNoError(err)
	FileExists(f.ResourcesDir)

	f.ChartDir = filepath.Join(f.ResourcesDir, "charts")
}

// CommonAfterSuite performs necessary common steps after all tests of a suite a run
func CommonAfterSuite() {
	resourcesDir, err := filepath.Abs(filepath.Join("..", "..", "integration", "resources"))
	ExpectNoError(err)
	err = os.RemoveAll(filepath.Join(resourcesDir, "charts"))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	err = os.RemoveAll(filepath.Join(resourcesDir, "repository", "cache"))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

// RegisterCommonFrameworkFlags adds all flags that are needed to configure a common framework to the provided flagset.
func RegisterCommonFrameworkFlags(flagset *flag.FlagSet) *CommonConfig {
	if flagset == nil {
		flagset = flag.CommandLine
	}

	newCfg := &CommonConfig{}

	flag.StringVar(&newCfg.logLevel, "verbose", "", "verbosity level, when set, logging level will be DEBUG")

	cfg = newCfg
	return cfg
}
