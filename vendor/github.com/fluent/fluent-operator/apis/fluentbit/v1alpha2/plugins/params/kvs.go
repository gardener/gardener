package params

import (
	"bytes"
	"fmt"
	"sort"
)

type kvTransformFunc func(string, string) (string, string)

type KVs struct {
	keys    []string
	values  []string
	Content string
}

func NewKVs() *KVs {
	return &KVs{
		keys:   []string{},
		values: []string{},
	}
}

func (kvs *KVs) Insert(key, value string) {
	kvs.keys = append(kvs.keys, key)
	kvs.values = append(kvs.values, value)
}

func (kvs *KVs) InsertStringMap(m map[string]string, f kvTransformFunc) {
	if len(m) > 0 {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for _, k := range keys {
			v := m[k]
			if f != nil {
				k, v = f(k, v)
			}
			kvs.Insert(k, v)
		}
	}
}

func (kvs *KVs) Merge(tail *KVs) {
	kvs.keys = append(kvs.keys, tail.keys...)
	kvs.values = append(kvs.values, tail.values...)
}

func (kvs *KVs) String() string {
	if kvs == nil {
		return ""
	}

	if kvs.Content != "" {
		return kvs.Content
	}

	var buf bytes.Buffer
	for i := 0; i < len(kvs.keys); i++ {
		buf.WriteString(fmt.Sprintf("    %s    %s\n", kvs.keys[i], kvs.values[i]))
	}
	return buf.String()
}
