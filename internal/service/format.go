package service

import (
	"fmt"

	"github.com/RacoonMediaServer/rms-notes/internal/obsidian"
)

func formatTask(t *obsidian.Task) string {
	result := "<b>Напоминание</b>\n\n"
	result += fmt.Sprintf("<b>Задача:</b> %s\n", t.Text)
	if t.Priority != obsidian.PriorityNo {
		result += fmt.Sprintf("<b>Приоритет:</b> %s\n", t.Priority)
	}
	if t.Recurrent != obsidian.RepetitionNo {
		result += fmt.Sprintf("<b>Повторение:</b> %s\n", t.Recurrent)
	}
	return result
}
