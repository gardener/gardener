// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/onsi/gomega/gbytes"
	"k8s.io/cli-runtime/pkg/genericiooptions"
)

// NewTestIOStreams returns a valid genericiooptions.IOStreams for tests, where all streams are a gbytes.Buffer for use
// with the gbytes.Say matcher. Similar to genericiooptions.NewTestIOStreams but for use with gomega.
func NewTestIOStreams() (genericiooptions.IOStreams, *gbytes.Buffer, *gbytes.Buffer, *gbytes.Buffer) {
	in, out, errOut := gbytes.NewBuffer(), gbytes.NewBuffer(), gbytes.NewBuffer()

	return genericiooptions.IOStreams{
		In:     in,
		Out:    out,
		ErrOut: errOut,
	}, in, out, errOut
}
