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
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/apache/iceberg-go/io"
	"github.com/stretchr/testify/require"
)

func TestRegistryPreservesNativePathCharacters(t *testing.T) {
	reg := io.NewRegistry()
	reg.Unregister("")

	var receivedPath string
	require.NoError(t, reg.Register("", func(_ context.Context, parsed *url.URL, _ map[string]string) (io.IO, error) {
		receivedPath = parsed.Path

		return io.LocalFS{}, nil
	}))

	for _, path := range []string{
		filepath.Join(t.TempDir(), "100% complete", "metadata.json"),
		filepath.Join(t.TempDir(), "literal%20table", "metadata#1.json"),
		filepath.Join(t.TempDir(), "table?query", "metadata.json"),
		"./partition:2026/data.parquet",
	} {
		t.Run(path, func(t *testing.T) {
			_, err := reg.LoadFS(t.Context(), nil, path)
			require.NoError(t, err)
			require.Equal(t, path, receivedPath)
		})
	}
}

func TestRegistryCanWriteNativePathContainingPercent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "100% complete", "metadata.json")
	fs, err := io.LoadFS(t.Context(), nil, path)
	require.NoError(t, err)

	writer, ok := fs.(io.WriteFileIO)
	require.True(t, ok)
	require.NoError(t, writer.WriteFile(t.Context(), path, []byte("metadata")))

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, []byte("metadata"), got)
}

func TestRegistryStillRecognizesOpaqueRegisteredScheme(t *testing.T) {
	reg := io.NewRegistry()
	var received *url.URL
	require.NoError(t, reg.Register("custom-opaque-path", func(_ context.Context, parsed *url.URL, _ map[string]string) (io.IO, error) {
		received = parsed

		return io.LocalFS{}, nil
	}))

	_, err := reg.LoadFS(t.Context(), nil, "custom-opaque-path:resource")
	require.NoError(t, err)
	require.NotNil(t, received)
	require.Equal(t, "custom-opaque-path", received.Scheme)
	require.Equal(t, "resource", received.Opaque)

	reg.Unregister("s3")
	_, err = reg.LoadFS(t.Context(), nil, "s3:bucket/key")
	require.ErrorIs(t, err, io.ErrIOSchemeNotFound)

	_, err = reg.LoadFS(t.Context(), nil, "unregistered-scheme:resource")
	require.ErrorIs(t, err, io.ErrIOSchemeNotFound)
}
