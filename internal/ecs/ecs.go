// Licensed to Elasticsearch B.V. under one or more agreements.
// Elasticsearch B.V. licenses this file to you under the Apache 2.0 License.
// See the LICENSE file in the project root for more information.

package ecs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/elastic/rule-validator/internal/field"
)

const defaultURL = "https://raw.githubusercontent.com"

type resolverFile struct {
	Version string                  `json:"version"`
	Ref     string                  `json:"ref"`
	Fields  map[string]*field.Field `json:"fields"`
}

type fieldDef struct {
	Type        field.Type `yaml:"type"`
	Description string     `yaml:"description"`
	Normalize   any        `yaml:"normalize"`
}

type config struct {
	url      string
	cacheDir string
}

type Option func(*config)

func WithCacheDir(dir string) Option {
	return func(c *config) {
		c.cacheDir = dir
	}
}

func WithBaseURL(value string) Option {
	u, err := url.Parse(value)
	if err != nil {
		panic(err)
	}

	return func(c *config) {
		c.url = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	}
}

type Resolver struct {
	version string
	ref     string
	fields  map[string]*field.Field
}

func (r *Resolver) UnmarshalJSON(bytes []byte) error {
	var rf resolverFile
	if err := json.Unmarshal(bytes, &rf); err != nil {
		return err
	}

	r.version = rf.Version
	r.ref = rf.Ref
	r.fields = rf.Fields

	return nil
}

func (r *Resolver) MarshalJSON() ([]byte, error) {
	return json.Marshal(&resolverFile{
		Version: r.version,
		Ref:     r.ref,
		Fields:  r.fields,
	})
}

func (r *Resolver) Lookup(name string) *field.Field {
	return r.fields[name]
}

func (r *Resolver) Version() string {
	return r.version
}

func (r *Resolver) Ref() string {
	return r.ref
}

func NewResolver(ref string, options ...Option) (*Resolver, error) {
	var cfg config
	for _, opt := range options {
		opt(&cfg)
	}
	if cfg.url == "" {
		cfg.url = defaultURL
	}

	if cfg.cacheDir != "" {
		if r, err := loadResolver(ref, cfg.cacheDir); err == nil {
			slog.Debug("Loaded ECS resolver from cache", slog.String("ref", r.ref), slog.String("version", r.version))
			return r, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("Failed to load resolver, fetching from web...", slog.String("error", err.Error()))
		}
	}

	r, err := buildResolver(ref, cfg.url)
	if err != nil {
		return nil, err
	}

	if cfg.cacheDir != "" {
		if err = saveResolver(ref, cfg.cacheDir, r); err != nil {
			slog.Warn("Failed to save resolver", slog.String("error", err.Error()))
		} else {
			slog.Debug("Saved ECS resolver to cache", slog.String("ref", r.ref), slog.String("version", r.version))
		}
	}

	return r, nil
}

func getCacheFilename(ref, cacheDir string) string {
	return filepath.Join(cacheDir, ref+".json")
}

func loadResolver(ref, cacheDir string) (*Resolver, error) {
	cacheFilename := getCacheFilename(ref, cacheDir)

	raw, err := os.ReadFile(cacheFilename)
	if err != nil {
		return nil, err
	}

	var r Resolver
	if err = json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}

	return &r, nil
}

func saveResolver(ref, cacheDir string, r *Resolver) error {
	cacheFilename := getCacheFilename(ref, cacheDir)

	if err := os.MkdirAll(filepath.Dir(cacheFilename), 0o755); err != nil {
		return err
	}

	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err = os.WriteFile(cacheFilename, data, 0o644); err != nil {
		return err
	}

	return nil
}

func buildResolver(ref, baseURL string) (*Resolver, error) {
	ref = strings.TrimPrefix(ref, "git@")

	versionRaw, err := fetchFile(baseURL + "/elastic/ecs/" + ref + "/version")
	if err != nil {
		return nil, err
	}

	schemaRaw, err := fetchFile(baseURL + "/elastic/ecs/" + ref + "/generated/ecs/ecs_flat.yml")
	if err != nil {
		return nil, err
	}
	var fieldDefs map[string]fieldDef
	if err = yaml.Unmarshal(schemaRaw, &fieldDefs); err != nil {
		return nil, err
	}

	r := Resolver{
		ref:     ref,
		version: string(bytes.TrimSpace(versionRaw)),
		fields:  make(map[string]*field.Field),
	}

	for k, v := range fieldDefs {
		r.fields[k] = &field.Field{
			Name: k,
			Type: v.Type,
		}

		// Add lat, long, type, and coordinates sub-fields for geo_point types.
		if v.Type == field.TypeGeoPoint {
			name := k + ".lat"
			r.fields[name] = &field.Field{
				Name: name,
				Type: field.TypeGeoPoint,
			}
			name = k + ".lon"
			r.fields[name] = &field.Field{
				Name: name,
				Type: field.TypeGeoPoint,
			}
			name = k + ".type"
			r.fields[name] = &field.Field{
				Name: name,
				Type: field.TypeGeoPoint,
			}
			name = k + ".coordinates"
			r.fields[name] = &field.Field{
				Name: name,
				Type: field.TypeGeoPoint,
			}
		}
	}

	return &r, nil
}

func fetchFile(url string) ([]byte, error) {
	slog.Debug("Fetching remote file", slog.String("url", url))

	var res *http.Response
	var err error
	if res, err = http.Get(url); err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch remote file (%q): status %s", url, res.Status)
	}

	return io.ReadAll(res.Body)
}
