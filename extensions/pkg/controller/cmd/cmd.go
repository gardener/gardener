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
	"os"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	// Log is log.Log. Exposed for testing.
	Log = log.Log
	// Exit calls os.Exit. Exposed for testing.
	Exit = os.Exit
)

// LogErrAndExit logs the given error with msg and keysAndValues and calls `os.Exit(1)`.
func LogErrAndExit(err error, msg string, keysAndValues ...interface{}) {
	Log.Error(err, msg, keysAndValues...)
	Exit(1)
}
