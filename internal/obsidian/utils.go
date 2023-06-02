package obsidian

import (
	"os"
	"strings"
)

func escapeFileName(fn string) string {
	rpl := strings.NewReplacer("#", " ", "^", " ", "[", " ", "]", " ", "|", " ")
	return rpl.Replace(fn)
}

func isFileHidden(path string) bool {
	dirs := strings.Split(path, string(os.PathSeparator))
	for _, d := range dirs {
		if strings.HasPrefix(d, ".") {
			return true
		}
	}

	return false
}
