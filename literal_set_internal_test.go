// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package iceberg

import (
	"slices"
	"testing"

	"github.com/apache/arrow-go/v18/parquet/variant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLiteralSetVariant(t *testing.T) {
	// build takes the calling (sub)test's t so a failure is reported there.
	build := func(t *testing.T, v any) VariantLiteral {
		t.Helper()

		return VariantLiteral(scalarVariant(t, func(b *variant.Builder) error { return b.Append(v) }))
	}

	t.Run("contains", func(t *testing.T) {
		set := newLiteralSet(build(t, int64(1)), build(t, int64(2)))
		require.Equal(t, 2, set.Len())

		assert.True(t, set.Contains(build(t, int64(1))))
		assert.True(t, set.Contains(build(t, int64(2))))
		assert.False(t, set.Contains(build(t, int64(3))))
	})

	t.Run("same value bytes with different metadata", func(t *testing.T) {
		objA := build(t, map[string]any{"a": int64(1)})
		objB := build(t, map[string]any{"b": int64(1)})
		// Guard the premise: only the metadata dictionary tells these apart.
		require.Equal(t, variant.Value(objA).Bytes(), variant.Value(objB).Bytes())
		require.NotEqual(t, variant.Value(objA).Metadata().Bytes(), variant.Value(objB).Metadata().Bytes())

		set := newLiteralSet(objA, objB)
		assert.Equal(t, 2, set.Len())

		set = newLiteralSet(objA)
		assert.True(t, set.Contains(build(t, map[string]any{"a": int64(1)})))
		assert.False(t, set.Contains(objB))
	})

	t.Run("same concatenated bytes with shifted boundary", func(t *testing.T) {
		// Metadata accepts trailing bytes, so the metadata/value split can
		// move while the concatenation stays the same. addliteral overwrites on
		// a key match without calling Equals, so only the length prefix in
		// variantKey keeps both members.
		left, err := variant.New([]byte{0x01, 0x00, 0x00, 0x0C}, []byte{0x00})
		require.NoError(t, err)
		right, err := variant.New([]byte{0x01, 0x00, 0x00}, []byte{0x0C, 0x00})
		require.NoError(t, err)

		// Guard the premise: distinct values with identical concatenated bytes.
		require.Equal(t,
			slices.Concat(left.Metadata().Bytes(), left.Bytes()),
			slices.Concat(right.Metadata().Bytes(), right.Bytes()))
		require.False(t, VariantLiteral(left).Equals(VariantLiteral(right)))

		set := newLiteralSet(VariantLiteral(left), VariantLiteral(right))
		assert.Equal(t, 2, set.Len())
		assert.True(t, set.Contains(VariantLiteral(left)))
		assert.True(t, set.Contains(VariantLiteral(right)))
	})
}

func TestLiteralSetKeepsDifferentByteLiteralTypes(t *testing.T) {
	// These four literals have identical payloads, but Equals distinguishes
	// binary, fixed, geometry and geography values.
	payload := []byte("1234567890abcdef")
	geometry, err := LiteralFromBytes(GeometryType{}, payload)
	require.NoError(t, err)
	geography, err := LiteralFromBytes(GeographyType{}, payload)
	require.NoError(t, err)

	binary := BinaryLiteral(payload)
	fixed := FixedLiteral(payload)
	literals := []Literal{binary, fixed, geometry, geography}

	set := newLiteralSet(literals...)
	require.Equal(t, len(literals), set.Len())
	for _, literal := range literals {
		assert.True(t, set.Contains(literal), "missing %T", literal)
	}
	assert.ElementsMatch(t, literals, set.Members())

	var all []Literal
	assert.True(t, set.All(func(literal Literal) bool {
		all = append(all, literal)

		return true
	}))
	assert.ElementsMatch(t, literals, all)
	assert.True(t, set.Equals(newLiteralSet(geography, geometry, fixed, binary)))

	// Appending another type with identical bytes must not overwrite the
	// binary value or make an existing member disappear.
	incremental := newLiteralSet(binary)
	incremental.Add(fixed, geometry, geography)
	assert.Equal(t, len(literals), incremental.Len())
	assert.True(t, incremental.Equals(set))
	assert.False(t, newLiteralSet(binary).Contains(fixed))
	assert.False(t, newLiteralSet(geometry).Contains(geography))
}
