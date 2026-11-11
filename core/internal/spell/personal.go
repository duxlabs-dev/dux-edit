package spell

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

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

// appendWord adds w to the word list at path unless it is there already, replacing the file atomically.
func appendWord(path, w string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == w {
			return nil
		}
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	data = append(data, w+"\n"...)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".custom_dictionary-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
