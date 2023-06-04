package obsidian

import (
	"bufio"
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

func loadFile(fileName string) ([]string, error) {
	var lines []string
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scan := bufio.NewScanner(f)
	for scan.Scan() {
		lines = append(lines, scan.Text())
	}

	return lines, scan.Err()
}

func saveFile(fileName string, lines []string) error {
	f, err := os.OpenFile(fileName, os.O_WRONLY|os.O_TRUNC, 0655)
	if err != nil {
		return err
	}
	defer f.Close()

	wr := bufio.NewWriter(f)
	for _, l := range lines {
		if _, err = wr.WriteString(l + "\n"); err != nil {
			return err
		}
	}

	return wr.Flush()
}
