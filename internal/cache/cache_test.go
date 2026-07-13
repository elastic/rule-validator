// Licensed to Elasticsearch B.V. under one or more agreements.
// Elasticsearch B.V. licenses this file to you under the Apache 2.0 License.
// See the LICENSE file in the project root for more information.

package cache

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCache_GetOrCreateRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rule-validator-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	c, err := New(tmpDir)
	require.NoError(t, err)

	firstRepo, err := c.GetOrCreateRepo(t.Context(), "../git/testdata/test-repo.git")
	require.NoError(t, err)
	require.NotNil(t, firstRepo)

	secondRepo, err := c.GetOrCreateRepo(t.Context(), "../git/testdata/test-repo.git")
	require.NoError(t, err)
	require.Equal(t, firstRepo, secondRepo)
}
