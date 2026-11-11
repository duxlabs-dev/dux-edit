// Package plugins reads plugin.json manifests. Loading and enabling happen in the UI.
package plugins

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Manifest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Icon        string   `json:"icon"`
	Dir         string   `json:"dir"`
	Editor      string   `json:"editor"`
	Settings    string   `json:"settings"`
	Permissions []string `json:"permissions"`
	// Error says why a manifest cannot be loaded; such entries are still listed.
	Error string `json:"error,omitempty"`
}

type rawManifest struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Author      string            `json:"author"`
	Icon        string            `json:"icon"`
	Type        string            `json:"type"`
	Component   string            `json:"component"`
	Components  map[string]string `json:"components"`
	Settings    string            `json:"settings"`
	Permissions []string          `json:"permissions"`
}

// The id names the plugin's state file, so it must not carry path characters.
var idPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*$`)

const builtIn = "dankNotepadModule"

// Scan reads <dir>/*/plugin.json in each dir. The first dir that has an id wins.
func Scan(dirs []string) []Manifest {
	var out []Manifest
	seen := map[string]bool{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			pluginDir := filepath.Join(dir, entry.Name())
			m, ok := read(pluginDir)
			if !ok || seen[m.ID] {
				continue
			}
			seen[m.ID] = true
			out = append(out, m)
		}
	}
	return out
}

func read(pluginDir string) (Manifest, bool) {
	data, err := os.ReadFile(filepath.Join(pluginDir, "plugin.json"))
	if err != nil {
		return Manifest{}, false
	}
	m := Manifest{ID: filepath.Base(pluginDir), Dir: pluginDir, Permissions: []string{}}
	var raw rawManifest
	if err := json.Unmarshal(data, &raw); err != nil {
		m.Error = "invalid plugin.json: " + err.Error()
		return m, true
	}
	if raw.ID != "" {
		m.ID = raw.ID
	}
	m.Name, m.Description, m.Version, m.Author, m.Icon = raw.Name, raw.Description, raw.Version, raw.Author, raw.Icon
	if m.Name == "" {
		m.Name = m.ID
	}
	if raw.Permissions != nil {
		m.Permissions = raw.Permissions
	}
	m.Error = validate(&m, raw)
	return m, true
}

func validate(m *Manifest, raw rawManifest) string {
	switch {
	case raw.ID == builtIn:
		return "built in"
	case !idPattern.MatchString(raw.ID):
		m.ID = filepath.Base(m.Dir)
		return "invalid id"
	}
	editor := raw.Components["editor"]
	if editor == "" && raw.Type == "editor" {
		editor = raw.Component
	}
	if editor == "" {
		return "no editor surface"
	}
	var err error
	if m.Editor, err = inside(editor); err != nil {
		return err.Error()
	}
	if raw.Settings == "" {
		return ""
	}
	if m.Settings, err = inside(raw.Settings); err != nil {
		return err.Error()
	}
	return ""
}

var errEscapes = errors.New("component path leaves the plugin directory")

// inside returns path relative to the plugin directory, refusing absolute paths and "..".
func inside(path string) (string, error) {
	clean := filepath.Clean(strings.TrimPrefix(path, "./"))
	if !filepath.IsLocal(clean) {
		return "", errEscapes
	}
	return clean, nil
}
