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

package table

import (
	"testing"

	"github.com/apache/iceberg-go"
	"github.com/stretchr/testify/require"
)

func TestChangelogTaskConstructorsInitializeFullFileRangeAndRowLineage(t *testing.T) {
	builder, err := iceberg.NewDataFileBuilder(*iceberg.UnpartitionedSpec,
		iceberg.EntryContentData, "data/lineage.parquet", iceberg.ParquetFile,
		nil, nil, nil, 10, 1024)
	require.NoError(t, err)
	data := builder.FirstRowID(123).Build()

	constructors := []struct {
		name    string
		newTask func(iceberg.DataFile) (ChangelogScanTask, error)
	}{
		{"added rows", func(file iceberg.DataFile) (ChangelogScanTask, error) {
			return NewAddedRowsScanTask(file, nil, 0, 1)
		}},
		{"deleted data file", func(file iceberg.DataFile) (ChangelogScanTask, error) {
			return NewDeletedDataFileScanTask(file, nil, 0, 1)
		}},
		{"deleted rows", func(file iceberg.DataFile) (ChangelogScanTask, error) {
			return NewDeletedRowsScanTask(file, nil, nil, 0, 1)
		}},
	}

	for _, tc := range constructors {
		t.Run(tc.name, func(t *testing.T) {
			task, err := tc.newTask(data)
			require.NoError(t, err)
			scan := task.ScanTask()
			require.Equal(t, int64(0), scan.Start)
			require.Equal(t, data.FileSizeBytes(), scan.Length)
			require.Equal(t, data.FirstRowID(), scan.FirstRowID)

			_, err = tc.newTask(nil)
			require.ErrorIs(t, err, ErrInvalidMetadata)
		})
	}
}
