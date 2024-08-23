package service

import (
	"context"
	"fmt"
	"time"

	"github.com/RacoonMediaServer/rms-notes/internal/obsidian"
	"github.com/RacoonMediaServer/rms-packages/pkg/communication"
	rms_bot_client "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-bot-client"
	"go-micro.dev/v4/logger"
)

func (n *Notes) runScheduleEvents() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.job != nil {
		n.sched.RemoveByReference(n.job)
		n.job = nil
	}
	notifyTime := fmt.Sprintf("%02d:00", n.settings.NotificationTime)
	n.job, _ = n.sched.Every(1).Day().At(notifyTime).Do(func() {
		n.mu.RLock()
		vaults := n.vaults
		n.mu.RUnlock()

		for u, v := range vaults {
			logger.Infof("Refreshing vault %d...", u)
			if err := v.Refresh(obsidian.Scheduled); err != nil {
				logger.Logf(logger.ErrorLevel, "Refresh vault %d failed: %s", u, err)
			}
			n.notifyAboutScheduledTasks(u, v.GetTasks())
		}
	})
}

func (n *Notes) notifyAboutScheduledTasks(user int32, tasks []*obsidian.Task) {
	now := time.Now()
	now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	for _, t := range tasks {
		if t.DueDate == nil || t.Done {
			continue
		}
		if now.Compare(*t.DueDate) >= 0 {
			logger.Infof("Task is expired: %s", t)

			_, err := n.bot.SendMessage(context.Background(), &rms_bot_client.SendMessageRequest{Message: &communication.BotMessage{
				Type: communication.MessageType_Interaction,
				Text: formatTask(t),
				Buttons: []*communication.Button{
					{
						Title:   "Отложить",
						Command: fmt.Sprintf("/tasks snooze %s", t.Hash()),
					},
					{
						Title:   "Выполнить",
						Command: fmt.Sprintf("/tasks done %s", t.Hash()),
					},
					{
						Title:   "Удалить",
						Command: fmt.Sprintf("/tasks remove %s", t.Hash()),
					},
				},
				KeyboardStyle: communication.KeyboardStyle_Message,
				Attachment:    nil,
				User:          user,
			}})

			if err != nil {
				logger.Errorf("Send notification failed: %s", err)
			}
		}
	}
}
