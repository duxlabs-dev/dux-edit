package spell

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"strings"
)

// runner is the hunspell process boundary; tests fake it.
type runner interface {
	// Check returns the words dict rejects.
	Check(ctx context.Context, bin, dict string, words []string) (map[string]bool, error)
	Suggest(ctx context.Context, bin, dict, word string) ([]string, error)
}

type execRunner struct{}

func (execRunner) Check(ctx context.Context, bin, dict string, words []string) (map[string]bool, error) {
	cmd := exec.CommandContext(ctx, bin, "-i", "UTF-8", "-L", "-d", dict)
	cmd.Stdin = strings.NewReader(strings.Join(words, "\n") + "\n")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	bad := map[string]bool{}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		if w := strings.TrimSpace(scanner.Text()); w != "" {
			bad[w] = true
		}
	}
	return bad, scanner.Err()
}

func (execRunner) Suggest(ctx context.Context, bin, dict, word string) ([]string, error) {
	cmd := exec.CommandContext(ctx, bin, "-i", "UTF-8", "-a", "-d", dict)
	// "^" makes ispell pipe mode read the line as text, never as a command.
	cmd.Stdin = strings.NewReader("^" + word + "\n")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseSuggestions(string(out)), nil
}

// parseSuggestions reads the "& word count offset: a, b" line of ispell pipe output.
func parseSuggestions(out string) []string {
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "& ") {
			continue
		}
		_, list, ok := strings.Cut(line, ": ")
		if !ok {
			return nil
		}
		return strings.Split(strings.TrimSpace(list), ", ")
	}
	return nil
}
