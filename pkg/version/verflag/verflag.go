// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package verflag

import (
	"fmt"
	"os"
	"strconv"

	"github.com/gardener/gardener/pkg/version"

	flag "github.com/spf13/pflag"
)

// TODO (ialidzhikov): When we vendor kubernetes-1.19, replace usage of github.com/gardener/gardener/pkg/version and
// github.com/gardener/gardener/pkg/version/verflag with k8s.io/component-base/version and k8s.io/component-base/version/verflag.
// With kubernetes-1.19 the programName is configurable (https://github.com/kubernetes/kubernetes/pull/90139).

type versionValue int

const (
	// VersionFalse represents 'false' value for the version flag
	VersionFalse versionValue = 0
	// VersionTrue represents 'true' value for the version flag
	VersionTrue versionValue = 1
	// VersionRaw represents 'raw' value for the version flag
	VersionRaw versionValue = 2
)

const strRawVersion string = "raw"

// IsBoolFlag is defined to allow the flag to be defined without an argument
func (v *versionValue) IsBoolFlag() bool {
	return true
}

// Get gets the version value for the flag.Getter interface.
func (v *versionValue) Get() interface{} {
	return versionValue(*v)
}

// Set sets the version value for the flag.Value interface.
func (v *versionValue) Set(s string) error {
	if s == strRawVersion {
		*v = VersionRaw
		return nil
	}
	boolVal, err := strconv.ParseBool(s)
	if boolVal {
		*v = VersionTrue
	} else {
		*v = VersionFalse
	}
	return err
}

// String returns a string representation of the version value.
func (v *versionValue) String() string {
	if *v == VersionRaw {
		return strRawVersion
	}
	return fmt.Sprintf("%v", bool(*v == VersionTrue))
}

// Type returns the type of the flag as required by the pflag.Value interface
func (v *versionValue) Type() string {
	return "version"
}

// VersionVar defines the given version flag
func VersionVar(p *versionValue, name string, value versionValue, usage string) {
	*p = value
	flag.Var(p, name, usage)
	// "--version" will be treated as "--version=true"
	flag.Lookup(name).NoOptDefVal = "true"
}

// Version returns a new version flag
func Version(name string, value versionValue, usage string) *versionValue {
	p := new(versionValue)
	VersionVar(p, name, value, usage)
	return p
}

const versionFlagName = "version"

var (
	versionFlag = Version(versionFlagName, VersionFalse, "Print version information and quit")
	programName = "Gardener"
)

// AddFlags registers this package's flags on arbitrary FlagSets, such that they point to the
// same value as the global flags.
func AddFlags(fs *flag.FlagSet) {
	fs.AddFlag(flag.Lookup(versionFlagName))
}

// PrintAndExitIfRequested will check if the --version flag was passed
// and, if so, print the version and exit.
func PrintAndExitIfRequested() {
	if *versionFlag == VersionRaw {
		fmt.Printf("%#v\n", version.Get())
		os.Exit(0)
	} else if *versionFlag == VersionTrue {
		fmt.Printf("%s %s\n", programName, version.Get())
		os.Exit(0)
	}
}
