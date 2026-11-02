package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/image/v5/types"
	"go.yaml.in/yaml/v4"
)

var _ yaml.Unmarshaler = (*tlsVerifyConfig)(nil)

func TestTLSVerifyConfig(t *testing.T) {
	type container struct { // An example of a larger config file
		TLSVerify tlsVerifyConfig `yaml:"tls-verify"`
	}

	for _, c := range []struct {
		input    string
		expected tlsVerifyConfig
	}{
		{
			input:    `tls-verify: true`,
			expected: tlsVerifyConfig{skip: types.OptionalBoolFalse},
		},
		{
			input:    `tls-verify: false`,
			expected: tlsVerifyConfig{skip: types.OptionalBoolTrue},
		},
		{
			input:    `not-a-tls-verify-field: ""`,
			expected: tlsVerifyConfig{skip: types.OptionalBoolUndefined},
		},
	} {
		config := container{}
		err := yaml.Load([]byte(c.input), &config)
		require.NoError(t, err, c.input)
		assert.Equal(t, c.expected, config.TLSVerify, c.input)
	}

	// Invalid input
	config := container{}
	err := yaml.Load([]byte(`tls-verify: "not a valid bool"`), &config)
	assert.Error(t, err)
}

func TestSync(t *testing.T) {
	// Invalid command-line arguments
	for _, args := range [][]string{
		{},
		{"a1"},
		{"a1", "a2", "a3"},
	} {
		out, err := runSkopeo(append([]string{"sync"}, args...)...)
		assertTestFailed(t, out, err, "exactly two arguments expected")
	}

	// FIXME: Much more test coverage
	// Actual feature tests exist in integration and systemtest
}

func TestSyncYAMLSelection(t *testing.T) {
	for _, tt := range []struct {
		name string
		cfg  registrySyncConfig
		err  string
	}{
		{"invalid repository", registrySyncConfig{Images: map[string][]string{"invalid name": {"latest"}}}, "parsing repository name"},
		{"tagged repository", registrySyncConfig{Images: map[string][]string{"repo:latest": {"latest"}}}, "not a repository"},
		{"invalid reference", registrySyncConfig{Images: map[string][]string{"repo": {"v{REDIS_VERSION}"}}}, "reference"},
		{"mixed references", registrySyncConfig{Images: map[string][]string{"repo": {"latest", "v{REDIS_VERSION}"}}}, "reference"},
		{"invalid regex", registrySyncConfig{ImagesByTagRegex: map[string]string{"repo": "["}}, "parsing tag filters"},
		{"invalid semver", registrySyncConfig{ImagesBySemver: map[string]string{"repo": "invalid constraint"}}, "parsing semver filters"},
		{"invalid regex repository", registrySyncConfig{ImagesByTagRegex: map[string]string{"invalid name": ".*"}}, "parsing repository name"},
		{"invalid semver repository", registrySyncConfig{ImagesBySemver: map[string]string{"invalid name": ">=1"}}, "parsing repository name"},
		{"valid references", registrySyncConfig{Images: map[string][]string{"repo": {"latest", "v1.0.0"}}}, ""},
		{"empty registry", registrySyncConfig{}, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config, err := yaml.Marshal(map[string]any{"example.com": map[string]any{
				"images": tt.cfg.Images, "images-by-tag-regex": tt.cfg.ImagesByTagRegex,
				"images-by-semver": tt.cfg.ImagesBySemver,
			}})
			require.NoError(t, err)
			file := filepath.Join(t.TempDir(), "sync.yaml")
			require.NoError(t, os.WriteFile(file, config, 0o600))
			for _, keepGoing := range []bool{false, true} {
				out, err := runSkopeo("--insecure-policy", "sync", "--src", "yaml", "--dest", "dir", "--dry-run",
					fmt.Sprintf("--keep-going=%t", keepGoing), file, t.TempDir())
				if tt.err == "" {
					require.NoError(t, err, out)
				} else {
					require.ErrorContains(t, err, tt.err, out)
				}
			}
		})
	}
}

func TestSyncRegistryTagSelection(t *testing.T) {
	for _, tag := range []string{"invalid tag", "v1.0.0", "not-semver"} {
		t.Run(tag, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/v2/":
					fmt.Fprint(w, `{}`)
				case "/v2/repo/tags/list":
					fmt.Fprintf(w, `{"name":"repo","tags":[%q]}`, tag)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			ctx := types.SystemContext{DockerInsecureSkipTLSVerify: types.OptionalBoolTrue}
			for _, cfg := range []registrySyncConfig{
				{Images: map[string][]string{"repo": nil}},
				{ImagesByTagRegex: map[string]string{"repo": "^v"}},
				{ImagesBySemver: map[string]string{"repo": ">=1"}},
			} {
				descs, err := imagesToCopyFromRegistry(strings.TrimPrefix(server.URL, "http://"), cfg, ctx)
				if tag == "invalid tag" {
					require.ErrorContains(t, err, "invalid tag")
				} else {
					require.NoError(t, err)
					if tag == "v1.0.0" || len(cfg.Images) != 0 {
						require.Len(t, descs, 1)
						require.Len(t, descs[0].ImageRefs, 1)
					} else {
						assert.Empty(t, descs)
					}
				}
			}
		})
	}
}

// TestSyncTLSPrecedence validates the interactions of tls-verify in YAML and --src-tls-verify in the CLI.
func TestSyncTLSPrecedence(t *testing.T) {
	for _, tt := range []struct {
		cli            string
		yaml           string
		wantSkip       types.OptionalBool
		wantDaemonSkip bool
	}{
		{"--src-tls-verify=false", `# nothing`, types.OptionalBoolTrue, true},
		{"--src-tls-verify=true", `# nothing`, types.OptionalBoolFalse, false},
		{"", `# nothing`, types.OptionalBoolUndefined, false},
		{"--src-tls-verify=false", "tls-verify: true", types.OptionalBoolFalse, false},
		{"--src-tls-verify=true", "tls-verify: false", types.OptionalBoolTrue, true},
	} {
		t.Run(fmt.Sprintf("%#v + %q", tt.cli, tt.yaml), func(t *testing.T) {
			opts := fakeImageOptions(t, "src-", true, []string{}, []string{tt.cli})
			sourceCtx, err := opts.newSystemContext()
			require.NoError(t, err)
			var cfg registrySyncConfig
			err = yaml.Load(fmt.Appendf(nil, `
%s
images:
  repo: # Specifying an explicit repo+tag avoids imagesToCopyFromRegistry trying to contact the registry.
    - latest
`, tt.yaml,
			), &cfg)
			require.NoError(t, err)

			descs, err := imagesToCopyFromRegistry("example.com", cfg, *sourceCtx)
			require.NoError(t, err)
			require.NotEmpty(t, descs)
			ctx := descs[0].Context
			require.NotNil(t, ctx)
			assert.Equal(t, tt.wantSkip, ctx.DockerInsecureSkipTLSVerify)
			assert.Equal(t, tt.wantDaemonSkip, ctx.DockerDaemonInsecureSkipTLSVerify)
		})
	}
}
