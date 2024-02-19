package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/RacoonMediaServer/rms-notes/internal/model"
	"github.com/RacoonMediaServer/rms-notes/internal/nextcloud"
	"github.com/RacoonMediaServer/rms-notes/internal/obsidian"
	"github.com/RacoonMediaServer/rms-packages/pkg/pubsub"
	rms_bot_client "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-bot-client"
	rms_notes "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-notes"
	"github.com/RacoonMediaServer/rms-packages/pkg/service/servicemgr"
	"go-micro.dev/v4"
	"go-micro.dev/v4/logger"
	"google.golang.org/protobuf/types/known/emptypb"
	"sync"
	"time"
)

type Notes struct {
	db  Database
	pub micro.Event
	bot rms_bot_client.RmsBotClientService

	mu       sync.RWMutex
	settings *rms_notes.NotesSettings
	users    map[int32]*model.NotesUser
	vaults   map[int32]*obsidian.Vault
}

func (n *Notes) IsUserLogged(ctx context.Context, request *rms_notes.IsUserLoggedRequest, response *rms_notes.IsUserLoggedResponse) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	_, response.Result = n.users[request.User]
	return nil
}

func (n *Notes) UserLogin(ctx context.Context, request *rms_notes.UserLoginRequest, response *rms_notes.UserLoginResponse) error {
	user := model.NotesUser{
		TelegramUser: request.User,
		Endpoint:     request.Endpoint,
		Login:        request.Login,
		Password:     request.Password,
	}
	o := n.createVault(&user)

	// TODO: check access
	if err := n.db.AddUser(&user); err != nil {
		return fmt.Errorf("add user to database failed: %w", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	n.users[request.User] = &user
	n.vaults[request.User] = o

	return nil
}

func (n *Notes) AddNote(ctx context.Context, request *rms_notes.AddNoteRequest, empty *emptypb.Empty) error {
	n.mu.RLock()
	o, ok := n.vaults[request.User]
	n.mu.RUnlock()

	if !ok {
		return errors.New("user must login")
	}

	if err := o.NewNote(request.Title, request.Text); err != nil {
		logger.Errorf("Create a new note failed: %s", err)
		return err
	}

	logger.Infof("Note '%s' created", request.Title)
	return nil
}

func (n *Notes) AddTask(ctx context.Context, request *rms_notes.AddTaskRequest, empty *emptypb.Empty) error {
	n.mu.RLock()
	o, ok := n.vaults[request.User]
	n.mu.RUnlock()

	if !ok {
		return errors.New("user must login")
	}

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
	if err := o.AddTask(t); err != nil {
		logger.Errorf("Add task failed: %s", err)
		return err
	}
	return nil
}

func (n *Notes) SnoozeTask(ctx context.Context, request *rms_notes.SnoozeTaskRequest, empty *emptypb.Empty) error {
	n.mu.RLock()
	o, ok := n.vaults[request.User]
	n.mu.RUnlock()

	if !ok {
		return errors.New("user must login")
	}

	date := time.Now().AddDate(0, 0, 1)
	var err error
	if request.DueDate != nil {
		date, err = time.Parse(obsidian.DateFormat, *request.DueDate)
		if err != nil {
			return fmt.Errorf("invalid date format: %s", err)
		}
	}
	if err = o.Snooze(request.Id, date); err != nil {
		logger.Errorf("Cannot snooze task %s to %s: %s", request.Id, date, err)
		return err
	}

	return nil
}

func (n *Notes) DoneTask(ctx context.Context, request *rms_notes.DoneTaskRequest, empty *emptypb.Empty) error {
	n.mu.RLock()
	o, ok := n.vaults[request.User]
	n.mu.RUnlock()

	if !ok {
		return errors.New("user must login")
	}

	if err := o.Done(request.Id); err != nil {
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

	n.mu.Lock()
	defer n.mu.Unlock()

	n.settings = settings

	for _, o := range n.vaults {
		o.Stop()
	}
	n.vaults = map[int32]*obsidian.Vault{}
	for _, user := range n.users {
		n.vaults[user.TelegramUser] = n.createVault(user)
	}

	return nil
}

func New(db Database, s servicemgr.ClientFactory) (*Notes, error) {
	settings, err := db.LoadSettings()
	if err != nil {
		return nil, err
	}

	users, err := db.LoadUsers()
	if err != nil {
		return nil, err
	}

	pub := pubsub.NewPublisher(s)
	f := servicemgr.NewServiceFactory(s)
	bot := f.NewBotClient()

	n := &Notes{
		db:       db,
		pub:      pub,
		bot:      bot,
		users:    users,
		settings: settings,
		vaults:   make(map[int32]*obsidian.Vault),
	}

	for _, u := range users {
		n.vaults[u.TelegramUser] = n.createVault(u)
	}

	return n, nil
}

func (n *Notes) createVault(user *model.NotesUser) *obsidian.Vault {
	webDav := nextcloud.WebDAV{
		Root:     user.Endpoint,
		User:     user.Login,
		Password: user.Password,
	}

	return obsidian.New(n.settings, n.pub, n.bot, nextcloud.NewClient(webDav), user.TelegramUser)
}
