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

	"github.com/elastic/rule-validator/internal/cache"
	"github.com/elastic/rule-validator/internal/field"
)

var ErrFieldNotFound = errors.New("field not found")

const defaultURL = "https://raw.githubusercontent.com"

type schema struct {
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
	url   string
	cache *cache.Cache
}

type Option func(*config)

func WithCache(c *cache.Cache) Option {
	return func(cfg *config) {
		cfg.cache = c
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
	cache   *cache.Cache
	baseURL string
	schemas map[string]*schema
}

func (r *Resolver) Lookup(ref, name string) (*field.Field, error) {
	var err error
	s, ok := r.schemas[ref]
	if !ok {
		if s, err = r.getSchema(ref); err != nil {
			return nil, fmt.Errorf("failed to build schema using ref %q: %w", ref, err)
		}
		r.schemas[ref] = s
	}

	f, ok := s.Fields[name]
	if !ok {
		return nil, ErrFieldNotFound
	}

	return f, nil
}

func (r *Resolver) loadSchema(ref string) (*schema, error) {
	cacheFilename := getCacheFilename(ref, r.cache.ECSDir())

	raw, err := os.ReadFile(cacheFilename)
	if err != nil {
		return nil, err
	}

	var s schema
	if err = json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}

	return &s, nil
}

func (r *Resolver) saveSchema(s *schema) error {
	cacheFilename := getCacheFilename(s.Ref, r.cache.ECSDir())

	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err = os.WriteFile(cacheFilename, data, 0o644); err != nil {
		return err
	}

	return nil
}

func (r *Resolver) buildSchema(ref string) (*schema, error) {
	ref = strings.TrimPrefix(ref, "git@")

	versionRaw, err := fetchFile(r.baseURL + "/elastic/ecs/" + ref + "/version")
	if err != nil {
		return nil, err
	}

	schemaRaw, err := fetchFile(r.baseURL + "/elastic/ecs/" + ref + "/generated/ecs/ecs_flat.yml")
	if err != nil {
		return nil, err
	}
	var fieldDefs map[string]fieldDef
	if err = yaml.Unmarshal(schemaRaw, &fieldDefs); err != nil {
		return nil, err
	}

	s := schema{
		Ref:     ref,
		Version: string(bytes.TrimSpace(versionRaw)),
		Fields:  make(map[string]*field.Field),
	}

	for k, v := range fieldDefs {
		s.Fields[k] = &field.Field{
			Name: k,
			Type: v.Type,
		}

		// Add lat, long, type, and coordinates sub-fields for geo_point types.
		if v.Type == field.TypeGeoPoint {
			name := k + ".lat"
			s.Fields[name] = &field.Field{
				Name: name,
				Type: field.TypeGeoPoint,
			}
			name = k + ".lon"
			s.Fields[name] = &field.Field{
				Name: name,
				Type: field.TypeGeoPoint,
			}
			name = k + ".type"
			s.Fields[name] = &field.Field{
				Name: name,
				Type: field.TypeGeoPoint,
			}
			name = k + ".coordinates"
			s.Fields[name] = &field.Field{
				Name: name,
				Type: field.TypeGeoPoint,
			}
		}
	}

	return &s, nil
}

func (r *Resolver) getSchema(ref string) (*schema, error) {
	if r.cache != nil {
		if s, err := r.loadSchema(ref); err == nil {
			slog.Debug("Loaded ECS ECS schema from cache", slog.String("ref", s.Ref), slog.String("version", s.Version))
			return s, nil
		}
	}

	s, err := r.buildSchema(ref)
	if err != nil {
		return nil, err
	}

	if r.cache != nil {
		if err = r.saveSchema(s); err != nil {
			slog.Warn("Failed to save ECS schema", slog.String("error", err.Error()), slog.String("ref", s.Ref), slog.String("version", s.Version))
		} else {
			slog.Debug("Saved ECS resolver to cache", slog.String("ref", s.Ref), slog.String("version", s.Version))
		}
	}

	return s, nil
}

func NewResolver(options ...Option) *Resolver {
	var cfg config
	for _, opt := range options {
		opt(&cfg)
	}
	if cfg.url == "" {
		cfg.url = defaultURL
	}

	return &Resolver{
		cache:   cfg.cache,
		baseURL: cfg.url,
		schemas: map[string]*schema{},
	}
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

func getCacheFilename(ref, cacheDir string) string {
	return filepath.Join(cacheDir, ref+".json")
}
