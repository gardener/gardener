// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/utils/ptr"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/pkg/logger"
)

const (
	// LeaderElectionFlag is the name of the command line flag to specify whether to do leader election or not.
	LeaderElectionFlag = "leader-election"
	// LeaderElectionIDFlag is the name of the command line flag to specify the leader election ID.
	LeaderElectionIDFlag = "leader-election-id"
	// LeaderElectionNamespaceFlag is the name of the command line flag to specify the leader election namespace.
	LeaderElectionNamespaceFlag = "leader-election-namespace"
	// WebhookServerHostFlag is the name of the command line flag to specify the webhook config host for 'url' mode.
	WebhookServerHostFlag = "webhook-config-server-host"
	// WebhookServerPortFlag is the name of the command line flag to specify the webhook server port.
	WebhookServerPortFlag = "webhook-config-server-port"
	// WebhookCertDirFlag is the name of the command line flag to specify the webhook certificate directory.
	WebhookCertDirFlag = "webhook-config-cert-dir"
	// MetricsBindAddressFlag is the name of the command line flag to specify the TCP address that the controller
	// should bind to for serving prometheus metrics.
	// It can be set to "0" to disable the metrics serving.
	MetricsBindAddressFlag = "metrics-bind-address"
	// HealthBindAddressFlag is the name of the command line flag to specify the TCP address that the controller
	// should bind to for serving health probes
	HealthBindAddressFlag = "health-bind-address"

	// MaxConcurrentReconcilesFlag is the name of the command line flag to specify the maximum number of
	// concurrent reconciliations a controller can do.
	MaxConcurrentReconcilesFlag = "max-concurrent-reconciles"

	// KubeconfigFlag is the name of the command line flag to specify a kubeconfig used to retrieve
	// a rest.Config for a manager.Manager.
	KubeconfigFlag = clientcmd.RecommendedConfigPathFlag
	// MasterURLFlag is the name of the command line flag to specify the master URL override for
	// a rest.Config of a manager.Manager.
	MasterURLFlag = "master"

	// ControllersFlag is the name of the command line flag to enable individual controllers.
	ControllersFlag = "controllers"

	// DisableFlag is the name of the command line flag to disable individual controllers.
	DisableFlag = "disable-controllers"

	// GardenerVersionFlag is the name of the command line flag containing the Gardener version.
	GardenerVersionFlag = "gardener-version"
	// AutonomousShootClusterFlag is the name of the command line flag indicating that the extension runs in an
	// autonomous shoot cluster.
	AutonomousShootClusterFlag = "autonomous-shoot-cluster"

	// LogLevelFlag is the name of the command line flag containing the log level.
	LogLevelFlag = "log-level"

	// LogFormatFlag is the name of the command line flag containing the log format.
	LogFormatFlag = "log-format"
)

// LeaderElectionNameID returns a leader election ID for the given name.
func LeaderElectionNameID(name string) string {
	return name + "-leader-election"
}

// Flagger adds flags to a given FlagSet.
type Flagger interface {
	// AddFlags adds the flags of this Flagger to the given FlagSet.
	AddFlags(*pflag.FlagSet)
}

type prefixedFlagger struct {
	prefix  string
	flagger Flagger
}

// AddFlags implements Flagger.AddFlags.
func (p *prefixedFlagger) AddFlags(fs *pflag.FlagSet) {
	temp := pflag.NewFlagSet("", pflag.ExitOnError)
	p.flagger.AddFlags(temp)
	temp.VisitAll(func(flag *pflag.Flag) {
		flag.Name = fmt.Sprintf("%s%s", p.prefix, flag.Name)
	})
	fs.AddFlagSet(temp)
}

// PrefixFlagger creates a flagger that prefixes all its flags with the given prefix.
func PrefixFlagger(prefix string, flagger Flagger) Flagger {
	return &prefixedFlagger{prefix, flagger}
}

// PrefixOption creates an option that prefixes all its flags with the given prefix.
func PrefixOption(prefix string, option Option) Option {
	return struct {
		Flagger
		Completer
	}{PrefixFlagger(prefix, option), option}
}

// Completer completes some work.
type Completer interface {
	// Complete completes the work, optionally returning an error.
	Complete() error
}

// Option is a Flagger and Completer.
// It sets command line flags and does some work when the flags have been parsed, optionally producing
// an error.
type Option interface {
	Flagger
	Completer
}

// OptionAggregator is a builder that aggregates multiple options.
type OptionAggregator []Option

// NewOptionAggregator instantiates a new OptionAggregator and registers all given options.
func NewOptionAggregator(options ...Option) OptionAggregator {
	var builder OptionAggregator
	builder.Register(options...)
	return builder
}

// Register registers the given options in this OptionAggregator.
func (b *OptionAggregator) Register(options ...Option) {
	*b = append(*b, options...)
}

