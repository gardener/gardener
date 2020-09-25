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

package flow

import (
	"context"
)

// ProgressReporterFn is continuously called on progress in a flow.
type ProgressReporterFn func(context.Context, *Stats)

// ProgressReporter is used to report the current progress of a flow.
type ProgressReporter interface {
	// Start starts the progress reporter.
	Start(context.Context) error
	// Stop stops the progress reporter.
	Stop()
	// Report reports the progress using the current statistics.
	Report(context.Context, *Stats)
}
