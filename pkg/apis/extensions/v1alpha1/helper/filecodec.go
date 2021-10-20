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

package helper

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var validFileCodecIDs = map[extensionsv1alpha1.FileCodecID]struct{}{
	extensionsv1alpha1.B64FileCodecID:     {},
	extensionsv1alpha1.GZIPFileCodecID:    {},
	extensionsv1alpha1.GZIPB64FileCodecID: {},
}

// FileCodec is a codec to en- and decode data in cloud-init scripts with.j
type FileCodec interface {
	Encode([]byte) ([]byte, error)
	Decode([]byte) ([]byte, error)
}

var (
	// B64FileCodec is the base64 FileCodec.
	B64FileCodec FileCodec = b64FileCodec{}
	// GZIPFileCodec is the gzip FileCodec.
	GZIPFileCodec FileCodec = gzipFileCodec{}
)

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

type gzipFileCodec struct{}

func (gzipFileCodec) Encode(data []byte) ([]byte, error) {
	var out bytes.Buffer
	w := gzip.NewWriter(&out)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (gzipFileCodec) Decode(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	return io.ReadAll(r)
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
	extensionsv1alpha1.B64FileCodecID:  B64FileCodec,
	extensionsv1alpha1.GZIPFileCodecID: GZIPFileCodec,
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
