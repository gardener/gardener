// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

// LoginConfig is the dashboard login config structure.
type LoginConfig struct {
	LoginTypes     []string       `json:"loginTypes"`
	LandingPageURL string         `json:"landingPageUrl,omitempty"`
	Branding       map[string]any `json:"branding,omitempty"`
	Themes         map[string]any `json:"themes,omitempty"`
}
