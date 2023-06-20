package obsidian

import (
	"context"
	"fmt"
	"github.com/RacoonMediaServer/rms-packages/pkg/communication"
	"github.com/RacoonMediaServer/rms-packages/pkg/events"
	"github.com/RacoonMediaServer/rms-packages/pkg/misc"
	rms_bot_client "github.com/RacoonMediaServer/rms-packages/pkg/service/rms-bot-client"
	"go-micro.dev/v4/logger"
	"time"
)

const notificationTimeout = 20 * time.Second

func (m *Manager) observeFolder() {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.processEvents()
	}()
}

func (m *Manager) processEvents() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.w.OnChanged():
			logger.Infof("[obsidian] Something changed")
			if err := m.collectTasks(); err != nil {
				logger.Warnf("Extract tasks info from Obsidian folder failed: %s", err)
			}
			if !m.initiated {
				m.initiated = true
				m.checkScheduledTasks()
			}
		case <-m.check:
			m.checkScheduledTasks()
		}
	}
}

func (m *Manager) panicMalfunction(text string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), notificationTimeout)
	defer cancel()

	message := fmt.Sprintf("%s: %s", text, err)
	logger.Error(message)

	event := events.Malfunction{
		Timestamp:  time.Now().Unix(),
		Error:      message,
		System:     events.Malfunction_Services,
		Code:       events.Malfunction_ActionFailed,
		StackTrace: misc.GetStackTrace(),
	}

	if err := m.pub.Publish(ctx, &event); err != nil {
		logger.Warnf("Notify about malfunction failed: %s", err)
	}

	panic(message)
}

func (m *Manager) checkScheduledTasks() {
	logger.Infof("Check scheduled tasks")
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	for _, t := range m.tasks {
		if t.DueDate == nil || t.Done {
			continue
		}
		if now.Compare(*t.DueDate) >= 0 {
			logger.Infof("Task is expired: %s", t)

			_, err := m.bot.SendMessage(context.Background(), &rms_bot_client.SendMessageRequest{Message: &communication.BotMessage{
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
				},
				KeyboardStyle: communication.KeyboardStyle_Message,
				Attachment:    nil,
				User:          0,
			}})

			if err != nil {
				logger.Errorf("Send notification failed: %s", err)
			}
		}
	}
}
