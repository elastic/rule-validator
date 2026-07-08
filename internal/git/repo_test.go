// Licensed to Elasticsearch B.V. under one or more agreements.
// Elasticsearch B.V. licenses this file to you under the Apache 2.0 License.
// See the LICENSE file in the project root for more information.

package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustNewRepo(t *testing.T) *Repo {
	// Required to read embedded test bare repository.
	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Setenv("GIT_CONFIG_PARAMETERS", "'safe.bareRepository=explicit'"))
	require.NoError(t, os.Setenv("GIT_DIR", filepath.Join(cwd, "testdata/test-repo.git")))

	r, err := NewRepo(context.Background(), "test-repo", "testdata/test-repo.git", "")
	require.NoError(t, err)

	r.dir = "" // For tests, relying on GIT_DIR env instead of -C <dir> due to safe.bareRepository=explicit.

	return r
}

func TestNewRepo(t *testing.T) {
	// Required to read embedded test bare repository.
	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Setenv("GIT_CONFIG_PARAMETERS", "'safe.bareRepository=explicit'"))
	require.NoError(t, os.Setenv("GIT_DIR", filepath.Join(cwd, "testdata/test-repo.git")))

	r, err := NewRepo(context.Background(), "test-repo", "testdata/test-repo.git", "")
	require.NoError(t, err)

	assert.Equal(t, "test-repo", r.Name())
	assert.Equal(t, "testdata/test-repo.git", r.Dir())

	r.dir = "" // For tests, relying on GIT_DIR env instead of -C <dir> due to safe.bareRepository=explicit.
}

func TestRepo_Tags(t *testing.T) {
	r := mustNewRepo(t)

	tags, err := r.Tags(context.Background())
	require.NoError(t, err)

	want := []string{
		"non-version-tag",
		"v1.0.0",
		"v2.0.0",
	}

	assert.Equal(t, want, tags)
}

func TestRepo_TagToHash(t *testing.T) {
	tests := map[string]struct {
		tag  string
		want string
	}{
		"non-version-tag": {
			tag:  "non-version-tag",
			want: "e6de7000cbb3b449b66000491a9e9045aaf8404a",
		},
		"v1.0.0": {
			tag:  "v1.0.0",
			want: "dec29ea45ceea553b69aabdd0405ba11c8c30474",
		},
		"v2.0.0": {
			tag:  "v2.0.0",
			want: "d1c13afa7c261fd323e5c87228f33c82072a2747",
		},
	}

	r := mustNewRepo(t)

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := r.TagToHash(context.Background(), tc.tag)
			require.NoError(t, err)

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRepo_FileHistory(t *testing.T) {
	tests := map[string]struct {
		filename string
		want     []string
	}{
		"single-commit": {
			filename: "README.md",
			want:     []string{"dec29ea"},
		},
		"multiple-commits": {
			filename: "test.txt",
			want:     []string{"d1c13af", "dec29ea"},
		},
	}

	r := mustNewRepo(t)

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := r.FileHistory(context.Background(), "main", tc.filename)
			require.NoError(t, err)

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRepo_Changeset(t *testing.T) {
	tests := map[string]struct {
		ref  string
		want []string
	}{
		"initial-commit": {
			ref: "dec29ea45ceea553b69aabdd0405ba11c8c30474",
			want: []string{
				"README.md",
				"test.json",
				"test.txt",
			},
		},
		"single-file-change": {
			ref:  "d1c13afa7c261fd323e5c87228f33c82072a2747",
			want: []string{"test.txt"},
		},
		"new-subfolder-file": {
			ref:  "befed6dc855d6b9637d97a38607935656c9f592e",
			want: []string{"folder/test.yml"},
		},
	}

	r := mustNewRepo(t)

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := r.Changeset(context.Background(), tc.ref)
			require.NoError(t, err)

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRepo_ListTree(t *testing.T) {
	tests := map[string]struct {
		ref       string
		dirpath   string
		treeOnly  bool
		recursive bool
		want      []string
	}{
		"root": {
			ref: "main",
			want: []string{
				"README.md",
				"folder",
				"foo.txt",
				"test.json",
				"test.txt",
			},
		},
		"root-treeOnly": {
			ref:      "main",
			treeOnly: true,
			want: []string{
				"folder",
			},
		},
		"root-recursive": {
			ref:       "main",
			recursive: true,
			want: []string{
				"README.md",
				"folder/deep-folder/bar.txt",
				"folder/test.yml",
				"foo.txt",
				"test.json",
				"test.txt",
			},
		},
		"root-treeOnly-recursive": {
			ref:       "main",
			recursive: true,
			treeOnly:  true,
			want: []string{
				"folder",
				"folder/deep-folder",
			},
		},
		"dirPath": {
			ref:     "main",
			dirpath: "folder",
			want: []string{
				"deep-folder",
				"test.yml",
			},
		},
		"dirPath-treeOnly": {
			ref:      "main",
			dirpath:  "folder",
			treeOnly: true,
			want: []string{
				"deep-folder",
			},
		},
		"dirPath-recursive": {
			ref:       "main",
			dirpath:   "folder",
			recursive: true,
			want: []string{
				"deep-folder/bar.txt",
				"test.yml",
			},
		},
		"dirPath-treeOnly-recursive": {
			ref:       "main",
			dirpath:   "folder",
			treeOnly:  true,
			recursive: true,
			want: []string{
				"deep-folder",
			},
		},
	}

	r := mustNewRepo(t)

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := r.ListTree(context.Background(), tc.ref, tc.dirpath, tc.treeOnly, tc.recursive)
			require.NoError(t, err)

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRepo_ReadFile(t *testing.T) {
	tests := map[string]struct {
		ref      string
		filename string
		want     []byte
	}{
		"readme": {
			ref:      "main",
			filename: "README.md",
			want:     []byte("# Test Repo\n\nThis is a test repo.\n"),
		},
		"test-txt-dec29ea": {
			ref:      "dec29ea",
			filename: "test.txt",
			want:     []byte("This is some text.\n"),
		},
		"test-txt-d1c13af": {
			ref:      "d1c13af",
			filename: "test.txt",
			want:     []byte("This is some altered text.\n"),
		},
		"folder-deep-folder-bar-txt": {
			ref:      "main",
			filename: "folder/deep-folder/bar.txt",
			want:     []byte("Another file.\n"),
		},
	}

	r := mustNewRepo(t)

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := r.ReadFile(context.Background(), tc.ref, tc.filename)
			require.NoError(t, err)

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRepo_ParseYAMLFile(t *testing.T) {
	type testStruct struct {
		Alpha string `yaml:"alpha"`
		Beta  string `yaml:"beta"`
	}

	r := mustNewRepo(t)

	var ts testStruct
	err := r.ParseYAMLFile(context.Background(), "main", "folder/test.yml", &ts)
	require.NoError(t, err)

	want := testStruct{
		Alpha: "one",
		Beta:  "two",
	}

	assert.Equal(t, want, ts)
}
