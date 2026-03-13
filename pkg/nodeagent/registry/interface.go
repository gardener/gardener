// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"os"

	"github.com/docker/cli/cli/config/configfile"
)

// Extractor is an interface for extracting files from a container image.
type Extractor interface {
	// CopyFromImage copies a file from a given image reference to the destination file.
	CopyFromImage(ctx context.Context, imageRef string, filePathInImage string, destination string, permissions os.FileMode, configFile *configfile.ConfigFile) error
}
