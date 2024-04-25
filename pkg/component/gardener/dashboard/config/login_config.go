// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package config

// LoginConfig is the dashboard login config structure.
type LoginConfig struct {
	LoginTypes     []string               `json:"loginTypes"`
	LandingPageURL string                 `json:"landingPageUrl,omitempty"`
	Branding       map[string]interface{} `json:"branding,omitempty"`
	Themes         map[string]interface{} `json:"themes,omitempty"`
}
