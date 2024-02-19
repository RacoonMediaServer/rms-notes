package service

import (
	"context"
	"fmt"
	"github.com/RacoonMediaServer/rms-notes/internal/config"
	"github.com/RacoonMediaServer/rms-notes/internal/nextcloud"
	"github.com/RacoonMediaServer/rms-notes/internal/obsidian"
	"github.com/RacoonMediaServer/rms-packages/pkg/pubsub"
	rms_bot_client "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-bot-client"
	rms_notes "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-notes"
	"github.com/RacoonMediaServer/rms-packages/pkg/service/servicemgr"
	"go-micro.dev/v4"
	"go-micro.dev/v4/logger"
	"google.golang.org/protobuf/types/known/emptypb"
	"time"
)

type Notes struct {
	db  Database
	o   *obsidian.Manager
	pub micro.Event
	bot rms_bot_client.RmsBotClientService
}

func (n *Notes) AddNote(ctx context.Context, request *rms_notes.AddNoteRequest, empty *emptypb.Empty) error {
	if err := n.o.NewNote(request.Title, request.Text); err != nil {
		logger.Errorf("Create a new note failed: %s", err)
		return err
	}

	logger.Infof("Note '%s' created", request.Title)
	return nil
}

func (n *Notes) AddTask(ctx context.Context, request *rms_notes.AddTaskRequest, empty *emptypb.Empty) error {
	t := obsidian.Task{Text: request.Text}
	if request.DueDate != nil {
		date, err := time.Parse(obsidian.DateFormat, *request.DueDate)
		if err != nil {
			err = fmt.Errorf("invalid date: %s", *request.DueDate)
			logger.Warn(err)
			return err
		}
		t.DueDate = &date
	}
	if err := n.o.AddTask(t); err != nil {
		logger.Errorf("Add task failed: %s", err)
		return err
	}
	return nil
}

func (n *Notes) SnoozeTask(ctx context.Context, request *rms_notes.SnoozeTaskRequest, empty *emptypb.Empty) error {
	date := time.Now().AddDate(0, 0, 1)
	var err error
	if request.DueDate != nil {
		date, err = time.Parse(obsidian.DateFormat, *request.DueDate)
		if err != nil {
			return fmt.Errorf("invalid date format: %s", err)
		}
	}
	if err = n.o.Snooze(request.Id, date); err != nil {
		logger.Errorf("Cannot snooze task %s to %s: %s", request.Id, date, err)
		return err
	}

	return nil
}

func (n *Notes) DoneTask(ctx context.Context, request *rms_notes.DoneTaskRequest, empty *emptypb.Empty) error {
	if err := n.o.Done(request.Id); err != nil {
		logger.Errorf("Done task %s failed: %s", request.Id, err)
		return err
	}

	return nil
}

func (n *Notes) GetSettings(ctx context.Context, empty *emptypb.Empty, settings *rms_notes.NotesSettings) error {
	loaded, err := n.db.LoadSettings()
	if err != nil {
		err = fmt.Errorf("load settings failed: %w", err)
		logger.Error(err)
		return err
	}

	settings.Directory = loaded.Directory
	settings.NotesDirectory = loaded.NotesDirectory
	settings.TasksFile = loaded.TasksFile
	settings.NotificationTime = loaded.NotificationTime

	return nil
}

func (n *Notes) SetSettings(ctx context.Context, settings *rms_notes.NotesSettings, empty *emptypb.Empty) error {
	if err := n.db.SaveSettings(settings); err != nil {
		logger.Errorf("Save settings failed: %s", err)
		return err
	}

	n.o.Stop()
	n.o = obsidian.New(settings, n.pub, n.bot, nextcloud.NewClient(config.Config().WebDAV))

	return nil
}

func New(db Database, s servicemgr.ClientFactory) (*Notes, error) {
	settings, err := db.LoadSettings()
	if err != nil {
		return nil, err
	}

	pub := pubsub.NewPublisher(s)
	f := servicemgr.NewServiceFactory(s)
	bot := f.NewBotClient()

	return &Notes{
		db:  db,
		o:   obsidian.New(settings, pub, bot, nextcloud.NewClient(config.Config().WebDAV)),
		pub: pub,
		bot: bot,
	}, nil
}
