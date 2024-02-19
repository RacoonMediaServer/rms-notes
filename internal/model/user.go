package model

type NotesUser struct {
	TelegramUser int32 `gorm:"primaryKey"`
	Endpoint     string
	Login        string
	Password     string
}
