// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/spf13/pflag"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/certificates"
	extensionsshootwebhook "github.com/gardener/gardener/extensions/pkg/webhook/shoot"
	"github.com/gardener/gardener/pkg/utils/flow"
)

const (
	// ModeFlag is the name of the command line flag to specify the webhook config mode.
	ModeFlag = "webhook-config-mode"
	// URLFlag is the name of the command line flag to specify the URL that is used to register the webhooks in Kubernetes.
	URLFlag = "webhook-config-url"
	// ServicePortFlag is the name of the command line flag to specify the service port that exposes the webhook server.
	// If not specified it will fall back to the webhook server port.
	ServicePortFlag = "webhook-config-service-port"
	// NamespaceFlag is the name of the command line flag to specify the webhook config namespace where CA bundles, services etc. of the webhook are created.
	NamespaceFlag = "webhook-config-namespace"
	// OwnerNamespaceFlag is the name of the command line flag to specify the namespace which is used as the owner reference for the webhook registration.
	OwnerNamespaceFlag = "webhook-config-owner-namespace"
)

// ServerOptions are command line options that can be set for ServerConfig.
type ServerOptions struct {
	// Mode is the URl that is used to register the webhooks in Kubernetes.
	Mode string
	// URL is the URl that is used to register the webhooks in Kubernetes.
	URL string
	// ServicePort is the service port that exposes the webhook server.
	ServicePort int
	// Namespace is the webhook config namespace where CA bundles, services etc. of the webhook are created.
	Namespace string
	// OwnerNamespace is the namespace which is used as the owner reference for the webhook registration.
	OwnerNamespace string

	config *ServerConfig
}

// ServerConfig is a completed webhook server configuration.
type ServerConfig struct {
	// Mode is the webhook client config mode (service or url).
	Mode string
	// URL is the URL that is used to register the webhooks in Kubernetes.
	URL string
	// ServicePort is the service port that exposes the webhook server.
	ServicePort int
	// Namespace is the webhook config namespace where CA bundles, services etc. of the webhook are created.
	Namespace string
	// OwnerNamespace is the namespace which is used as the owner reference for the webhook registration.
	OwnerNamespace string
}

// Complete implements Completer.Complete.
func (w *ServerOptions) Complete() error {
	if w.OwnerNamespace == "" {
		w.OwnerNamespace = w.Namespace
	}
	w.config = &ServerConfig{
		Mode:           w.Mode,
		URL:            w.URL,
		ServicePort:    w.ServicePort,
		Namespace:      w.Namespace,
		OwnerNamespace: w.OwnerNamespace,
	}

	if len(w.Mode) == 0 {
		w.config.Mode = extensionswebhook.ModeService
	}

	return nil
}

// Completed returns the completed ServerConfig. Only call this if `Complete` was successful.
func (w *ServerOptions) Completed() *ServerConfig {
	return w.config
}

// AddFlags implements Flagger.AddFlags.
func (w *ServerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&w.Mode, ModeFlag, w.Mode, "The webhook mode - either 'url' (when running outside the cluster) or 'service' (when running inside the cluster).")
	fs.StringVar(&w.URL, URLFlag, w.URL, "The webhook URL when running outside of the cluster it is serving.")
	fs.IntVar(&w.ServicePort, ServicePortFlag, w.ServicePort, "The service port that exposes the webhook server.  If not specified it will fallback to the webhook server port.")
	fs.StringVar(&w.Namespace, NamespaceFlag, w.Namespace, "The webhook config namespace where CA bundles, services etc. of the webhook are created.")
	fs.StringVar(&w.OwnerNamespace, OwnerNamespaceFlag, w.OwnerNamespace, fmt.Sprintf("The namespace used for owner reference of the webhook registration. Defaults to %q flag if not set.", NamespaceFlag))
}

const (
	// DisableFlag is the name of the command line flag to disable individual webhooks.
	DisableFlag = "disable-webhooks"
	// disableAllWildcard is the wildcard to disable all webhooks.
	disableAllWildcard = "*"
)

// NameToFactory binds a specific name to a webhook's factory function.
type NameToFactory struct {
	Name string
	Func func(manager.Manager) (*extensionswebhook.Webhook, error)
}

