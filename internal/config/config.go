package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Config struct {
	path   string
	values map[string]string
}

func New(path string) *Config {
	return &Config{path: path, values: make(map[string]string)}
}

func (c *Config) Load() error {
	file, err := os.Open(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	c.values = make(map[string]string)
	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), "\"")
		if key == "" {
			continue
		}
		if section != "" {
			key = section + "." + key
		}
		c.values[key] = value
	}
	return scanner.Err()
}

func (c *Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	sections := make(map[string]map[string]string)
	for fullKey, value := range c.values {
		section, key, ok := strings.Cut(fullKey, ".")
		if !ok || section == "" || key == "" {
			return fmt.Errorf("config: invalid key %q", fullKey)
		}
		if sections[section] == nil {
			sections[section] = make(map[string]string)
		}
		sections[section][key] = value
	}

	var b strings.Builder
	var sectionNames []string
	for section := range sections {
		sectionNames = append(sectionNames, section)
	}
	sort.Strings(sectionNames)
	for i, section := range sectionNames {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "[%s]\n", section)
		var keys []string
		for key := range sections[section] {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "\t%s = %s\n", key, sections[section][key])
		}
	}
	return os.WriteFile(c.path, []byte(b.String()), 0o644)
}

func (c *Config) Get(key string) string {
	return c.values[key]
}

func (c *Config) Set(key, value string) error {
	if !strings.Contains(key, ".") || strings.HasPrefix(key, ".") || strings.HasSuffix(key, ".") {
		return fmt.Errorf("config: key must use section.name form")
	}
	if strings.ContainsAny(key, "\r\n") || strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("config: keys and values cannot contain newlines")
	}
	c.values[key] = value
	return nil
}

func (c *Config) Unset(key string) error {
	if _, ok := c.values[key]; !ok {
		return fmt.Errorf("config: key %q not found", key)
	}
	delete(c.values, key)
	return nil
}

func (c *Config) All() map[string]string {
	out := make(map[string]string, len(c.values))
	for key, value := range c.values {
		out[key] = value
	}
	return out
}
