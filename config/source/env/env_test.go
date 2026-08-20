package env

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/go-admin-team/go-admin-core/v2/config/source"
)

func setEnvVars(vars map[string]string) {
	for k, v := range vars {
		os.Setenv(k, v)
	}
}

func unsetEnvVars(vars map[string]string) {
	for k := range vars {
		os.Unsetenv(k)
	}
}

func unmarshalData(data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := json.Unmarshal(data, &result)
	return result, err
}

func TestEnv_Read(t *testing.T) {
	expected := map[string]map[string]string{
		"database": {
			"host":       "localhost",
			"password":   "password",
			"datasource": "user:password@tcp(localhost:port)/db?charset=utf8mb4&parseTime=True&loc=Local",
		},
	}

	envVars := map[string]string{
		"DATABASE_HOST":       "localhost",
		"DATABASE_PASSWORD":   "password",
		"DATABASE_DATASOURCE": "user:password@tcp(localhost:port)/db?charset=utf8mb4&parseTime=True&loc=Local",
	}

	setEnvVars(envVars)
	defer unsetEnvVars(envVars)

	newSource := NewSource()
	c, err := newSource.Read()
	if err != nil {
		t.Error(err)
	}

	actual, err := unmarshalData(c.Data)
	if err != nil {
		t.Error(err)
	}

	actualDB := actual["database"].(map[string]interface{})

	for k, v := range expected["database"] {
		a := actualDB[k]

		if a != v {
			t.Errorf("expected %v got %v", v, a)
		}
	}
}

func TestEnvvar_Prefixes(t *testing.T) {
	envVars := map[string]string{
		"APP_DATABASE_HOST":      "localhost",
		"APP_DATABASE_PASSWORD":  "password",
		"VAULT_ADDR":             "vault:1337",
		"GO_ADMIN_CORE_REGISTRY": "mdns",
	}

	setEnvVars(envVars)
	defer unsetEnvVars(envVars)

	var prefixtests = []struct {
		prefixOpts   []source.Option
		expectedKeys []string
	}{
		{[]source.Option{WithPrefix("APP", "GO_ADMIN_CORE")}, []string{"app", "go"}},
		{[]source.Option{WithPrefix("GO_ADMIN_CORE"), WithStrippedPrefix("APP")}, []string{"database", "go"}},
		{[]source.Option{WithPrefix("GO_ADMIN_CORE"), WithStrippedPrefix("APP")}, []string{"database", "go"}},
	}

	for _, pt := range prefixtests {
		newSource := NewSource(pt.prefixOpts...)

		c, err := newSource.Read()
		if err != nil {
			t.Error(err)
		}

		actual, err := unmarshalData(c.Data)
		if err != nil {
			t.Error(err)
		}

		// assert other prefixes ignored
		if l := len(actual); l != len(pt.expectedKeys) {
			t.Errorf("expected %v top keys, got %v", len(pt.expectedKeys), l)
		}

		for _, k := range pt.expectedKeys {
			if !containsKey(actual, k) {
				t.Errorf("expected key %v, not found", k)
			}
		}
	}
}

func TestEnvvar_WatchNextNoOpsUntilStop(t *testing.T) {
	src := NewSource(WithStrippedPrefix("GO_ADMIN_CORE_"))
	w, err := src.Watch()
	if err != nil {
		t.Error(err)
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		w.Stop()
	}()

	if _, err := w.Next(); err != source.ErrWatcherStopped {
		t.Errorf("expected watcher stopped error, got %v", err)
	}
}

func containsKey(m map[string]interface{}, s string) bool {
	for k := range m {
		if k == s {
			return true
		}
	}
	return false
}
