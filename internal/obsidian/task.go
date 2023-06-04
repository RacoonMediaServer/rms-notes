package obsidian

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	taskStart     = regexp.MustCompile(`^\s*(\*|-) \[(x| |X)\]\s*`)
	dueDateRegex  = regexp.MustCompile(`📅 (\d\d\d\d-\d\d-\d\d)`)
	doneDateRegex = regexp.MustCompile(`✅ (\d\d\d\d-\d\d-\d\d)`)
)

type Priority int

const (
	PriorityNo Priority = iota
	PriorityLow
	PriorityMedium
	PriorityHigh
)

type Repetition int

const (
	RepetitionNo Repetition = iota
	RepetitionEveryDay
	RepetitionEveryWeek
	RepetitionEveryMonth
	RepetitionEveryYear
)

type Task struct {
	Text      string
	DueDate   *time.Time
	Priority  Priority
	Recurrent Repetition
	Done      bool
	DoneDate  *time.Time
}

func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "🔽"
	case PriorityMedium:
		return "🔼"
	case PriorityHigh:
		return "⏫"
	default:
		return ""
	}
}

func (r Repetition) String() string {
	switch r {
	case RepetitionEveryDay:
		return "🔁 every day"
	case RepetitionEveryWeek:
		return "🔁 every week"
	case RepetitionEveryMonth:
		return "🔁 every month"
	case RepetitionEveryYear:
		return "🔁 every year"
	default:
		return ""
	}
}

func (t Task) String() string {
	done := " "
	if t.Done {
		done = "x"
	}
	result := fmt.Sprintf("* [%s] %s", done, t.Text)
	if t.Priority != PriorityNo {
		result += fmt.Sprintf(" %s", t.Priority)
	}
	if t.Recurrent != RepetitionNo {
		result += fmt.Sprintf(" %s", t.Recurrent)
	}
	if t.DueDate != nil {
		result += " 📅 " + t.DueDate.Format(DateFormat)
	}
	if t.DoneDate != nil {
		result += " ✅ " + t.DoneDate.Format(DateFormat)
	}
	return result
}

func (t Task) Hash() string {
	bytes := t.String()
	h := sha256.New()
	h.Write([]byte(bytes))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func ParseTask(line string) *Task {
	found := taskStart.FindStringSubmatch(line)
	if len(found) != 3 {
		return nil
	}
	t := &Task{}
	if found[2] != " " {
		t.Done = true
	}
	line = strings.Replace(line, found[0], "", 1)

	for p := PriorityLow; p <= PriorityHigh; p++ {
		if strings.Index(line, p.String()) != -1 {
			t.Priority = p
			line = strings.Replace(line, p.String(), "", 1)
			break
		}
	}

	for r := RepetitionEveryDay; r <= RepetitionEveryYear; r++ {
		if strings.Index(line, r.String()) != -1 {
			t.Recurrent = r
			line = strings.Replace(line, r.String(), "", 1)
			break
		}
	}

	found = dueDateRegex.FindStringSubmatch(line)
	if len(found) == 2 {
		line = strings.Replace(line, found[0], "", 1)
		date, err := time.Parse(DateFormat, found[1])
		if err == nil {
			t.DueDate = &date
		}
	}

	found = doneDateRegex.FindStringSubmatch(line)
	if len(found) == 2 {
		line = strings.Replace(line, found[0], "", 1)
		date, err := time.Parse(DateFormat, found[1])
		if err == nil {
			t.DoneDate = &date
		}
	}
	t.Text = strings.Trim(line, " ")
	return t
}

func (t Task) NextDate() time.Time {
	switch t.Recurrent {
	case RepetitionNo:
		return *t.DueDate
	case RepetitionEveryDay:
		return t.DueDate.Add(24 * time.Hour)
	case RepetitionEveryWeek:
		return t.DueDate.Add(24 * 7 * time.Hour)

		// TODO: нормальный расчет следующей даты
	case RepetitionEveryMonth:
		return t.DueDate.Add(30 * 24 * time.Hour)
	case RepetitionEveryYear:
		return t.DueDate.Add(365 * 24 * time.Hour)
	}

	return time.Now()
}
