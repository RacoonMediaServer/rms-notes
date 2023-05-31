package service

import (
	"context"
	rms_notes "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-notes"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Notes struct {
	db       Database
	settings *rms_notes.NotesSettings
}

func (n Notes) AddNote(ctx context.Context, request *rms_notes.AddNoteRequest, empty *emptypb.Empty) error {
	//TODO implement me
	panic("implement me")
}

func (n Notes) AddTask(ctx context.Context, request *rms_notes.AddTaskRequest, empty *emptypb.Empty) error {
	//TODO implement me
	panic("implement me")
}

func (n Notes) SnoozeTask(ctx context.Context, request *rms_notes.SnoozeTaskRequest, empty *emptypb.Empty) error {
	//TODO implement me
	panic("implement me")
}

func (n Notes) DoneTask(ctx context.Context, request *rms_notes.DoneTaskRequest, empty *emptypb.Empty) error {
	//TODO implement me
	panic("implement me")
}

func (n Notes) GetSettings(ctx context.Context, empty *emptypb.Empty, settings *rms_notes.NotesSettings) error {
	//TODO implement me
	panic("implement me")
}

func (n Notes) SetSettings(ctx context.Context, settings *rms_notes.NotesSettings, empty *emptypb.Empty) error {
	//TODO implement me
	panic("implement me")
}

func New(db Database) (*Notes, error) {
	settings, err := db.LoadSettings()
	if err != nil {
		return nil, err
	}

	return &Notes{
		db:       db,
		settings: settings,
	}, nil
}
