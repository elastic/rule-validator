// Licensed to Elasticsearch B.V. under one or more agreements.
// Elasticsearch B.V. licenses this file to you under the Apache 2.0 License.
// See the LICENSE file in the project root for more information.

package cache

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/rule-validator/internal/git"
)

const (
	ecsDir   = "ecs"
	reposDir = "repos"
)

var repoNameReplacer = strings.NewReplacer(".", "_", "/", "_")

type Cache struct {
	dir   string
	repos map[string]*git.Repo
}

func (c *Cache) Dir() string {
	return c.dir
}

func (c *Cache) ECSDir() string {
	return filepath.Join(c.dir, ecsDir)
}

func (c *Cache) ReposDir() string {
	return filepath.Join(c.dir, reposDir)
}

func (c *Cache) GetOrCreateRepo(ctx context.Context, remote string) (*git.Repo, error) {
	u, err := url.Parse(remote)
	if err != nil {
		return nil, err
	}

	name := repoNameReplacer.Replace(u.Host)
	name += repoNameReplacer.Replace(strings.TrimSuffix(u.Path, ".git"))

	if repo, ok := c.repos[name]; ok {
		return repo, nil
	}

	repo, err := git.NewRepo(ctx, name, filepath.Join(c.ReposDir(), name), remote)
	if err != nil {
		return nil, err
	}

	c.repos[name] = repo

	return repo, nil
}

func New(dir string) (*Cache, error) {
	if stat, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("cache: unable to create directory: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("cache: %q stat error: %w", dir, err)
	} else if !stat.IsDir() {
		return nil, fmt.Errorf("cache: %q is not a directory", dir)
	}

	if err := os.MkdirAll(filepath.Join(dir, ecsDir), 0o755); err != nil {
		return nil, fmt.Errorf("cache: unable to create directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, reposDir), 0o755); err != nil {
		return nil, fmt.Errorf("cache: unable to create directory: %w", err)
	}

	return &Cache{
		dir:   dir,
		repos: map[string]*git.Repo{},
	}, nil
}