// AddFlags implements Flagger.AddFlags.
func (b *OptionAggregator) AddFlags(fs *pflag.FlagSet) {
	for _, option := range *b {
		option.AddFlags(fs)
	}
}

// Complete implements Completer.Complete.
func (b *OptionAggregator) Complete() error {
	for _, option := range *b {
		if err := option.Complete(); err != nil {
			return err
		}
	}
	return nil
}

// ManagerOptions are command line options that can be set for manager.Options.
type ManagerOptions struct {
	// LeaderElection is whether leader election is turned on or not.
	LeaderElection bool
	// LeaderElectionID is the id to do leader election with.
	LeaderElectionID string
	// LeaderElectionNamespace is the namespace to do leader election in.
	LeaderElectionNamespace string
	// WebhookServerHost is the host for the webhook server.
	WebhookServerHost string
	// WebhookServerPort is the port for the webhook server.
	WebhookServerPort int
	// WebhookCertDir is the directory that contains the webhook server key and certificate.
	WebhookCertDir string
	// MetricsBindAddress is the TCP address that the controller should bind to for serving prometheus metrics.
	MetricsBindAddress string
	// HealthBindAddress is the TCP address that the controller should bind to for serving health probes.
	HealthBindAddress string
	// LogLevel defines the level/severity for the logs. Must be one of [info,debug,error]
	LogLevel string
	// LogFormat defines the format for the logs. Must be one of [json,text]
	LogFormat string

	config *ManagerConfig
}

// AddFlags implements Flagger.AddFlags.
func (m *ManagerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&m.LeaderElection, LeaderElectionFlag, m.LeaderElection, "Whether to use leader election or not when running this controller manager.")
	fs.StringVar(&m.LeaderElectionID, LeaderElectionIDFlag, m.LeaderElectionID, "The leader election id to use.")
	fs.StringVar(&m.LeaderElectionNamespace, LeaderElectionNamespaceFlag, m.LeaderElectionNamespace, "The namespace to do leader election in.")
	fs.StringVar(&m.WebhookServerHost, WebhookServerHostFlag, m.WebhookServerHost, "The webhook server host.")
	fs.IntVar(&m.WebhookServerPort, WebhookServerPortFlag, m.WebhookServerPort, "The webhook server port.")
	fs.StringVar(&m.WebhookCertDir, WebhookCertDirFlag, m.WebhookCertDir, "The directory that contains the webhook server key and certificate.")
	fs.StringVar(&m.MetricsBindAddress, MetricsBindAddressFlag, ":8080", "bind address for the metrics server")
	fs.StringVar(&m.HealthBindAddress, HealthBindAddressFlag, ":8081", "bind address for the health server")
	fs.StringVar(&m.LogLevel, LogLevelFlag, logger.InfoLevel, "The level/severity for the logs. Must be one of [info,debug,error]")
	fs.StringVar(&m.LogFormat, LogFormatFlag, logger.FormatJSON, "The format for the logs. Must be one of [json,text]")
}

// Complete implements Completer.Complete.
func (m *ManagerOptions) Complete() error {
	if !sets.New(logger.AllLogLevels...).Has(m.LogLevel) {
		return fmt.Errorf("invalid --%s: %s", LogLevelFlag, m.LogLevel)
	}

	if !sets.New(logger.AllLogFormats...).Has(m.LogFormat) {
		return fmt.Errorf("invalid --%s: %s", LogFormatFlag, m.LogFormat)
	}

	logger, err := logger.NewZapLogger(m.LogLevel, m.LogFormat)
	if err != nil {
		return fmt.Errorf("error instantiating zap logger: %w", err)
	}

	m.config = &ManagerConfig{
		m.LeaderElection,
		m.LeaderElectionID,
		m.LeaderElectionNamespace,
		m.WebhookServerHost,
		m.WebhookServerPort,
		m.WebhookCertDir,
		m.MetricsBindAddress,
		m.HealthBindAddress,
		logger}
	return nil
}

// Completed returns the completed ManagerConfig. Only call this if `Complete` was successful.
func (m *ManagerOptions) Completed() *ManagerConfig {
	return m.config
}

// ManagerConfig is a completed manager configuration.
type ManagerConfig struct {
	// LeaderElection is whether leader election is turned on or not.
	LeaderElection bool
	// LeaderElectionID is the id to do leader election with.
	LeaderElectionID string
	// LeaderElectionNamespace is the namespace to do leader election in.
	LeaderElectionNamespace string
	// WebhookServerHost is the host for the webhook server.
	WebhookServerHost string
	// WebhookServerPort is the port for the webhook server.
	WebhookServerPort int
	// WebhookCertDir is the directory that contains the webhook server key and certificate.
	WebhookCertDir string
	// MetricsBindAddress is the TCP address that the controller should bind to for serving prometheus metrics.
	MetricsBindAddress string
	// HealthBindAddress is the TCP address that the controller should bind to for serving health probes.
	HealthBindAddress string
	// Logger is a logr.Logger compliant logger
	Logger logr.Logger
}

