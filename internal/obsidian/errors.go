package obsidian

import (
	"fmt"
)

type ErrorKind int

const (
	ErrAddNoteFailed ErrorKind = iota
	ErrAddTaskFailed
	ErrSnoozeTaskFailed
	ErrRemoveTaskFailed
	ErrDoneTaskFailed
)

type Error struct {
	Kind ErrorKind
	Err  error
	Item string
}

func (e *Error) Error() string {
	prefix := ""
	switch e.Kind {
	case ErrAddNoteFailed:
		prefix = fmt.Sprintf("add note '%s' failed", e.Item)
	case ErrAddTaskFailed:
		prefix = fmt.Sprintf("add task '%s' failed", e.Item)
	case ErrSnoozeTaskFailed:
		prefix = fmt.Sprintf("snooze task '%s' failed", e.Item)
	case ErrRemoveTaskFailed:
		prefix = fmt.Sprintf("remove task '%s' failed", e.Item)
	case ErrDoneTaskFailed:
		prefix = fmt.Sprintf("done task '%s' failed", e.Item)
	}

	return fmt.Sprintf("%s: %s", prefix, e.Err)
}

func makeError(kind ErrorKind, err error, item string) error {
	if err == nil {
		return nil
	}
	return &Error{Kind: kind, Err: err, Item: item}
}

func wrapDeferFn(kind ErrorKind, fn deferFn, item string) deferFn {
	return func() error {
		return makeError(kind, fn(), item)
	}
}
