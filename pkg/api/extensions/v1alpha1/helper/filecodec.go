// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"encoding/base64"
	"fmt"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var validFileCodecIDs = map[extensionsv1alpha1.FileCodecID]struct{}{
	extensionsv1alpha1.PlainFileCodecID: {},
	extensionsv1alpha1.B64FileCodecID:   {},
}

// FileCodec is a codec to en- and decode data in cloud-init scripts with.j
type FileCodec interface {
	Encode([]byte) ([]byte, error)
	Decode([]byte) ([]byte, error)
}

var (
	// PlainFileCodec is a noop FileCodec.
	PlainFileCodec FileCodec = plainFileCodec{}
	// B64FileCodec is the base64 FileCodec.
	B64FileCodec FileCodec = b64FileCodec{}
)

type plainFileCodec struct{}

func (plainFileCodec) Encode(data []byte) ([]byte, error) {
	return data, nil
}
func (plainFileCodec) Decode(data []byte) ([]byte, error) {
	return data, nil
}

type b64FileCodec struct{}

var encoding = base64.StdEncoding

func (b64FileCodec) Encode(data []byte) ([]byte, error) {
	dst := make([]byte, encoding.EncodedLen(len(data)))
	encoding.Encode(dst, data)
	return dst, nil
}

func (b64FileCodec) Decode(data []byte) ([]byte, error) {
	dst := make([]byte, encoding.DecodedLen(len(data)))
	n, err := encoding.Decode(dst, data)
	return dst[:n], err
}

// ParseFileCodecID tries to parse a string into a FileCodecID.
func ParseFileCodecID(s string) (extensionsv1alpha1.FileCodecID, error) {
	id := extensionsv1alpha1.FileCodecID(s)
	if _, ok := validFileCodecIDs[id]; !ok {
		return id, fmt.Errorf("invalid file codec id %q", id)
	}
	return id, nil
}

var fileCodecIDToFileCodec = map[extensionsv1alpha1.FileCodecID]FileCodec{
	extensionsv1alpha1.PlainFileCodecID: PlainFileCodec,
	extensionsv1alpha1.B64FileCodecID:   B64FileCodec,
}

// FileCodecForID retrieves the FileCodec for the given FileCodecID.
func FileCodecForID(id extensionsv1alpha1.FileCodecID) FileCodec {
	return fileCodecIDToFileCodec[id]
}

// Decode decodes the given data using the codec from resolving the given codecIDString.
// It's a shorthand for parsing the FileCodecID and calling the `Decode` method on the obtained
// FileCodec.
func Decode(codecIDString string, data []byte) ([]byte, error) {
	id, err := ParseFileCodecID(codecIDString)
	if err != nil {
		return nil, err
	}

	return FileCodecForID(id).Decode(data)
}
