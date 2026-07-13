// Licensed to Elasticsearch B.V. under one or more agreements.
// Elasticsearch B.V. licenses this file to you under the Apache 2.0 License.
// See the LICENSE file in the project root for more information.

package ecs

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/rule-validator/internal/cache"
	"github.com/elastic/rule-validator/internal/field"
)

func setupServer(t *testing.T) *httptest.Server {
	versionData940, err := os.ReadFile("testdata/v9.4.0/ecs_version")
	require.NoError(t, err)

	schemaData940, err := os.ReadFile("testdata/v9.4.0/ecs_flat.yml")
	require.NoError(t, err)

	versionData930, err := os.ReadFile("testdata/v9.3.0/ecs_version")
	require.NoError(t, err)

	schemaData930, err := os.ReadFile("testdata/v9.3.0/ecs_flat.yml")
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/elastic/ecs/v9.4.0/version" {
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(versionData940)
			require.NoError(t, err)
			return
		}
		if r.URL.Path == "/elastic/ecs/v9.4.0/generated/ecs/ecs_flat.yml" {
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(schemaData940)
			require.NoError(t, err)
			return
		}
		if r.URL.Path == "/elastic/ecs/v9.3.0/version" {
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(versionData930)
			require.NoError(t, err)
			return
		}
		if r.URL.Path == "/elastic/ecs/v9.3.0/generated/ecs/ecs_flat.yml" {
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(schemaData930)
			require.NoError(t, err)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))

	return srv
}

func TestResolver_Resolve(t *testing.T) {
	tests := []struct {
		name      string
		ref       string
		fieldName string
		want      field.Field
		wantErr   bool
	}{
		{
			name:      "valid-v9.4.0-related-hosts",
			ref:       "v9.4.0",
			fieldName: "related.hosts",
			want: field.Field{
				Name: "related.hosts",
				Type: field.TypeKeyword,
			},
		},
		{
			name:      "valid-v9.3.0-destination-ip",
			ref:       "v9.3.0",
			fieldName: "destination.ip",
			want: field.Field{
				Name: "destination.ip",
				Type: field.TypeIP,
			},
		},
		{
			name:      "missing-ref",
			ref:       "v9.2.0",
			fieldName: "related.hosts",
			wantErr:   true,
		},
		{
			name:      "missing-field",
			ref:       "v9.4.0",
			fieldName: "related.user",
			wantErr:   true,
		},
	}

	tmpDir, err := os.MkdirTemp("", "rule-validator-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	srv := setupServer(t)
	defer srv.Close()

	c, err := cache.New(tmpDir)
	require.NoError(t, err)

	resolver := NewResolver(WithBaseURL(srv.URL), WithCache(c))

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, gotErr := resolver.Lookup(tc.ref, tc.fieldName)

			if tc.wantErr {
				assert.NotNil(t, gotErr)
			} else {
				assert.Nil(t, gotErr)
				assert.Equal(t, tc.want, *got)
			}
		})
	}
}