// SwitchOptions are options to build an AddToManager function that filters the disabled webhooks.
type SwitchOptions struct {
	Disabled []string

	nameToWebhookFactory     map[string]func(manager.Manager) (*extensionswebhook.Webhook, error)
	webhookFactoryAggregator FactoryAggregator
}

// Register registers the given NameToWebhookFuncs in the options.
func (w *SwitchOptions) Register(pairs ...NameToFactory) {
	for _, pair := range pairs {
		w.nameToWebhookFactory[pair.Name] = pair.Func
	}
}

// AddFlags implements Option.
func (w *SwitchOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringSliceVar(&w.Disabled, DisableFlag, w.Disabled, "List of webhooks to disable")
}

// Complete implements Option.
func (w *SwitchOptions) Complete() error {
	disabled := sets.New[string]()
	for _, disabledName := range w.Disabled {
		if disabledName == disableAllWildcard {
			return nil
		}
		if _, ok := w.nameToWebhookFactory[disabledName]; !ok {
			return fmt.Errorf("cannot disable unknown webhook %q", disabledName)
		}
		disabled.Insert(disabledName)
	}

	for name, addToManager := range w.nameToWebhookFactory {
		if !disabled.Has(name) {
			w.webhookFactoryAggregator.Register(addToManager)
		}
	}
	return nil
}

// Completed returns the completed SwitchConfig. Call this only after successfully calling `Completed`.
func (w *SwitchOptions) Completed() *SwitchConfig {
	return &SwitchConfig{Disabled: len(w.webhookFactoryAggregator) == 0, WebhooksFactory: w.webhookFactoryAggregator.Webhooks}
}

// SwitchConfig is the completed configuration of SwitchOptions.
type SwitchConfig struct {
	Disabled        bool
	WebhooksFactory func(manager.Manager) ([]*extensionswebhook.Webhook, error)
}

// Switch binds the given name to the given AddToManager function.
func Switch(name string, f func(manager.Manager) (*extensionswebhook.Webhook, error)) NameToFactory {
	return NameToFactory{
		Name: name,
		Func: f,
	}
}

// NewSwitchOptions creates new SwitchOptions with the given initial pairs.
func NewSwitchOptions(pairs ...NameToFactory) *SwitchOptions {
	opts := SwitchOptions{nameToWebhookFactory: map[string]func(manager.Manager) (*extensionswebhook.Webhook, error){}, webhookFactoryAggregator: FactoryAggregator{}}
	opts.Register(pairs...)
	return &opts
}

// AddToManagerOptions are options to create an `AddToManager` function from ServerOptions and SwitchOptions.
type AddToManagerOptions struct {
	extensionName                   string
	shootWebhookManagedResourceName string
	shootNamespaceSelector          map[string]string

	Server ServerOptions
	Switch SwitchOptions
}

// NewAddToManagerOptions creates new AddToManagerOptions with the given server name, server, and switch options.
// It is supposed to be used for webhooks which should be automatically registered in the cluster via a MutatingWebhookConfiguration.
func NewAddToManagerOptions(
	extensionName string,
	shootWebhookManagedResourceName string,
	shootNamespaceSelector map[string]string,
	serverOpts *ServerOptions,
	switchOpts *SwitchOptions,
) *AddToManagerOptions {
	return &AddToManagerOptions{
		extensionName:                   extensionName,
		shootWebhookManagedResourceName: shootWebhookManagedResourceName,
		shootNamespaceSelector:          shootNamespaceSelector,
		Server:                          *serverOpts,
		Switch:                          *switchOpts,
	}
}

// AddFlags implements Option.
func (c *AddToManagerOptions) AddFlags(fs *pflag.FlagSet) {
	c.Switch.AddFlags(fs)
	c.Server.AddFlags(fs)
}

// Complete implements Option.
func (c *AddToManagerOptions) Complete() error {
	if err := c.Switch.Complete(); err != nil {
		return err
	}

	return c.Server.Complete()
}

// Completed returns the completed AddToManagerConfig. Only call this if a previous call to `Complete` succeeded.
func (c *AddToManagerOptions) Completed() *AddToManagerConfig {
	return &AddToManagerConfig{
		extensionName:                   c.extensionName,
		shootWebhookManagedResourceName: c.shootWebhookManagedResourceName,
		shootNamespaceSelector:          c.shootNamespaceSelector,

		Server: *c.Server.Completed(),
		Switch: *c.Switch.Completed(),
	}
}

