// Licensed to Elasticsearch B.V. under one or more agreements.
// Elasticsearch B.V. licenses this file to you under the Apache 2.0 License.
// See the LICENSE file in the project root for more information.

package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"
)

func getEnv() []string {
	return append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",                // Disable HTTPS credential prompts.
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes", // Disable SSH passphrase/password prompts.
		"GIT_CONFIG_COUNT=1",                   // Disable gc to prevent zombie child processes.
		"GIT_CONFIG_KEY_0=gc.auto",
		"GIT_CONFIG_VALUE_0=0",
	)
}

type Repo struct {
	name string
	dir  string
}

// Name is the name of the repository.
func (r *Repo) Name() string {
	return r.name
}

// Dir is the path to the repository.
func (r *Repo) Dir() string {
	return r.dir
}

func (r *Repo) isBareRepo(ctx context.Context) bool {
	out, err := r.RunCommand(ctx, "rev-parse", "--is-bare-repository")
	if err != nil {
		return false
	}

	isBare, err := strconv.ParseBool(strings.TrimSpace(string(out)))
	if err != nil {
		return false
	}

	return isBare
}

// RunCommand runs an arbitrary git command against the repo.
//
//	git -C <dir> args...
func (r *Repo) RunCommand(ctx context.Context, args ...string) ([]byte, error) {
	cmdArgs := []string{"-C", r.dir}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Env = getEnv()

	slog.Debug("Running git command", slog.String("cmd", cmd.String()))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git: command failed [%s]: %w: %s", cmd.String(), err, output)
	}

	return output, nil
}

// FileHistory gets the history of a file in the form of a list of commit hashes.
func (r *Repo) FileHistory(ctx context.Context, ref, filename string) ([]string, error) {
	output, err := r.RunCommand(ctx, "log", "--format=%h", ref, "--", filename)
	if err != nil {
		return nil, err
	}

	var hashes []string

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		hashes = append(hashes, scanner.Text())
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("git: failed to scan output: %w", err)
	}

	return hashes, nil
}

// Tags lists the tags for the repo.
func (r *Repo) Tags(ctx context.Context) ([]string, error) {
	output, err := r.RunCommand(ctx, "tag")
	if err != nil {
		return nil, err
	}

	var tags []string

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		tags = append(tags, scanner.Text())
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("git: failed to scan output: %w", err)
	}

	return tags, nil
}

// TagToHash converts a tag to a commit hash.
func (r *Repo) TagToHash(ctx context.Context, tag string) (string, error) {
	output, err := r.RunCommand(ctx, "rev-parse", tag)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// Changeset gets the list of changes for a given ref.
func (r *Repo) Changeset(ctx context.Context, ref string) ([]string, error) {
	output, err := r.RunCommand(ctx, "show", "--pretty=", "--name-only", ref)
	if err != nil {
		return nil, err
	}

	var files []string

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		files = append(files, scanner.Text())
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("git: failed to scan output: %w", err)
	}

	return files, nil
}

// ListTree will list the directory tree at dirpath at the given ref.
func (r *Repo) ListTree(ctx context.Context, ref, dirpath string, treesOnly, recursive bool) ([]string, error) {
	args := []string{"ls-tree", "--name-only"}
	if treesOnly {
		args = append(args, "-d")
	}
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, ref+":"+dirpath)

	output, err := r.RunCommand(ctx, args...)
	if err != nil && bytes.Contains(output, []byte("valid object name")) {
		return nil, nil
	}
	var items []string

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		items = append(items, scanner.Text())
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("git: failed to scan output: %w", err)
	}

	return items, nil
}

// ReadFile will read and return the contents of a file at the given ref.
func (r *Repo) ReadFile(ctx context.Context, ref, filename string) ([]byte, error) {
	return r.RunCommand(ctx, "cat-file", "-p", ref+":"+filename)
}

// ParseYAMLFile will parse the yaml file at the given ref.
func (r *Repo) ParseYAMLFile(ctx context.Context, ref, filename string, v any) error {
	data, err := r.ReadFile(ctx, ref, filename)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, v)
}

func (r *Repo) clone(ctx context.Context, remote string) error {
	if remote == "" {
		return fmt.Errorf("git: no remote specified")
	}

	parentDir := filepath.Dir(r.dir)

	slog.Info("Cloning repo...", slog.String("repo", r.name), slog.String("dir", r.dir))

	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("git: could not create directory %q: %w", parentDir, err)
	}

	start := time.Now()

	cmd := exec.CommandContext(ctx, "git", "clone",
		"--bare",
		remote,
		r.dir)
	cmd.Env = getEnv()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git: clone failed %w: %s", err, output)
	}

	slog.Debug("Git clone complete", slog.String("repo", r.name), slog.String("duration", time.Since(start).String()))

	return nil
}

func (r *Repo) fetch(ctx context.Context) error {
	slog.Info("Fetching repo...", slog.String("repo", r.name), slog.String("dir", r.dir))

	start := time.Now()

	if _, err := r.RunCommand(ctx, "fetch", "--tags"); err != nil {
		return err
	}

	slog.Debug("Git fetch complete", slog.String("repo", r.name), slog.String("duration", time.Since(start).String()))

	return nil
}

// NewRepo creates a new Repo. It will clone the repo as a bare repository if
// it does not already exist, or it will run fetch on the existing repository.
func NewRepo(ctx context.Context, name, dir, remote string) (*Repo, error) {
	r := Repo{
		name: name,
		dir:  dir,
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = r.clone(ctx, remote); err != nil {
			return nil, err
		}
	} else {
		if !r.isBareRepo(ctx) {
			return nil, fmt.Errorf("git: repository is not a bare repository")
		}

		if err = r.fetch(ctx); err != nil {
			return nil, err
		}
	}

	return &r, nil
}