// Apply sets the values of this ManagerConfig in the given manager.Options.
func (c *ManagerConfig) Apply(opts *manager.Options) {
	opts.LeaderElection = c.LeaderElection
	opts.LeaderElectionResourceLock = resourcelock.LeasesResourceLock
	opts.LeaderElectionID = c.LeaderElectionID
	opts.LeaderElectionNamespace = c.LeaderElectionNamespace
	opts.Metrics = metricsserver.Options{BindAddress: c.MetricsBindAddress}
	opts.HealthProbeBindAddress = c.HealthBindAddress
	opts.Logger = c.Logger
	opts.Controller = controllerconfig.Controller{RecoverPanic: ptr.To(true)}
	opts.WebhookServer = webhook.NewServer(webhook.Options{
		Host:    c.WebhookServerHost,
		Port:    c.WebhookServerPort,
		CertDir: c.WebhookCertDir,
	})
}

// Options initializes empty manager.Options, applies the set values and returns it.
func (c *ManagerConfig) Options() manager.Options {
	var opts manager.Options
	c.Apply(&opts)
	return opts
}

// ControllerOptions are command line options that can be set for controller.Options.
type ControllerOptions struct {
	// MaxConcurrentReconciles are the maximum concurrent reconciles.
	MaxConcurrentReconciles int

	config *ControllerConfig
}

// AddFlags implements Flagger.AddFlags.
func (c *ControllerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&c.MaxConcurrentReconciles, MaxConcurrentReconcilesFlag, c.MaxConcurrentReconciles, "The maximum number of concurrent reconciliations.")
}

// Complete implements Completer.Complete.
func (c *ControllerOptions) Complete() error {
	c.config = &ControllerConfig{c.MaxConcurrentReconciles}
	return nil
}

// Completed returns the completed ControllerConfig. Only call this if `Complete` was successful.
func (c *ControllerOptions) Completed() *ControllerConfig {
	return c.config
}

// ControllerConfig is a completed controller configuration.
type ControllerConfig struct {
	// MaxConcurrentReconciles is the maximum number of concurrent reconciles.
	MaxConcurrentReconciles int
}

// Apply sets the values of this ControllerConfig in the given controller.Options.
func (c *ControllerConfig) Apply(opts *controller.Options) {
	opts.MaxConcurrentReconciles = c.MaxConcurrentReconciles
}

// Options initializes empty controller.Options, applies the set values and returns it.
func (c *ControllerConfig) Options() controller.Options {
	var opts controller.Options
	c.Apply(&opts)
	return opts
}

// RESTOptions are command line options that can be set for rest.Config.
type RESTOptions struct {
	// Kubeconfig is the path to a kubeconfig.
	Kubeconfig string
	// MasterURL is an override for the URL in a kubeconfig. Only used if out-of-cluster.
	MasterURL string

	config *RESTConfig
}

// RESTConfig is a completed REST configuration.
type RESTConfig struct {
	// Config is the rest.Config.
	Config *rest.Config
}

var (
	// BuildConfigFromFlags creates a build configuration from the given flags. Exposed for testing.
	BuildConfigFromFlags = clientcmd.BuildConfigFromFlags
	// InClusterConfig obtains the current in-cluster config. Exposed for testing.
	InClusterConfig = rest.InClusterConfig
	// Getenv obtains the environment variable with the given name. Exposed for testing.
	Getenv = os.Getenv
	// RecommendedHomeFile is the recommended location of the kubeconfig. Exposed for testing.
	RecommendedHomeFile = clientcmd.RecommendedHomeFile
)

func (r *RESTOptions) buildConfig() (*rest.Config, error) {
	// If a flag is specified with the config location, use that
	if len(r.Kubeconfig) > 0 {
		return BuildConfigFromFlags(r.MasterURL, r.Kubeconfig)
	}
	// If an env variable is specified with the config location, use that
	if kubeconfig := Getenv(clientcmd.RecommendedConfigPathEnvVar); len(kubeconfig) > 0 {
		return BuildConfigFromFlags(r.MasterURL, kubeconfig)
	}
	// If no explicit location, try the in-cluster config
	if c, err := InClusterConfig(); err == nil {
		return c, nil
	}

	return BuildConfigFromFlags("", RecommendedHomeFile)
}

// Complete implements RESTCompleter.Complete.
func (r *RESTOptions) Complete() error {
	config, err := r.buildConfig()
	if err != nil {
		return err
	}

	r.config = &RESTConfig{config}
	return nil
}

// Completed returns the completed RESTConfig. Only call this if `Complete` was successful.
func (r *RESTOptions) Completed() *RESTConfig {
	return r.config
}

