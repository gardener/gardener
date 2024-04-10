// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package config

// Config is the dashboard config structure.
type Config struct {
	Port                                   int32                  `yaml:"port"`
	LogFormat                              string                 `yaml:"logFormat"`
	LogLevel                               string                 `yaml:"logLevel"`
	APIServerURL                           string                 `yaml:"apiServerUrl"`
	MaxRequestBodySize                     string                 `yaml:"maxRequestBodySize"`
	ExperimentalUseWatchCacheForListShoots string                 `yaml:"experimentalUseWatchCacheForListShoots"`
	ReadinessProbe                         ReadinessProbe         `yaml:"readinessProbe"`
	UnreachableSeeds                       UnreachableSeeds       `yaml:"unreachableSeeds"`
	ContentSecurityPolicy                  *ContentSecurityPolicy `yaml:"contentSecurityPolicy,omitempty"`
}

// ReadinessProbe is the readiness probe configuration.
type ReadinessProbe struct {
	PeriodSeconds int `yaml:"periodSeconds"`
}

// UnreachableSeeds is the configuration for unreachable seeds.
type UnreachableSeeds struct {
	MatchLabels map[string]string `yaml:"matchLabels"`
}

// ContentSecurityPolicy is the configuration for the content security policy.
type ContentSecurityPolicy struct {
	ConnectSources []string `yaml:"connectSrc"`
}