// AddToManagerConfig is a completed AddToManager configuration.
type AddToManagerConfig struct {
	extensionName                   string
	shootWebhookManagedResourceName string
	shootNamespaceSelector          map[string]string

	Server ServerConfig
	Switch SwitchConfig
	Clock  clock.Clock
}

// AddToManager instantiates all webhooks of this configuration. If there are any webhooks, it creates a
// webhook server, registers the webhooks and adds the server to the manager. Otherwise, it is a no-op.
// It generates and registers the seed targeted webhooks via a MutatingWebhookConfiguration.
func (c *AddToManagerConfig) AddToManager(ctx context.Context, mgr manager.Manager, sourceCluster cluster.Cluster, mergeShootWebhooksIntoSeedWebhooks bool) (*atomic.Value, error) {
	if c.Clock == nil {
		c.Clock = &clock.RealClock{}
	}

	webhooks, err := c.Switch.WebhooksFactory(mgr)
	if err != nil {
		return nil, fmt.Errorf("could not create webhooks: %w", err)
	}
	webhookServer := mgr.GetWebhookServer()

	defaultServer, ok := webhookServer.(*webhook.DefaultServer)
	if !ok {
		return nil, fmt.Errorf("expected *webhook.DefaultServer, got %T", webhookServer)
	}

	servicePort := defaultServer.Options.Port
	if (c.Server.Mode == extensionswebhook.ModeService || c.Server.Mode == extensionswebhook.ModeURLWithServiceName) && c.Server.ServicePort > 0 {
		servicePort = c.Server.ServicePort
	}

	for _, wh := range webhooks {
		path := wh.Path
		if path == "" {
			path = "/" + wh.Name
		} else if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		if wh.Handler != nil {
			webhookServer.Register(path, wh.Handler)
		} else {
			webhookServer.Register(path, wh.Webhook)
		}
	}

	seedWebhookConfigs, shootWebhookConfigs, err := extensionswebhook.BuildWebhookConfigs(
		webhooks,
		mgr.GetClient(),
		c.Server.Namespace,
		c.extensionName,
		servicePort,
		c.Server.Mode,
		c.Server.URL,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create webhooks: %w", err)
	}

	if mergeShootWebhooksIntoSeedWebhooks {
		clientConfig := func(in admissionregistrationv1.WebhookClientConfig) (admissionregistrationv1.WebhookClientConfig, error) {
			var path string
			if in.Service != nil {
				path = ptr.Deref(in.Service.Path, "")
			} else if u := in.URL; u != nil {
				parsedURL, err := url.Parse(*u)
				if err != nil {
					return admissionregistrationv1.WebhookClientConfig{}, fmt.Errorf("failed to parse URL %q: %w", *u, err)
				}
				path = parsedURL.Path
			}

			return extensionswebhook.BuildClientConfigFor(path, c.Server.Namespace, c.extensionName, servicePort, c.Server.Mode, c.Server.URL, nil), nil
		}

		if shootWebhookConfigs.ValidatingWebhookConfig != nil {
			for _, webhook := range shootWebhookConfigs.ValidatingWebhookConfig.Webhooks {
				mutatedClientConfig, err := clientConfig(webhook.ClientConfig)
				if err != nil {
					return nil, fmt.Errorf("failed computing new client config while merging shoot validating webhook %q into seed webhooks: %w", webhook.Name, err)
				}
				webhook.ClientConfig = mutatedClientConfig
				seedWebhookConfigs.ValidatingWebhookConfig.Webhooks = append(seedWebhookConfigs.ValidatingWebhookConfig.Webhooks, webhook)
			}
		}

		if shootWebhookConfigs.MutatingWebhookConfig != nil {
			for _, webhook := range shootWebhookConfigs.MutatingWebhookConfig.Webhooks {
				mutatedClientConfig, err := clientConfig(webhook.ClientConfig)
				if err != nil {
					return nil, fmt.Errorf("failed computing new client config while merging shoot mutating webhook %q into seed webhooks: %w", webhook.Name, err)
				}
				webhook.ClientConfig = mutatedClientConfig
				seedWebhookConfigs.MutatingWebhookConfig.Webhooks = append(seedWebhookConfigs.MutatingWebhookConfig.Webhooks, webhook)
			}
		}
	}

	atomicShootWebhookConfigs := &atomic.Value{}

	if c.Server.Namespace == "" {
		// If the namespace is not set (e.g. when running locally), then we can't use the secrets manager for managing
		// the webhook certificates. We simply generate a new certificate and write it to CertDir in this case.
		mgr.GetLogger().Info("Running webhooks with unmanaged certificates (i.e., the webhook CA will not be rotated automatically). " +
			"This mode is supposed to be used for development purposes only. Make sure to configure --webhook-config-namespace in production.")

		caBundle, err := certificates.GenerateUnmanagedCertificates(c.extensionName, defaultServer.Options.CertDir, c.Server.Mode, c.Server.URL)
		if err != nil {
			return nil, fmt.Errorf("error generating new certificates for webhook server: %w", err)
		}

		for _, webhookConfig := range shootWebhookConfigs.GetWebhookConfigs() {
			if err := extensionswebhook.InjectCABundleIntoWebhookConfig(webhookConfig, caBundle); err != nil {
				return nil, err
			}
		}
		atomicShootWebhookConfigs.Store(shootWebhookConfigs.DeepCopy())

		// register seed webhook config once we become leader – with the CA bundle we just generated
		// also reconcile all shoot webhook configs to update the CA bundle
		if err := mgr.Add(runOnceWithLeaderElection(flow.Sequential(
			c.reconcileSeedWebhookConfig(mgr, seedWebhookConfigs, caBundle),
			c.reconcileShootWebhookConfigs(mgr, shootWebhookConfigs),
		))); err != nil {
			return nil, err
		}

		return atomicShootWebhookConfigs, nil
	}

	// register seed webhook config once we become leader – without CA bundle
	// We only care about registering the desired webhooks here, but not the CA bundle, it will be managed by the
	// reconciler. That's why we also don't reconcile the shoot webhook configs here. They are registered in the
	// ControlPlane actuator and our reconciler will update the included CA bundles if necessary.
	if err := mgr.Add(runOnceWithLeaderElection(
		c.reconcileSeedWebhookConfig(mgr, seedWebhookConfigs, nil),
	)); err != nil {
		return nil, err
	}

	if err := certificates.AddCertificateManagementToManager(
		ctx,
		mgr,
		sourceCluster,
		c.Clock,
		seedWebhookConfigs,
		&shootWebhookConfigs,
		atomicShootWebhookConfigs,
		c.shootNamespaceSelector,
		c.shootWebhookManagedResourceName,
		c.extensionName,
		c.Server.Namespace,
		c.Server.Mode,
		c.Server.URL,
	); err != nil {
		return nil, err
	}

	return atomicShootWebhookConfigs, nil
}

