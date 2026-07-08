// Licensed to Elasticsearch B.V. under one or more agreements.
// Elasticsearch B.V. licenses this file to you under the Apache 2.0 License.
// See the LICENSE file in the project root for more information.

package metadata

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/Masterminds/semver/v3"
)

const defaultStackReleaseURL = "https://elastic-release-api.s3.us-west-2.amazonaws.com/public/past-releases.json"

type StackRelease struct {
	Name       string          `json:"name"`
	First      *semver.Version `json:"first"`
	Latest     *semver.Version `json:"latest"`
	FirstDate  time.Time       `json:"first_date"`
	LatestDate time.Time       `json:"latest_date"`
}

type pastRelease struct {
	Version *semver.Version `json:"version"`
	Date    date            `json:"public_release_date"`
}

type date struct {
	time.Time
}

func (d *date) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" || s == `""` {
		return nil
	}

	s = s[1 : len(s)-1]
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}

	d.Time = t
	return nil
}

type stackReleaseConfig struct {
	url string
}

type StackReleaseOption func(*stackReleaseConfig)

func WithURL(url string) StackReleaseOption {
	return func(cfg *stackReleaseConfig) {
		cfg.url = url
	}
}

func GetStackRelease(options ...StackReleaseOption) ([]StackRelease, error) {
	var cfg stackReleaseConfig
	for _, option := range options {
		option(&cfg)
	}
	if cfg.url == "" {
		cfg.url = defaultStackReleaseURL
	}

	type response struct {
		Releases []pastRelease `json:"releases"`
	}
	var res response
	if err := fetchJSONFile(cfg.url, &res); err != nil {
		return nil, err
	}
	if len(res.Releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}
	sort.Slice(res.Releases, func(i, j int) bool {
		return res.Releases[i].Version.GreaterThan(res.Releases[j].Version)
	})

	toName := func(v *semver.Version) string {
		return fmt.Sprintf("%d.%d", v.Major(), v.Minor())
	}
	latest := res.Releases[0].Version

	pastReleaseMap := map[string]*pastRelease{}
	for i := range res.Releases {
		pastReleaseMap[res.Releases[i].Version.String()] = &res.Releases[i]
	}

	namedReleases := map[string]StackRelease{}
	for _, release := range res.Releases {
		name := toName(release.Version)
		if _, ok := namedReleases[name]; ok {
			continue
		}

		originalVer := semver.MustParse(name)
		originalRel := pastReleaseMap[originalVer.String()]

		namedReleases[name] = StackRelease{
			Name:       name,
			First:      originalVer,
			Latest:     release.Version,
			FirstDate:  originalRel.Date.Time,
			LatestDate: release.Date.Time,
		}
	}

	var releases []StackRelease
	// Get latest release + last 2 minor releases.
	latestMinor := int(latest.Minor())
	for i := latestMinor; i >= max(latestMinor-2, 0); i-- {
		name := fmt.Sprintf("%d.%d", latest.Major(), i)

		releases = append(releases, namedReleases[name])
	}

	// Get latest last major release.
	lastMajor := latest.Major() - 1
	for _, release := range res.Releases {
		if release.Version.Major() != lastMajor {
			continue
		}

		name := toName(release.Version)
		releases = append(releases, namedReleases[name])

		break
	}

	return releases, nil
}

func fetchJSONFile(url string, v any) error {
	slog.Debug("Fetching remote file", slog.String("url", url))

	var res *http.Response
	var err error
	if res, err = http.Get(url); err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch remote file (%q): status %s", url, res.Status)
	}

	dec := json.NewDecoder(res.Body)

	return dec.Decode(v)
}
