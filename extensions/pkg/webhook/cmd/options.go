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

package cmd

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"

	"github.com/spf13/pflag"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// ModeFlag is the name of the command line flag to specify the webhook config mode.
	ModeFlag = "webhook-config-mode"
	// URLFlag is the name of the command line flag to specify the URL that is used to register the webhooks in Kubernetes.
	URLFlag = "webhook-config-url"
	// ServicePortFlag is the name of the command line flag to specify the service port that exposes the webhook server.
	// If not specified it will fallback to the webhook server port.
	ServicePortFlag = "webhook-config-service-port"
	// NamespaceFlag is the name of the command line flag to specify the webhook config namespace for 'service' mode.
	NamespaceFlag = "webhook-config-namespace"
)

// ServerOptions are command line options that can be set for ServerConfig.
type ServerOptions struct {
	// Mode is the URl that is used to register the webhooks in Kubernetes.
	Mode string
	// URL is the URl that is used to register the webhooks in Kubernetes.
	URL string
	// ServicePort is the service port that exposes the webhook server.
	ServicePort int
	// Namespace is the webhook config namespace for 'service' mode.
	Namespace string

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
	// Namespace is the webhook config namespace for 'service' mode.
	Namespace string
}

// Complete implements Completer.Complete.
func (w *ServerOptions) Complete() error {
	w.config = &ServerConfig{
		Mode:        w.Mode,
		URL:         w.URL,
		ServicePort: w.ServicePort,
		Namespace:   w.Namespace,
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
	fs.StringVar(&w.Namespace, NamespaceFlag, w.Namespace, "The webhook config namespace for 'service' mode.")
}

// DisableFlag is the name of the command line flag to disable individual webhooks.
const DisableFlag = "disable-webhooks"

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
	disabled := sets.NewString()
	for _, disabledName := range w.Disabled {
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
	return &SwitchConfig{WebhooksFactory: w.webhookFactoryAggregator.Webhooks}
}

// SwitchConfig is the completed configuration of SwitchOptions.
type SwitchConfig struct {
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
	serverName string
	Server     ServerOptions
	Switch     SwitchOptions
}

// NewAddToManagerOptions creates new AddToManagerOptions with the given server name, server, and switch options.
// It is supposed to be used for webhooks which should be automatically registered in the cluster via a Mutatingwebhookconfiguration.
func NewAddToManagerOptions(serverName string, serverOpts *ServerOptions, switchOpts *SwitchOptions) *AddToManagerOptions {
	return &AddToManagerOptions{
		serverName: serverName,
		Server:     *serverOpts,
		Switch:     *switchOpts,
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
		serverName: c.serverName,
		Server:     *c.Server.Completed(),
		Switch:     *c.Switch.Completed(),
	}
}

// AddToManagerConfig is a completed AddToManager configuration.
type AddToManagerConfig struct {
	serverName string
	Server     ServerConfig
	Switch     SwitchConfig
}

// AddToManager instantiates all webhooks of this configuration. If there are any webhooks, it creates a
// webhook server, registers the webhooks and adds the server to the manager. Otherwise, it is a no-op.
// It generates and registers the seed targeted webhooks via a Mutatingwebhookconfiguration.
func (c *AddToManagerConfig) AddToManager(mgr manager.Manager) ([]admissionregistrationv1.MutatingWebhook, []admissionregistrationv1.MutatingWebhook, error) {
	ctx := context.Background()

	webhooks, err := c.Switch.WebhooksFactory(mgr)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create webhooks: %w", err)
	}
	webhookServer := mgr.GetWebhookServer()

	servicePort := webhookServer.Port
	if (c.Server.Mode == extensionswebhook.ModeService || c.Server.Mode == extensionswebhook.ModeURLWithServiceName) && c.Server.ServicePort > 0 {
		servicePort = c.Server.ServicePort
	}

	for _, wh := range webhooks {
		if wh.Handler != nil {
			webhookServer.Register("/"+wh.Name, wh.Handler)
		} else {
			webhookServer.Register("/"+wh.Name, wh.Webhook)
		}
	}

	caBundle, err := extensionswebhook.GenerateCertificates(ctx, mgr, webhookServer.CertDir, c.Server.Namespace, c.serverName, c.Server.Mode, c.Server.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("could not generate certificates: %w", err)
	}

	seedWebhooks, shootWebhooks, err := extensionswebhook.RegisterWebhooks(ctx, mgr, c.Server.Namespace, c.serverName, servicePort, c.Server.Mode, c.Server.URL, caBundle, webhooks)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create webhooks: %w", err)
	}

	return seedWebhooks, shootWebhooks, nil
}

// NewAddToManagerSimpleOptions creates new AddToManagerSimpleOptions with the given switch options.
// It can be used for webhooks which are required to run only without an automatic registration in the K8s cluster.
// Hence, Validatingwebhookconfiguration or Mutatingwebhookconfiguration must be created separately.
func NewAddToManagerSimpleOptions(switchOpts *SwitchOptions) *AddToManagerSimpleOptions {
	return &AddToManagerSimpleOptions{
		Switch: *switchOpts,
	}
}

// AddToManagerSimpleOptions are options to create an `AddToManager` function from SwitchOptions.
type AddToManagerSimpleOptions struct {
	Switch SwitchOptions
}

// AddFlags implements Option.
func (o *AddToManagerSimpleOptions) AddFlags(fs *pflag.FlagSet) {
	o.Switch.AddFlags(fs)
}

// Complete implements Option.
func (o *AddToManagerSimpleOptions) Complete() error {
	return o.Switch.Complete()
}

// Completed returns the completed AddToManagerSimpleOptions. Only call this if a previous call to `Complete` succeeded.
func (o *AddToManagerSimpleOptions) Completed() *AddToManagerSimple {
	return &AddToManagerSimple{
		Switch: *o.Switch.Completed(),
	}
}

// AddToManagerSimple is a completed AddToManager configuration w/o webhook registration.
type AddToManagerSimple struct {
	Switch SwitchConfig
}

// AddToManager makes the configured webhooks known to the given manager.
// The registration for these webhooks must happen separately via Validatingwebhookconfiguration or Mutatingwebhookconfiguration.
func (s *AddToManagerSimple) AddToManager(mgr manager.Manager) error {
	webhooks, err := s.Switch.WebhooksFactory(mgr)
	if err != nil {
		return fmt.Errorf("could not create webhooks: %w", err)
	}

	webhookServer := mgr.GetWebhookServer()

	for _, wh := range webhooks {
		if wh.Handler != nil {
			webhookServer.Register(wh.Path, wh.Handler)
		} else {
			webhookServer.Register(wh.Path, wh.Webhook)
		}
	}

	return nil
}
