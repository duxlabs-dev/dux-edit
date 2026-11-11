// Package dmsimport copies DMS notepad tabs and settings. It never changes anything under the DMS dirs.
package dmsimport

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	duxpaths "github.com/duxlabs-dev/dux-go/paths"

	"github.com/duxlabs-dev/dux-edit/core/internal/swap"
)

type Tab struct {
	Title string `json:"title"`
	// Path is set for tabs on real files; scratch notes are copied to a swap draft and carry SwapID.
	Path   string `json:"path,omitempty"`
	SwapID string `json:"swapId,omitempty"`
}

type Result struct {
	OK       bool           `json:"ok"`
	Found    bool           `json:"found"`
	Tabs     []Tab          `json:"tabs"`
	Current  int            `json:"current"`
	Settings map[string]any `json:"settings"`
}

type session struct {
	CurrentTabIndex int `json:"currentTabIndex"`
	Tabs            []struct {
		Title       string `json:"title"`
		FilePath    string `json:"filePath"`
		IsTemporary bool   `json:"isTemporary"`
	} `json:"tabs"`
}

var settingKeys = map[string]string{
	"notepadUseMonospace":    "useMonospace",
	"notepadFontFamily":      "fontFamily",
	"notepadFontSize":        "fontSize",
	"notepadShowLineNumbers": "showLineNumbers",
	"notepadAutoSave":        "autoSave",
}

// Dirs are DMS's state and config directories.
func Dirs() (state, config string) {
	return filepath.Join(duxpaths.XDGStateHome(), "dms"), filepath.Join(duxpaths.XDGConfigHome(), "dms")
}

// Run reads <dmsStateDir>/notepad-session.json and notepad-files/, writing drafts into swapDir unless dryRun.
func Run(dmsStateDir, dmsConfigDir, swapDir string, dryRun bool) (Result, error) {
	result := Result{OK: true, Tabs: []Tab{}, Settings: map[string]any{}}
	raw, err := os.ReadFile(filepath.Join(dmsStateDir, "notepad-session.json"))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return result, nil
	case err != nil:
		return result, err
	}
	var s session
	if err := json.Unmarshal(raw, &s); err != nil {
		return result, err
	}
	result.Found = true
	result.Settings = readSettings(dmsConfigDir)

	for i, entry := range s.Tabs {
		tab := Tab{Title: entry.Title}
		switch {
		case !entry.IsTemporary:
			if !filepath.IsAbs(entry.FilePath) {
				continue
			}
			tab.Path = entry.FilePath
		default:
			text, ok := scratchText(dmsStateDir, entry.FilePath)
			if !ok {
				continue
			}
			if !dryRun {
				tab.SwapID = swap.NewID()
				side := swap.Sidecar{Title: entry.Title, Encoding: "utf-8", EOL: "lf", SavedAt: time.Now().UnixMilli()}
				if err := swap.Dir(swapDir).Write(tab.SwapID, text, side); err != nil {
					return result, err
				}
			}
		}
		result.Tabs = append(result.Tabs, tab)
		// A skipped current tab hands focus to the tab before it.
		if i <= s.CurrentTabIndex {
			result.Current = len(result.Tabs) - 1
		}
	}
	result.Current = max(result.Current, 0)
	return result, nil
}

// Scratch notes live under the state dir; a path that leaves it is not one of them.
func scratchText(stateDir, rel string) (string, bool) {
	if !filepath.IsLocal(rel) {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(stateDir, rel))
	if err != nil {
		return "", false
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ToValidUTF8(strings.ReplaceAll(text, "\r", "\n"), "�")
	if strings.TrimSpace(text) == "" {
		return "", false
	}
	return text, true
}

func readSettings(configDir string) map[string]any {
	settings := map[string]any{}
	raw, err := os.ReadFile(filepath.Join(configDir, "settings.json"))
	if err != nil {
		return settings
	}
	var dms map[string]any
	if json.Unmarshal(raw, &dms) != nil {
		return settings
	}
	for from, to := range settingKeys {
		if value, ok := dms[from]; ok {
			settings[to] = value
		}
	}
	return settings
}