func (c *AddToManagerConfig) reconcileSeedWebhookConfig(mgr manager.Manager, webhookConfigs extensionswebhook.Configs, caBundle []byte) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		for _, webhookConfig := range webhookConfigs.GetWebhookConfigs() {
			if err := extensionswebhook.ReconcileSeedWebhookConfig(ctx, mgr.GetClient(), webhookConfig, c.Server.OwnerNamespace, caBundle); err != nil {
				return fmt.Errorf("error reconciling seed webhook config: %w", err)
			}
		}
		return nil
	}
}

func (c *AddToManagerConfig) reconcileShootWebhookConfigs(mgr manager.Manager, shootWebhookConfigs extensionswebhook.Configs) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if shootWebhookConfigs.HasWebhookConfig() {
			if err := extensionsshootwebhook.ReconcileWebhooksForAllNamespaces(ctx, mgr.GetClient(), c.shootWebhookManagedResourceName, c.shootNamespaceSelector, shootWebhookConfigs); err != nil {
				return fmt.Errorf("error reconciling all shoot webhook configs: %w", err)
			}
		}

		return nil
	}
}

// runOnceWithLeaderElection is a function that is run exactly once when the manager, it is added to, becomes leader.
type runOnceWithLeaderElection func(ctx context.Context) error

func (r runOnceWithLeaderElection) NeedLeaderElection() bool {
	return true
}

func (r runOnceWithLeaderElection) Start(ctx context.Context) error {
	return r(ctx)
}
