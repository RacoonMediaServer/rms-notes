package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/RacoonMediaServer/rms-notes/internal/model"
	"github.com/RacoonMediaServer/rms-notes/internal/nextcloud"
	"github.com/RacoonMediaServer/rms-notes/internal/obsidian"
	"github.com/RacoonMediaServer/rms-packages/pkg/pubsub"
	rms_bot_client "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-bot-client"
	rms_notes "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-notes"
	"github.com/RacoonMediaServer/rms-packages/pkg/service/servicemgr"
	"github.com/go-co-op/gocron"
	"go-micro.dev/v4"
	"go-micro.dev/v4/logger"
	"google.golang.org/protobuf/types/known/emptypb"
)

const refreshRetryInterval = 60 * time.Second

type Notes struct {
	db  Database
	pub micro.Event
	bot rms_bot_client.RmsBotClientService

	sched *gocron.Scheduler

	mu       sync.RWMutex
	settings *rms_notes.NotesSettings
	users    map[int32]*model.NotesUser
	vaults   map[int32]*obsidian.Vault
	job      *gocron.Job
	ctx      context.Context
	cancel   context.CancelFunc
}

// RemoveTask implements rms_notes.RmsNotesHandler.
func (n *Notes) RemoveTask(ctx context.Context, request *rms_notes.RemoveTaskRequest, response *emptypb.Empty) error {
	n.mu.RLock()
	o, ok := n.vaults[request.User]
	n.mu.RUnlock()

	if !ok {
		return errors.New("user must login")
	}

	if err := o.RemoveTask(request.Id); err != nil {
		logger.Errorf("Remove task %s failed: %s", request.Id, err)
		return err
	}

	return nil
}

// SendTasksNotification implements rms_notes.RmsNotesHandler.
func (n *Notes) SendTasksNotification(ctx context.Context, request *rms_notes.SendTasksNotificationRequest, response *emptypb.Empty) error {
	n.mu.RLock()
	o, ok := n.vaults[request.User]
	n.mu.RUnlock()

	if !ok {
		return errors.New("user must login")
	}

	n.notifyAboutScheduledTasks(request.User, o.GetTasks())
	return nil
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

	// TODO: check access
	if err := n.db.AddUser(&user); err != nil {
		return fmt.Errorf("add user to database failed: %w", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	n.users[request.User] = &user
	n.vaults[request.User] = n.createVault(&user, n.settings.Directory)

	return nil
}

func (n *Notes) AddNote(ctx context.Context, request *rms_notes.AddNoteRequest, empty *emptypb.Empty) error {
	n.mu.RLock()
	o, ok := n.vaults[request.User]
	notesDirectory := n.settings.NotesDirectory
	n.mu.RUnlock()

	if !ok {
		return errors.New("user must login")
	}

	if err := o.AddNote(notesDirectory, request.Title, request.Text); err != nil {
		logger.Errorf("Create a new note failed: %s", err)
		return err
	}

	logger.Infof("Note '%s' created", request.Title)
	return nil
}

func (n *Notes) AddTask(ctx context.Context, request *rms_notes.AddTaskRequest, empty *emptypb.Empty) error {
	n.mu.RLock()
	o, ok := n.vaults[request.User]
	tasksFile := n.settings.TasksFile
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
	if err := o.AddTask(tasksFile, &t); err != nil {
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
	if err = o.SnoozeTask(request.Id, date); err != nil {
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

	if err := o.DoneTask(request.Id); err != nil {
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
	n.settings = settings

	n.cancel()
	n.ctx, n.cancel = context.WithCancel(context.Background())

	n.vaults = make(map[int32]*obsidian.Vault)
	for _, u := range n.users {
		n.vaults[u.TelegramUser] = n.createVault(u, settings.Directory)
	}
	n.mu.Unlock()

	n.runScheduleEvents()

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

	ctx, cancel := context.WithCancel(context.Background())
	n := &Notes{
		db:       db,
		pub:      pub,
		bot:      bot,
		users:    users,
		settings: settings,
		vaults:   make(map[int32]*obsidian.Vault),
		sched:    gocron.NewScheduler(time.Local),
		ctx:      ctx,
		cancel:   cancel,
	}

	for _, u := range users {
		n.vaults[u.TelegramUser] = n.createVault(u, settings.Directory)
	}

	n.runScheduleEvents()
	n.sched.StartAsync()

	return n, nil
}

func (n *Notes) createVault(user *model.NotesUser, directory string) *obsidian.Vault {
	webDav := nextcloud.WebDAV{
		Root:     user.Endpoint,
		User:     user.Login,
		Password: user.Password,
	}

	vault := obsidian.NewVault(n.ctx, directory, nextcloud.NewClient(webDav))
	vaultId := fmt.Sprintf("[%s / %d]", user.Login, user.TelegramUser)

	go func() {
		for {
			select {
			case <-n.ctx.Done():
				return
			case <-time.After(refreshRetryInterval):
				logger.Infof("Refreshing new vault %s...", vaultId)
				if err := vault.Refresh(obsidian.Scheduled); err == nil {
					logger.Infof("Tasks for vault %s loaded", vaultId)
					vault.StartWatchingChanges()
					n.notifyAboutScheduledTasks(user.TelegramUser, vault.GetTasks())
					return
				} else {
					logger.Errorf("Refresh obsidian vault %s failed: %s", vaultId, err)
				}
			}
		}
	}()
	return vault
}