// AddFlags implements Flagger.AddFlags.
func (r *RESTOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&r.Kubeconfig, KubeconfigFlag, "", "Paths to a kubeconfig. Only required if out-of-cluster.")
	fs.StringVar(&r.MasterURL, MasterURLFlag, "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}

// SwitchOptions are options to build an AddToManager function that filters the disabled controllers.
type SwitchOptions struct {
	Enabled  []string
	Disabled []string

	nameToAddToManager  map[string]func(context.Context, manager.Manager) error
	addToManagerBuilder extensionscontroller.AddToManagerBuilder
}

// Register registers the given NameToControllerFuncs in the options.
func (d *SwitchOptions) Register(pairs ...NameToAddToManagerFunc) {
	for _, pair := range pairs {
		d.nameToAddToManager[pair.Name] = pair.Func
	}
}

// NameToAddToManagerFunc binds a specific name to a controller's AddToManager function.
type NameToAddToManagerFunc struct {
	Name string
	Func func(context.Context, manager.Manager) error
}

// Switch binds the given name to the given AddToManager function.
func Switch(name string, f func(context.Context, manager.Manager) error) NameToAddToManagerFunc {
	return NameToAddToManagerFunc{
		Name: name,
		Func: f,
	}
}

// NewSwitchOptions creates new SwitchOptions with the given initial pairs.
func NewSwitchOptions(pairs ...NameToAddToManagerFunc) *SwitchOptions {
	opts := SwitchOptions{nameToAddToManager: make(map[string]func(context.Context, manager.Manager) error)}
	opts.Register(pairs...)
	return &opts
}

// AddFlags implements Option.
func (d *SwitchOptions) AddFlags(fs *pflag.FlagSet) {
	controllerNames := make([]string, 0, len(d.nameToAddToManager))
	for name := range d.nameToAddToManager {
		controllerNames = append(controllerNames, name)
	}
	fs.StringSliceVar(&d.Enabled, ControllersFlag, controllerNames, fmt.Sprintf("List of controllers to enable. If not set, all controllers are enabled. %v", controllerNames))
	fs.StringSliceVar(&d.Disabled, DisableFlag, d.Disabled, fmt.Sprintf("List of controllers to disable %v", controllerNames))
}

// Complete implements Option.
func (d *SwitchOptions) Complete() error {
	var (
		enabled  = sets.New[string]()
		disabled = sets.New[string]()
	)

	for _, enabledName := range d.Enabled {
		if _, ok := d.nameToAddToManager[enabledName]; !ok {
			return fmt.Errorf("cannot enable unknown controller %q", enabledName)
		}
		enabled.Insert(enabledName)
	}

	for _, disabledName := range d.Disabled {
		if _, ok := d.nameToAddToManager[disabledName]; !ok {
			return fmt.Errorf("cannot disable unknown controller %q", disabledName)
		}
		disabled.Insert(disabledName)
	}

	for name, addToManager := range d.nameToAddToManager {
		if enabled.Has(name) && !disabled.Has(name) {
			d.addToManagerBuilder.Register(addToManager)
		}
	}
	return nil
}

// Completed returns the completed SwitchConfig. Call this only after successfully calling `Completed`.
func (d *SwitchOptions) Completed() *SwitchConfig {
	return &SwitchConfig{d.addToManagerBuilder.AddToManager}
}

// SwitchConfig is the completed configuration of SwitchOptions.
type SwitchConfig struct {
	AddToManager func(context.Context, manager.Manager) error
}

// GeneralOptions are command line options that can be set for general configuration.
type GeneralOptions struct {
	// GardenerVersion is the version of the Gardener.
	GardenerVersion string
	// AutonomousShootCluster indicates whether the extension runs in an autonomous shoot cluster.
	AutonomousShootCluster bool

	config *GeneralConfig
}

// GeneralConfig is a completed general configuration.
type GeneralConfig struct {
	// GardenerVersion is the version of the Gardener.
	GardenerVersion string
	// AutonomousShootCluster indicates whether the extension runs in an autonomous shoot cluster.
	AutonomousShootCluster bool
}

// Complete implements Complete.
func (r *GeneralOptions) Complete() error {
	r.config = &GeneralConfig{r.GardenerVersion, r.AutonomousShootCluster}
	return nil
}

// Completed returns the completed GeneralConfig. Only call this if `Complete` was successful.
func (r *GeneralOptions) Completed() *GeneralConfig {
	return r.config
}

// AddFlags implements Flagger.AddFlags.
func (r *GeneralOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&r.GardenerVersion, GardenerVersionFlag, "", "Version of the gardenlet.")
	fs.BoolVar(&r.AutonomousShootCluster, AutonomousShootClusterFlag, false, "Does the extension run in an autonomous shoot cluster?")
}
