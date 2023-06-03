package obsidian

import "fmt"

func formatTask(t *Task) string {
	result := "<b>Напоминание</b>\n\n"
	result += fmt.Sprintf("<b>Задача:</b> %s\n", t.Text)
	if t.Priority != PriorityNo {
		result += fmt.Sprintf("<b>Приоритет:</b> %s\n", t.Priority)
	}
	if t.Recurrent != RepetitionNo {
		result += fmt.Sprintf("<b>Повторение:</b> %s\n", t.Recurrent)
	}
	return result
}
