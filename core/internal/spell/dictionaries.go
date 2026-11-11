package spell

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// dictionaryDirs follows hunspell's own search order.
func dictionaryDirs(getenv func(string) string, home string) []string {
	var dirs []string
	for _, dir := range strings.Split(getenv("DICPATH"), ":") {
		if dir != "" {
			dirs = append(dirs, dir)
		}
	}
	return append(dirs,
		filepath.Join(home, ".local/share/hunspell"),
		"/usr/local/share/hunspell",
		"/usr/share/hunspell",
		filepath.Join(home, ".nix-profile/share/hunspell"),
		"/run/current-system/sw/share/hunspell",
		"/usr/share/myspell",
		"/usr/share/myspell/dicts",
	)
}

// listDictionaries returns every <code>.aff with a matching .dic, sorted by code. The first dir wins.
func listDictionaries(dirs []string) []Language {
	seen := map[string]bool{}
	var out []Language
	for _, dir := range dirs {
		affs, _ := filepath.Glob(filepath.Join(dir, "*.aff"))
		for _, aff := range affs {
			code := strings.TrimSuffix(filepath.Base(aff), ".aff")
			if seen[code] || strings.HasPrefix(code, "hyph_") || strings.HasPrefix(code, "th_") {
				continue
			}
			base := strings.TrimSuffix(aff, ".aff")
			if _, err := os.Stat(base + ".dic"); err != nil {
				continue
			}
			seen[code] = true
			out = append(out, Language{Code: code, Path: base})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// localeCandidates lists the locales "auto" tries, in the order gettext reads them.
func localeCandidates(getenv func(string) string) []string {
	var out []string
	for _, l := range strings.Split(getenv("LANGUAGE"), ":") {
		out = append(out, normalizeLocale(l))
	}
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		out = append(out, normalizeLocale(getenv(name)))
	}
	kept := out[:0]
	for _, l := range out {
		if l != "" {
			kept = append(kept, l)
		}
	}
	if len(kept) == 0 {
		return []string{"en_US"}
	}
	return kept
}

// normalizeLocale turns "de_DE.UTF-8@euro" into "de_DE" and C or POSIX into en_US.
func normalizeLocale(l string) string {
	if i := strings.IndexAny(l, ".@"); i >= 0 {
		l = l[:i]
	}
	l = strings.ReplaceAll(l, "-", "_")
	if l == "C" || l == "POSIX" {
		return "en_US"
	}
	return l
}

// pickLanguage takes an exact match for any candidate first, then the same language, preferring de_DE for de
// and en_US for en.
func pickLanguage(candidates []string, langs []Language) (Language, bool) {
	byCode := map[string]Language{}
	for _, l := range langs {
		byCode[l.Code] = l
	}
	for _, c := range candidates {
		if l, ok := byCode[c]; ok {
			return l, true
		}
	}
	for _, c := range candidates {
		lang, _, _ := strings.Cut(c, "_")
		preferred := []string{lang + "_" + strings.ToUpper(lang), lang}
		if lang == "en" {
			preferred = []string{"en_US", "en"}
		}
		for _, p := range preferred {
			if l, ok := byCode[p]; ok {
				return l, true
			}
		}
		for _, l := range langs {
			if strings.HasPrefix(l.Code, lang+"_") {
				return l, true
			}
		}
	}
	return Language{}, false
}
