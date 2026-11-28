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

package io_test

import (
	"context"
	"io"
	"io/fs"
	"strings"
	"testing"

	icebergio "github.com/apache/iceberg-go/io"
	"github.com/stretchr/testify/require"
)

func TestMemFSWriterCloseDoesNotRepublish(t *testing.T) {
	memFS := icebergio.NewMemFS()
	ctx := context.Background()
	name := "mem://bucket/metadata.json"

	writer, err := memFS.Create(ctx, name)
	require.NoError(t, err)
	_, err = writer.Write([]byte("old metadata"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	// A newer writer can replace the file. Closing the old writer again
	// must not bring its stale contents back.
	require.NoError(t, memFS.WriteFile(ctx, name, []byte("new metadata")))
	require.NoError(t, writer.Close())

	file, err := memFS.Open(ctx, name)
	require.NoError(t, err)
	defer file.Close()

	got, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, "new metadata", string(got))
}

func TestMemFSWriterRejectsWritesAfterClose(t *testing.T) {
	memFS := icebergio.NewMemFS()
	ctx := context.Background()
	name := "mem://bucket/data.parquet"

	writer, err := memFS.Create(ctx, name)
	require.NoError(t, err)
	_, err = writer.Write([]byte("original"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	n, err := writer.Write([]byte("appended"))
	require.Zero(t, n)
	require.ErrorIs(t, err, fs.ErrClosed)

	n64, err := writer.ReadFrom(strings.NewReader("appended"))
	require.Zero(t, n64)
	require.ErrorIs(t, err, fs.ErrClosed)

	require.NoError(t, writer.Close())
	file, err := memFS.Open(ctx, name)
	require.NoError(t, err)
	defer file.Close()

	got, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, "original", string(got))
}

func TestMemFSWriterRepeatedClosePreservesFailure(t *testing.T) {
	memFS := icebergio.NewMemFS()
	ctx, cancel := context.WithCancel(context.Background())
	name := "mem://bucket/canceled.parquet"

	writer, err := memFS.Create(ctx, name)
	require.NoError(t, err)
	_, err = writer.Write([]byte("not published"))
	require.NoError(t, err)

	cancel()
	require.ErrorIs(t, writer.Close(), context.Canceled)
	require.ErrorIs(t, writer.Close(), context.Canceled)
	_, err = writer.Write([]byte("late"))
	require.ErrorIs(t, err, fs.ErrClosed)
	_, err = memFS.Open(context.Background(), name)
	require.ErrorIs(t, err, fs.ErrNotExist)
}
