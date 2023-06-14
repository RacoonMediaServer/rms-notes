package obsidian

import (
	"bufio"
	"bytes"
	"strings"
)

func escapeFileName(fn string) string {
	rpl := strings.NewReplacer("#", " ", "^", " ", "[", " ", "]", " ", "|", " ")
	return rpl.Replace(fn)
}

func (m *Manager) loadFile(fileName string) ([]string, error) {
	data, err := m.nc.Download(fileName)
	if err != nil {
		return nil, err
	}

	var lines []string
	f := bytes.NewReader(data)
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		lines = append(lines, scan.Text())
	}

	return lines, scan.Err()
}

func (m *Manager) saveFile(fileName string, lines []string) error {
	var builder strings.Builder
	for _, line := range lines {
		builder.WriteString(line)
		builder.WriteString("\n")
	}

	return m.nc.Upload(fileName, []byte(builder.String()))
}
