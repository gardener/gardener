// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"
	"fmt"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/utils"
)

// FilesToDiskScript is a utility function which generates a bash script for writing the provided files to the disk.
func FilesToDiskScript(ctx context.Context, reader client.Reader, namespace string, files []extensionsv1alpha1.File) (string, error) {
	var out string

	for _, file := range files {
		data, err := dataForFileContent(ctx, reader, namespace, &file.Content)
		if err != nil {
			return "", err
		}

		out += `
mkdir -p "` + path.Dir(file.Path) + `"
` + catDataIntoFile(file.Path, data, ptr.Deref(file.Content.TransmitUnencoded, false))

		if file.Permissions != nil {
			out += `
` + fmt.Sprintf(`chmod "%04o" "%s"`, *file.Permissions, file.Path)
		}
	}

	return out, nil
}

// UnitsToDiskScript is a utility function which generates a bash script for writing the provided units and their
// drop-ins to the disk.
func UnitsToDiskScript(units []extensionsv1alpha1.Unit) string {
	var out string

	for _, unit := range units {
		unitFilePath := path.Join("/", "etc", "systemd", "system", unit.Name)

		if unit.Content != nil {
			out += `
` + catDataIntoFile(unitFilePath, []byte(*unit.Content), false)
		}

		if len(unit.DropIns) > 0 {
			unitDropInsDirectoryPath := unitFilePath + ".d"
			out += `
mkdir -p "` + unitDropInsDirectoryPath + `"`

			for _, dropIn := range unit.DropIns {
				out += `
` + catDataIntoFile(path.Join(unitDropInsDirectoryPath, dropIn.Name), []byte(dropIn.Content), false)
			}
		}
	}

	return out
}

// WrapProvisionOSCIntoOneshotScript wraps the given script into an oneshot script which exits early when it is called again after finishing successfully.
func WrapProvisionOSCIntoOneshotScript(script string) string {
	var wrappedLines []string

	lines := strings.Split(script, "\n")

	for i, line := range lines {
		if strings.HasPrefix(line, "#") {
			wrappedLines = append(wrappedLines, line)
			continue
		}

		wrappedLines = append(wrappedLines,
			`if [ -f "/var/lib/osc/provision-osc-applied" ]; then`,
			`  echo "Provision OSC already applied, exiting..."`,
			`  exit 0`,
			`fi`,
			``,
		)

		wrappedLines = append(wrappedLines, lines[i:]...)

		break
	}

	wrappedLines = append(wrappedLines,
		`mkdir -p /var/lib/osc`,
		`touch /var/lib/osc/provision-osc-applied`,
		``,
	)

	return strings.Join(wrappedLines, "\n")
}

func dataForFileContent(ctx context.Context, c client.Reader, namespace string, content *extensionsv1alpha1.FileContent) ([]byte, error) {
	if inline := content.Inline; inline != nil {
		return extensionsv1alpha1helper.Decode(inline.Encoding, []byte(inline.Data))
	}

	secret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: content.SecretRef.Name}, secret); err != nil {
		return nil, err
	}

	return secret.Data[content.SecretRef.DataKey], nil
}

func catDataIntoFile(path string, data []byte, transmitUnencoded bool) string {
	if transmitUnencoded {
		return `
cat << EOF > "` + path + `"
` + string(data) + `
EOF`
	}

	return `
cat << EOF | base64 -d > "` + path + `"
` + utils.EncodeBase64(data) + `
EOF`
}
