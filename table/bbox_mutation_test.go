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

package table_test

import (
	"testing"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/table"
	"github.com/stretchr/testify/require"
)

func TestRowMutationsRejectBBoxFilters(t *testing.T) {
	schema := iceberg.NewSchema(0,
		iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true},
		iceberg.NestedField{ID: 2, Name: "geom", Type: iceberg.GeometryType{}},
	)
	tbl := newEqDeleteReadTestTableWithSchema(t, schema,
		iceberg.Properties{table.PropertyFormatVersion: "3"})
	arrowSchema, err := table.SchemaToArrowSchema(schema, nil, false, false)
	require.NoError(t, err)

	bbox := iceberg.BBoxIntersects(iceberg.Reference("geom"),
		iceberg.BoundingBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10})
	other := iceberg.EqualTo(iceberg.Reference("id"), int64(1))
	cases := []struct {
		name   string
		filter iceberg.BooleanExpression
	}{
		{name: "bbox", filter: bbox},
		{name: "negated bbox", filter: iceberg.NewNot(bbox)},
		{name: "bbox and predicate", filter: iceberg.NewAnd(bbox, other)},
		{name: "bbox or predicate", filter: iceberg.NewOr(bbox, other)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx := tbl.NewTransaction()
			err := tx.Delete(t.Context(), tc.filter, nil)
			require.ErrorIs(t, err, iceberg.ErrNotImplemented)
			require.ErrorContains(t, err, "bbox predicates")

			reader, err := array.NewRecordReader(arrowSchema, nil)
			require.NoError(t, err)
			defer reader.Release()

			tx = tbl.NewTransaction()
			err = tx.Overwrite(t.Context(), reader, nil, table.WithOverwriteFilter(tc.filter))
			require.ErrorIs(t, err, iceberg.ErrNotImplemented)
			require.ErrorContains(t, err, "bbox predicates")
		})
	}
}
