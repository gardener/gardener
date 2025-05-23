// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/pflag"
)

// WebhookOptions are command line options for this webhook.
type WebhookOptions struct {
	RemoteWriteURLs []string
	ExternalLabels  MapFlag

	config *WebhookConfig
}

// AddFlags implements Flagger.AddFlags.
func (w *WebhookOptions) AddFlags(fs *pflag.FlagSet) {
	if w.ExternalLabels == nil {
		w.ExternalLabels = MapFlag{}
	}

	fs.StringSliceVar(&w.RemoteWriteURLs, "remote-write-url", w.RemoteWriteURLs, "Remote write URLs to inject into Prometheus objects")
	fs.Var(&w.ExternalLabels, "external-labels", "External labels to inject into Prometheus objects")
}

// Complete implements Completer.Complete.
func (w *WebhookOptions) Complete() error {
	w.config = &WebhookConfig{
		RemoteWriteURLs: w.RemoteWriteURLs,
		ExternalLabels:  w.ExternalLabels,
	}
	return nil
}

// Completed returns the completed WebhookConfig. Only call this if `Complete` was successful.
func (w *WebhookOptions) Completed() *WebhookConfig {
	return w.config
}

// WebhookConfig is a completed webhook configuration.
type WebhookConfig struct {
	RemoteWriteURLs []string
	ExternalLabels  map[string]string
}

// Apply sets the values of this WebhookConfig in the given AddOptions.
func (w *WebhookConfig) Apply(opts *AddOptions) {
	opts.RemoteWriteURLs = w.RemoteWriteURLs
	opts.ExternalLabels = w.ExternalLabels
}

// MapFlag is a pflag.Value that accepts a map in comma-separated key-value pair representation, e.g.,
// "key1=value1,key2=value2,...".
type MapFlag map[string]string

// Set parses a string of the form "key1=value1,key2=value2,..." into a map[string]string.
func (m MapFlag) Set(value string) error {
	for _, pair := range strings.Split(value, ",") {
		if len(pair) == 0 {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		key := strings.TrimSpace(parts[0])
		if len(parts) != 2 {
			return fmt.Errorf("missing value for key %q", key)
		}
		m[key] = strings.TrimSpace(parts[1])
	}

	return nil
}

// String returns a string containing all map values, formatted as "key1=value1,key2=value2,...".
func (m MapFlag) String() string {
	if m == nil {
		return ""
	}

	var pairs []string
	for k, v := range m {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(pairs)
	return strings.Join(pairs, ",")
}

// Type implements the pflag.Value interface.
func (m MapFlag) Type() string {
	return "mapStringString"
}
