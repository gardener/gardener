// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/gardener/gardener/pkg/logger"
)

var commonCfg *CommonConfig

// CommonConfig is the configuration for a common framework
type CommonConfig struct {
	LogLevel         string
	DisableStateDump bool
	ResourceDir      string
	ChartDir         string
}

// CommonFramework represents the common gardener test framework that consolidates all
// shared features of the specific test frameworks (system, garderner, shoot)
type CommonFramework struct {
	Config           *CommonConfig
	Logger           logr.Logger
	DisableStateDump bool

	// ResourcesDir is the absolute path to the resources directory
	ResourcesDir string

	// TemplatesDir is the absolute path to the templates directory
	TemplatesDir string

	// Chart is the absolute path to the helm chart directory
	ChartDir string
}

// NewCommonFramework creates a new common framework and registers its ginkgo BeforeEach setup
func NewCommonFramework(cfg *CommonConfig) *CommonFramework {
	f := newCommonFrameworkFromConfig(cfg)
	ginkgo.BeforeEach(f.BeforeEach)
	return f
}

// newCommonFrameworkFromConfig creates a new common framework and without registering its ginkgo BeforeEach setup
func newCommonFrameworkFromConfig(cfg *CommonConfig) *CommonFramework {
	f := &CommonFramework{
		Config: cfg,
	}
	return f
}

// BeforeEach should be called in ginkgo's BeforeEach.
// It sets up the common framework.
func (f *CommonFramework) BeforeEach() {
	f.Config = mergeCommonConfigs(f.Config, commonCfg)
	f.DisableStateDump = f.Config.DisableStateDump

	logf.SetLogger(logger.MustNewZapLogger(f.Config.LogLevel, logger.FormatJSON, zap.WriteTo(ginkgo.GinkgoWriter)))
	f.Logger = logf.Log.WithName("test")

	if f.ResourcesDir == "" {
		var err error
		if f.Config.ResourceDir != "" {
			f.ResourcesDir, err = filepath.Abs(f.Config.ResourceDir)
		} else {
			// This is the default location if the framework is running in one of the gardener/shoot suites.
			// Otherwise the resource dir has to be adjusted
			f.ResourcesDir, err = filepath.Abs(filepath.Join("..", "..", "..", "framework", "resources"))
		}
		ExpectNoError(err)
	}
	FileExists(f.ResourcesDir)

	f.TemplatesDir = filepath.Join(f.ResourcesDir, "templates")

	f.ChartDir = filepath.Join(f.ResourcesDir, "charts")
	if f.Config.ChartDir != "" {
		f.ChartDir = f.Config.ChartDir
	}
}

// CommonAfterSuite performs necessary common steps after all tests of a suite a run
func CommonAfterSuite() {
	// run all registered cleanup functions
	RunCleanupActions()

	resourcesDir, err := filepath.Abs(filepath.Join("..", "..", "..", "framework", "resources"))
	ExpectNoError(err)
	err = os.RemoveAll(filepath.Join(resourcesDir, "charts"))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	err = os.RemoveAll(filepath.Join(resourcesDir, "repository", "cache"))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

func mergeCommonConfigs(base, overwrite *CommonConfig) *CommonConfig {
	if base == nil {
		return overwrite
	}
	if overwrite == nil {
		return base
	}

	if StringSet(overwrite.LogLevel) {
		base.LogLevel = overwrite.LogLevel
	}
	if StringSet(overwrite.ResourceDir) {
		base.ResourceDir = overwrite.ResourceDir
	}
	if StringSet(overwrite.ChartDir) {
		base.ChartDir = overwrite.ChartDir
	}
	if overwrite.DisableStateDump {
		base.DisableStateDump = overwrite.DisableStateDump
	}
	return base
}

// RegisterCommonFrameworkFlags adds all flags that are needed to configure a common framework to the provided flagset.
func RegisterCommonFrameworkFlags() *CommonConfig {
	newCfg := &CommonConfig{}

	flag.StringVar(&newCfg.LogLevel, "verbose", logger.InfoLevel, "verbosity level (defaults to info)")
	flag.BoolVar(&newCfg.DisableStateDump, "disable-dump", false, "Disable the state dump if a test fails")

	commonCfg = newCfg
	return commonCfg
}
