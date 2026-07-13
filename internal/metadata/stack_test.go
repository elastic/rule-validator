// Licensed to Elasticsearch B.V. under one or more agreements.
// Elasticsearch B.V. licenses this file to you under the Apache 2.0 License.
// See the LICENSE file in the project root for more information.

package metadata

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseTime(layout, value string) time.Time {
	t, err := time.Parse(layout, value)
	if err != nil {
		panic(err)
	}

	return t
}

func TestGetStackReleases(t *testing.T) {
	pastReleaseData, err := os.ReadFile("testdata/past-releases.json")
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/public/past-releases.json" {
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(pastReleaseData)
			require.NoError(t, err)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	got, err := GetStackRelease(WithURL(srv.URL + "/public/past-releases.json"))
	require.NoError(t, err)

	want := []StackRelease{
		{
			Name:       "9.4",
			First:      semver.MustParse("9.4.0"),
			Latest:     semver.MustParse("9.4.3"),
			FirstDate:  mustParseTime("2006-01-02", "2026-05-05"),
			LatestDate: mustParseTime("2006-01-02", "2026-06-30"),
		},
		{
			Name:       "9.3",
			First:      semver.MustParse("9.3.0"),
			Latest:     semver.MustParse("9.3.7"),
			FirstDate:  mustParseTime("2006-01-02", "2026-02-03"),
			LatestDate: mustParseTime("2006-01-02", "2026-06-30"),
		},
		{
			Name:       "9.2",
			First:      semver.MustParse("9.2.0"),
			Latest:     semver.MustParse("9.2.8"),
			FirstDate:  mustParseTime("2006-01-02", "2025-10-23"),
			LatestDate: mustParseTime("2006-01-02", "2026-04-08"),
		},
		{
			Name:       "8.19",
			First:      semver.MustParse("8.19.0"),
			Latest:     semver.MustParse("8.19.18"),
			FirstDate:  mustParseTime("2006-01-02", "2025-07-29"),
			LatestDate: mustParseTime("2006-01-02", "2026-06-30"),
		},
	}

	require.Len(t, got, len(want))
	// Note: "assert.Equal(t, want, got)" doesn't work on pointer fields.
	for i, v := range want {
		assert.Equal(t, v.Name, got[i].Name)
		assert.True(t, v.First.Equal(got[i].First), "[%d] %v != %v", i, v.First, got[i].First)
		assert.True(t, v.Latest.Equal(got[i].Latest), "[%d] %v != %v", i, v.Latest, got[i].Latest)
		assert.Equal(t, v.FirstDate, got[i].FirstDate)
		assert.Equal(t, v.LatestDate, got[i].LatestDate)
	}
}
