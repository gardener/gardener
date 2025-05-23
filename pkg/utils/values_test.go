// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/utils"
)

type object struct {
	Object  *object  `json:"object,omitempty"`
	Objects []object `json:"objects,omitempty"`
	String  *string  `json:"string,omitempty"`
	Int     *int32   `json:"int,omitempty"`
	Bool    *bool    `json:"bool,omitempty"`
}

// This is for instance an internal type which does not have json marshalling annotations
type objectUpperCase struct {
	Object     *objectUpperCase
	Objects    []objectUpperCase
	String     *string
	Int        *int32
	Bool       *bool
	BoolWithMe *bool
}

var _ = Describe("Values", func() {
	var (
		obj      *object
		objUpper *objectUpperCase
		values   map[string]any
	)

	BeforeEach(func() {
		obj = &object{
			Objects: []object{
				{
					Object: &object{
						String: ptr.To("foo"),
					},
					Int: ptr.To[int32](42),
				},
			},
			Bool: ptr.To(true),
		}

		objUpper = &objectUpperCase{
			Objects: []objectUpperCase{
				{
					Object: &objectUpperCase{
						String: ptr.To("foo"),
					},
					Int: ptr.To[int32](42),
				},
			},
			Bool: ptr.To(true),
		}

		values = map[string]any{
			"objects": []any{
				map[string]any{
					"object": map[string]any{
						"string": "foo",
					},
					"int": float64(42),
				},
			},
			"bool": true,
		}
	})

	Describe("#ToValuesMap", func() {
		It("should convert an object to a values map", func() {
			result, err := ToValuesMap(obj)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(values))
		})

		It("should convert an empty object to an empty values map", func() {
			result, err := ToValuesMap(&object{})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(map[string]any{}))
		})

		It("should convert nil to a nil values map", func() {
			result, err := ToValuesMap(nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if the object cannot be marshalled to JSON", func() {
			_, err := ToValuesMap(func() {})
			Expect(err).To(HaveOccurred())
		})

		It("should fail if the object cannot be unmarshalled back to a values map", func() {
			_, err := ToValuesMap("foo")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#ToValuesMapWithOptions", func() {
		It("should convert an object to a values map with lower-case keys", func() {
			result, err := ToValuesMapWithOptions(objUpper, Options{LowerCaseKeys: true})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(values))
		})

		It("should convert an object to a values map with lower-case keys - only the first letter should be changed", func() {
			objUpper.BoolWithMe = ptr.To(true)
			result, err := ToValuesMapWithOptions(objUpper, Options{LowerCaseKeys: true})
			Expect(err).ToNot(HaveOccurred())
			values["boolWithMe"] = true
			Expect(result).To(Equal(values))
		})

		It("should convert an object to a values map with lower-case keys - deep recursion", func() {
			objUpper = &objectUpperCase{
				Objects: []objectUpperCase{
					{
						Object: &objectUpperCase{
							String: ptr.To("foo"),
						},
						Objects: []objectUpperCase{
							{
								Int: ptr.To[int32](50),
								Object: &objectUpperCase{
									String: ptr.To("bar"),
								},
							},
						},
						Int: ptr.To[int32](42),
					},
				},
				Bool: ptr.To(true),
			}

			values = map[string]any{
				"objects": []any{
					map[string]any{
						"object": map[string]any{
							"string": "foo",
						},
						"objects": []any{
							map[string]any{
								"object": map[string]any{
									"string": "bar",
								},
								"int": float64(50),
							},
						},
						"int": float64(42),
					},
				},
				"bool": true,
			}

			result, err := ToValuesMapWithOptions(objUpper, Options{LowerCaseKeys: true})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(values))
		})

		It("should convert an object to a values map removing entries with zero values", func() {
			obj.String = ptr.To("")
			result, err := ToValuesMapWithOptions(obj, Options{RemoveZeroEntries: true})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(values))
		})

		It("should convert an object to a values map containing empty entries", func() {
			obj.String = ptr.To("")

			result, err := ToValuesMapWithOptions(obj, Options{RemoveZeroEntries: false})
			Expect(err).ToNot(HaveOccurred())
			values["string"] = ""
			Expect(result).To(Equal(values))
		})

		It("should convert an object to a values map with nested slices", func() {
			obj.String = ptr.To("")

			obj = &object{
				Objects: []object{
					{
						Object: &object{
							String: ptr.To("one"),
							Objects: []object{
								{
									String: ptr.To("two-l1"),
									Objects: []object{
										{
											String: ptr.To(""),
											Int:    ptr.To[int32](3),
										},
									},
								},
								{
									String: ptr.To("two-l2"),
									Objects: []object{
										{
											Int: ptr.To[int32](4),
										},
									},
								},
							},
						},
						Int: ptr.To[int32](42),
					},
				},
				Bool: ptr.To(true),
			}

			values = map[string]any{
				"objects": []any{
					map[string]any{
						"object": map[string]any{
							"string": "one",
							"objects": []any{
								map[string]any{
									"string": "two-l1",
									"objects": []any{
										map[string]any{
											// empty string removed
											"int": float64(3),
										},
									},
								},
								map[string]any{
									"string": "two-l2",
									"objects": []any{
										map[string]any{
											"int": float64(4),
										},
									},
								},
							},
						},
						"int": float64(42),
					},
				},
				"bool": true,
			}

			result, err := ToValuesMapWithOptions(obj, Options{RemoveZeroEntries: true})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(values))
		})
	})

	Describe("#FromValuesMap", func() {
		var result *object

		BeforeEach(func() {
			result = nil
		})

		It("should convert a values map to an object", func() {
			err := FromValuesMap(values, &result)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(obj))
		})

		It("should convert an empty values map to an empty object", func() {
			err := FromValuesMap(map[string]any{}, &result)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(&object{}))
		})

		It("should convert a nil values map to nil", func() {
			err := FromValuesMap(nil, &result)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if the values map cannot be marshalled to JSON", func() {
			err := FromValuesMap(map[string]any{"foo": func() {}}, &result)
			Expect(err).To(HaveOccurred())
		})

		It("should fail if the values map cannot be unmarshalled back to an object", func() {
			err := FromValuesMap(map[string]any{"object": "foo"}, &result)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#InitValuesMap", func() {
		It("should return the given values map if it is not nil", func() {
			Expect(InitValuesMap(values)).To(Equal(values))
		})

		It("should return a new values map if the given values map is nil", func() {
			Expect(InitValuesMap(nil)).To(Equal(map[string]any{}))
		})
	})

	Describe("#GetFromValuesMap", func() {
		It("should return the element at the specified location in the given values map", func() {
			result, err := GetFromValuesMap(values, "objects", 0, "object")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(map[string]any{"string": "foo"}))
		})

		It("should return nil if a map key doesn't exist", func() {
			result, err := GetFromValuesMap(values, "foo", "bar")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should return nil if a slice index doesn't exist", func() {
			result, err := GetFromValuesMap(values, "objects", 1, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should return nil with a nil values map", func() {
			result, err := GetFromValuesMap(nil, "foo")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should return the given values map with no keys", func() {
			result, err := GetFromValuesMap(values)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(values))
		})

		It("should fail if a string key is specified but its element is not a map", func() {
			result, err := GetFromValuesMap(values, "objects", "foo")
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if an int key is specified but its element is not a slice", func() {
			result, err := GetFromValuesMap(values, 0)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if a key is of type neither string nor int", func() {
			result, err := GetFromValuesMap(values, true)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Describe("#SetToValuesMap", func() {
		It("should set the element at the specified location in the given values map", func() {
			result, err := SetToValuesMap(values, map[string]any{"foo": "bar"}, "objects", 0, "object")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(map[string]any{
				"objects": []any{
					map[string]any{
						"object": map[string]any{
							"foo": "bar",
						},
						"int": float64(42),
					},
				},
				"bool": true,
			}))
		})

		It("should create the element if a map key doesn't exist", func() {
			result, err := SetToValuesMap(values, map[string]any{"foo": "bar"}, "foo", "bar")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(map[string]any{
				"objects": []any{
					map[string]any{
						"object": map[string]any{
							"string": "foo",
						},
						"int": float64(42),
					},
				},
				"bool": true,
				"foo": map[string]any{
					"bar": map[string]any{
						"foo": "bar",
					},
				},
			}))
		})

		It("should create the element if a slice index doesn't exist", func() {
			result, err := SetToValuesMap(values, map[string]any{"foo": "bar"}, "objects", 1, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(map[string]any{
				"objects": []any{
					map[string]any{
						"object": map[string]any{
							"string": "foo",
						},
						"int": float64(42),
					},
					[]any{
						map[string]any{
							"foo": "bar",
						},
					},
				},
				"bool": true,
			}))
		})

		It("should create a new values map with a nil values map", func() {
			result, err := SetToValuesMap(nil, map[string]any{"foo": "bar"}, "foo")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(map[string]any{
				"foo": map[string]any{
					"foo": "bar",
				},
			}))
		})

		It("should return the given values map with no keys", func() {
			result, err := SetToValuesMap(values, map[string]any{"foo": "bar"})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(values))
		})

		It("should return nil with a nil values map and no keys", func() {
			result, err := SetToValuesMap(nil, map[string]any{"foo": "bar"})
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should fail if a string key is specified but its element is not a map", func() {
			result, err := SetToValuesMap(values, nil, "objects", "foo")
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(values))
		})

		It("should fail if an int key is specified but its element is not a slice", func() {
			result, err := SetToValuesMap(values, nil, 0)
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(values))
		})

		It("should fail if a key is of type neither string nor int", func() {
			result, err := SetToValuesMap(values, nil, true)
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(values))
		})

		It("should fail if an index is out of range", func() {
			result, err := SetToValuesMap(values, nil, "objects", 2, "object")
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(values))
		})
	})

	Describe("#DeleteFromValuesMap", func() {
		It("should delete the element at the specified location in the given values map", func() {
			result, err := DeleteFromValuesMap(values, "objects", 0, "object")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(map[string]any{
				"objects": []any{
					map[string]any{
						"int": float64(42),
					},
				},
				"bool": true,
			}))
		})

		It("should return the given values map if a map key doesn't exist", func() {
			result, err := DeleteFromValuesMap(values, "foo", "bar")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(values))
		})

		It("should return the given values map if a slice index doesn't exist", func() {
			result, err := DeleteFromValuesMap(values, "objects", 1, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(values))
		})

		It("should return nil with a nil values map", func() {
			result, err := DeleteFromValuesMap(nil, "foo")
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should return the given values map with no keys", func() {
			result, err := DeleteFromValuesMap(values)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(values))
		})

		It("should fail if a string key is specified but its element is not a map", func() {
			result, err := DeleteFromValuesMap(values, "objects", "foo")
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(values))
		})

		It("should fail if an int key is specified but its element is not a slice", func() {
			result, err := DeleteFromValuesMap(values, 0)
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(values))
		})

		It("should fail if a key is of type neither string nor int", func() {
			result, err := DeleteFromValuesMap(values, true)
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal(values))
		})
	})
})
