package obsidian

import (
	"bufio"
	"bytes"
	"strings"
)

func (v *Vault) extractTasks(fileName string, selector taskSelector) ([]*Task, error) {
	var tasks []*Task
	data, err := v.vault.Read(fileName)
	if err != nil {
		return nil, err
	}
	f := bytes.NewReader(data)

	scan := bufio.NewScanner(f)
	for scan.Scan() {
		if t := ParseTask(scan.Text()); t != nil && selector(t) {
			tasks = append(tasks, t)
		}
	}

	return tasks, nil
}

func escapeFileName(fn string) string {
	rpl := strings.NewReplacer("#", " ", "^", " ", "[", " ", "]", " ", "|", " ")
	return rpl.Replace(fn)
}

func (m *Vault) loadNote(fileName string) ([]string, error) {
	data, err := m.vault.Read(fileName)
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

func (m *Vault) saveNote(fileName string, lines []string) error {
	var builder strings.Builder
	for _, line := range lines {
		builder.WriteString(line)
		builder.WriteString("\n")
	}

	return m.vault.Write(fileName, []byte(builder.String()))
}
