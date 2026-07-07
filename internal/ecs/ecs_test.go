// Licensed to Elasticsearch B.V. under one or more agreements.
// Elasticsearch B.V. licenses this file to you under the Apache 2.0 License.
// See the LICENSE file in the project root for more information.

package ecs

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/rule-validator/internal/field"
)

func assertField(t *testing.T, resolver *Resolver, name string, fieldType field.Type) {
	f, ok := resolver.fields[name]
	require.True(t, ok)
	require.NotNil(t, f)

	assert.Equal(t, name, f.Name)
	assert.Equal(t, fieldType, f.Type)
}

func TestNewResolver(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "rule-validator-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	ref := "v9.4.0"
	version := "9.4.0"

	versionData, err := os.ReadFile("testdata/ecs_version")
	require.NoError(t, err)

	schemaData, err := os.ReadFile("testdata/ecs_flat.yml")
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/elastic/ecs/"+ref+"/version" {
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(versionData)
			require.NoError(t, err)
			return
		}
		if r.URL.Path == "/elastic/ecs/"+ref+"/generated/ecs/ecs_flat.yml" {
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(schemaData)
			require.NoError(t, err)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	fmt.Println(srv.URL)

	res, err := NewResolver(ref, WithBaseURL(srv.URL), WithCacheDir(tmpDir))
	require.NoError(t, err)

	assert.Equal(t, ref, res.ref)
	assert.Equal(t, version, res.version)
	assert.Equal(t, 2, len(res.fields))
	assertField(t, res, "related.hosts", field.TypeKeyword)
	assertField(t, res, "source.ip", field.TypeIP)

	cached, err := loadResolver(ref, tmpDir)
	require.NoError(t, err)

	assert.Equal(t, res.ref, cached.ref)
	assert.Equal(t, res.version, cached.version)
	assert.Equal(t, res.fields, cached.fields)
}
