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
	"slices"
	"testing"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/table"
	"github.com/stretchr/testify/require"
)

func TestScanNestedInitialDefaultRespectsParentValidity(t *testing.T) {
	for _, requiredParent := range []bool{false, true} {
		name := "optional parent"
		if requiredParent {
			name = "required parent"
		}
		t.Run(name, func(t *testing.T) {
			oldSchema := iceberg.NewSchema(0,
				iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true},
				iceberg.NestedField{
					ID: 2, Name: "loc", Required: requiredParent,
					Type: iceberg.NewStructType(
						iceberg.NestedField{ID: 3, Name: "city", Type: iceberg.PrimitiveTypes.String},
					),
				},
			)
			tbl := newEqDeleteReadTestTableWithSchema(t, oldSchema,
				iceberg.Properties{table.PropertyFormatVersion: "3"})
			fileSchema, err := table.SchemaToArrowSchema(oldSchema, nil, false, false)
			require.NoError(t, err)
			dataPath := tbl.Location() + "/data/before-nested-default.parquet"
			rows := `[{"id":1,"loc":{"city":"Berlin"}}]`
			if !requiredParent {
				rows = `[{"id":1,"loc":{"city":"Berlin"}},{"id":2,"loc":null}]`
			}
			writeParquetFile(t, dataPath, fileSchema, rows)

			tx := tbl.NewTransaction()
			require.NoError(t, tx.AddDataFilesByPath(t.Context(), []string{dataPath}, nil, false))
			tbl, err = tx.Commit(t.Context())
			require.NoError(t, err)

			tx = tbl.NewTransaction()
			require.NoError(t, tx.UpdateSchema(true, false).
				AddColumn([]string{"loc", "country"}, iceberg.PrimitiveTypes.String, "", false, iceberg.StringLiteral("DE")).
				Commit())
			tbl, err = tx.Commit(t.Context())
			require.NoError(t, err)

			filterIDs := func(filter iceberg.BooleanExpression) []int64 {
				t.Helper()
				result, scanErr := tbl.Scan(
					table.WithRowFilter(filter), table.WithSelectedFields("id")).ToArrowTable(t.Context())
				require.NoError(t, scanErr)
				defer result.Release()
				ids := make([]int64, 0, int(result.NumRows()))
				for _, chunk := range result.Column(0).Data().Chunks() {
					values := chunk.(*array.Int64)
					for i := range values.Len() {
						ids = append(ids, values.Value(i))
					}
				}
				slices.Sort(ids)

				return ids
			}

			country := iceberg.Reference("loc.country")
			require.Equal(t, []int64{1}, filterIDs(iceberg.EqualTo(country, "DE")))
			require.Equal(t, []int64{1}, filterIDs(iceberg.NotEqualTo(country, "FR")))
			require.Equal(t, []int64{1}, filterIDs(iceberg.NewNot(iceberg.EqualTo(country, "FR"))))
			require.Empty(t, filterIDs(iceberg.NewNot(iceberg.EqualTo(country, "DE"))))
			if !requiredParent {
				require.Equal(t, []int64{2}, filterIDs(iceberg.IsNull(country)))
				require.Equal(t, []int64{1}, filterIDs(iceberg.NotNull(country)))

				// A merge-on-read delete must use the defaulted child as its
				// row predicate, while retaining the row with a null parent.
				tx = tbl.NewTransaction()
				require.NoError(t, tx.SetProperties(iceberg.Properties{
					table.WriteDeleteModeKey: table.WriteModeMergeOnRead,
				}))
				tbl, err = tx.Commit(t.Context())
				require.NoError(t, err)
				tbl, err = tbl.Delete(t.Context(), iceberg.EqualTo(country, "DE"), nil)
				require.NoError(t, err)
				require.Equal(t, []int64{2}, filterIDs(iceberg.AlwaysTrue{}))
			}
		})
	}
}
