package spell

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/duxlabs-dev/dux-edit/core/internal/atomicfile"
)

// Serializes the read and the write of appendWord, so two adds cannot both start from the same list.
var appendMu sync.Mutex

// personalFiles are the user's word lists for lang: ours first, then enchant's and hunspell's, read only.
func personalFiles(custom, lang string, getenv func(string) string, home string) []string {
	config := getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	return []string{
		custom,
		filepath.Join(config, "enchant", lang+".dic"),
		filepath.Join(home, ".hunspell_"+lang),
	}
}

func readWords(path string, into map[string]bool) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// hunspell personal dictionaries mark forbidden words with "*" and affix flags after "/".
		if line == "" || strings.HasPrefix(line, "*") {
			continue
		}
		w, _, _ := strings.Cut(line, "/")
		into[normalize(w)] = true
	}
}

// appendWord adds w to the word list at path unless it is there already; a symlinked list stays a symlink.
func appendWord(path, w string) error {
	appendMu.Lock()
	defer appendMu.Unlock()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == w {
			return nil
		}
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	data = append(data, w+"\n"...)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_, err = atomicfile.Save(path, data, atomicfile.Options{Mode: 0o644})
	return err
}
