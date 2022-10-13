// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package app

import (
	"fmt"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/gardener/gardener/pkg/logger"
)

type options struct {
	// logLevel defines the level/severity for the logs. Must be one of [info,debug,error]
	logLevel string
	// logFormat defines the format for the logs. Must be one of [json,text]
	logFormat string
}

func (o *options) addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.logLevel, "log-level", "info", "The level/severity for the logs. Must be one of [info,debug,error]")
	fs.StringVar(&o.logFormat, "log-format", "json", "The format for the logs. Must be one of [json,text]")
}

func (o *options) complete() error {
	return nil
}

func (o *options) validate() error {

	if !sets.NewString(logger.AllLogLevels...).Has(o.logLevel) {
		return fmt.Errorf("invalid --log-level: %s", o.logLevel)
	}

	if !sets.NewString(logger.AllLogFormats...).Has(o.logFormat) {
		return fmt.Errorf("invalid --log-format: %s", o.logFormat)
	}

	return nil
}
