// Copyright 2018 The Gardener Authors.
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

package utils

import (
	"bytes"
	"encoding/json"

	"github.com/jmoiron/jsonq"
)

// ConvertJSONToMap parses a byte slice containing valid JSON and returns a JsonQuery object which can be used
// to access arbitrary keys/fields in the JSON.
// See https://github.com/jmoiron/jsonq for detailed information how to access the values.
func ConvertJSONToMap(input []byte) *jsonq.JsonQuery {
	data := map[string]interface{}{}
	dec := json.NewDecoder(bytes.NewReader(input))
	dec.Decode(&data)
	return jsonq.NewQuery(data)
}
