package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var envLoadOnce sync.Once

func ensureEnvLoaded() {
	envLoadOnce.Do(func() {
		tryLoadDotEnv(filepath.Join(getWorkingDirectory(), ".env"))
		tryLoadDotEnv(".env")
	})
}

func tryLoadDotEnv(path string) {
	if path == "" {
		return
	}
	_ = loadDotEnv(path)
}

func getWorkingDirectory() string {
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return ""
}

func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if len(val) >= 2 {
			if (val[0] == '\'' && val[len(val)-1] == '\'') || (val[0] == '"' && val[len(val)-1] == '"') {
				val = val[1 : len(val)-1]
			}
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
	return s.Err()
}
