package settings

import (
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSettings(t *testing.T) *Settings {
	t.Helper()
	s := New("test")
	s.SetOutput(io.Discard)
	s.String("app-env", "", "production", "environment")
	s.Int("port", "p", 6001, "port")
	s.Int64("max-connections", "", 0, "max connections")
	s.Bool("redis-tls", "", false, "use TLS")
	s.Bool("version", "v", false, "print version")
	s.String("adapter-driver", "a", "local", "adapter")
	return s
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestParse_FlagForms(t *testing.T) {
	s := newTestSettings(t)
	require.NoError(t, s.Parse([]string{"--app-env=dev", "-p", "7000", "--redis-tls", "-a=redis", "--max-connections", "50"}))

	assert.Equal(t, "dev", s.GetString("app_env"))
	assert.Equal(t, 7000, s.GetInt("port"))
	assert.True(t, s.GetBool("redis-tls"))
	assert.Equal(t, "redis", s.GetString("adapter_driver"))
	assert.Equal(t, int64(50), s.GetInt64("max_connections"))
	assert.True(t, s.Changed("port"))
	assert.True(t, s.Changed("adapter-driver"), "shorthand marks canonical flag changed")
	assert.False(t, s.Changed("version"))
}

func TestParse_HelpAndUnknownFlag(t *testing.T) {
	s := newTestSettings(t)
	assert.True(t, errors.Is(s.Parse([]string{"--help"}), flag.ErrHelp))

	s = newTestSettings(t)
	assert.Error(t, s.Parse([]string{"--nope"}))
}

func TestDuplicateFlagPanics(t *testing.T) {
	s := New("test")
	s.String("app-env", "", "", "")
	assert.Panics(t, func() { s.String("app_env", "", "", "") })
}

func TestPrecedence(t *testing.T) {
	cfg := writeConfig(t, `{"app-env": "from-file", "port": 8000, "redis_tls": true}`)

	t.Run("flag default when nothing else set", func(t *testing.T) {
		s := newTestSettings(t)
		require.NoError(t, s.Parse(nil))
		assert.Equal(t, "production", s.GetString("app_env"))
		assert.Equal(t, 6001, s.GetInt("port"))
	})

	t.Run("config file beats flag default", func(t *testing.T) {
		s := newTestSettings(t)
		require.NoError(t, s.Parse(nil))
		require.NoError(t, s.ReadConfigFile(cfg))
		assert.Equal(t, "from-file", s.GetString("app_env"))
		assert.Equal(t, 8000, s.GetInt("port"))
		assert.True(t, s.GetBool("redis_tls"))
	})

	t.Run("env beats config file", func(t *testing.T) {
		t.Setenv("APP_ENV", "from-env")
		t.Setenv("PORT", "9000")
		s := newTestSettings(t)
		require.NoError(t, s.Parse(nil))
		require.NoError(t, s.ReadConfigFile(cfg))
		assert.Equal(t, "from-env", s.GetString("app-env"))
		assert.Equal(t, 9000, s.GetInt("port"))
	})

	t.Run("empty env var is ignored", func(t *testing.T) {
		t.Setenv("APP_ENV", "")
		s := newTestSettings(t)
		require.NoError(t, s.Parse(nil))
		assert.Equal(t, "production", s.GetString("app_env"))
	})

	t.Run("explicit flag beats env", func(t *testing.T) {
		t.Setenv("APP_ENV", "from-env")
		s := newTestSettings(t)
		require.NoError(t, s.Parse([]string{"--app-env", "from-flag"}))
		require.NoError(t, s.ReadConfigFile(cfg))
		assert.Equal(t, "from-flag", s.GetString("app_env"))
	})

	t.Run("env-only key is visible to Get", func(t *testing.T) {
		t.Setenv("CONFIG_FILE", "/tmp/x.json")
		s := newTestSettings(t)
		assert.Equal(t, "/tmp/x.json", s.GetString("config-file"))
		_, ok := s.Get("not_set_anywhere")
		assert.False(t, ok)
	})
}

func TestGetters_InvalidValueReturnsZero(t *testing.T) {
	t.Setenv("PORT", "abc")
	s := newTestSettings(t)
	assert.Equal(t, 0, s.GetInt("port"))
}

func TestReadConfigFile_Errors(t *testing.T) {
	s := newTestSettings(t)
	assert.Error(t, s.ReadConfigFile(filepath.Join(t.TempDir(), "missing.json")))
	assert.Error(t, s.ReadConfigFile(writeConfig(t, `{not json`)))
	assert.Error(t, s.ReadConfigFile(writeConfig(t, `[1,2]`)))
}

type testApp struct {
	ID             string `config:"app_id"`
	MaxConnections int64  `config:"app_max_connections"`
	Enabled        bool   `config:"app_enabled"`
	ActivityTimout int    `config:"app_activity_timeout"`
}

type testConfig struct {
	hidden         string
	Skipped        string    `config:"-"`
	Env            string    `config:"app_env"`
	Port           string    `config:"port"` // int flag decoded into a string field
	RedisTls       bool      `config:"redis_tls"`
	MaxConnections int64     `config:"max_connections"`
	Adapter        *string   `config:"adapter_driver"`
	Applications   []testApp `config:"applications"`
	Untagged       string
}

func TestUnmarshal(t *testing.T) {
	cfg := writeConfig(t, `{
		"Skipped": "should not land",
		"untagged": "matched by field name",
		"applications": [
			{"APP_ID": "1", "app_max_connections": 1000, "app_enabled": true, "app_activity_timeout": "60"},
			{"app_id": 2, "app_enabled": "false"}
		]
	}`)
	t.Setenv("REDIS_TLS", "true")

	s := newTestSettings(t)
	require.NoError(t, s.Parse([]string{"-p", "7000"}))
	require.NoError(t, s.ReadConfigFile(cfg))

	var got testConfig
	require.NoError(t, s.Unmarshal(&got))

	assert.Equal(t, "production", got.Env)
	assert.Equal(t, "7000", got.Port)
	assert.True(t, got.RedisTls)
	assert.Equal(t, int64(0), got.MaxConnections)
	require.NotNil(t, got.Adapter)
	assert.Equal(t, "local", *got.Adapter)
	assert.Empty(t, got.Skipped)
	assert.Equal(t, "matched by field name", got.Untagged)
	assert.Equal(t, []testApp{
		{ID: "1", MaxConnections: 1000, Enabled: true, ActivityTimout: 60},
		{ID: "2", Enabled: false},
	}, got.Applications)
}

func TestUnmarshal_Errors(t *testing.T) {
	tests := map[string]struct {
		env  map[string]string
		file string
		want string
	}{
		"bad int from env":       {env: map[string]string{"MAX_CONNECTIONS": "lots"}, want: "max_connections"},
		"bad bool from env":      {env: map[string]string{"REDIS_TLS": "maybe"}, want: "redis_tls"},
		"fractional number":      {file: `{"max_connections": 1.5}`, want: "max_connections"},
		"applications not array": {file: `{"applications": {"app_id": "1"}}`, want: "applications"},
		"nested bad value":       {file: `{"applications": [{"app_max_connections": "x"}]}`, want: "applications[0].app_max_connections"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			s := newTestSettings(t)
			require.NoError(t, s.Parse(nil))
			if tc.file != "" {
				require.NoError(t, s.ReadConfigFile(writeConfig(t, tc.file)))
			}
			var got testConfig
			err := s.Unmarshal(&got)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestUnmarshal_RequiresPointer(t *testing.T) {
	s := newTestSettings(t)
	assert.Error(t, s.Unmarshal(testConfig{}))
	assert.Error(t, s.Unmarshal(nil))
}
