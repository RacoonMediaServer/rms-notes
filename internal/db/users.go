package db

import "github.com/RacoonMediaServer/rms-notes/internal/model"

func (d *Database) LoadUsers() (map[int32]*model.NotesUser, error) {
	var users []*model.NotesUser
	if err := d.conn.Find(&users).Error; err != nil {
		return nil, err
	}
	result := map[int32]*model.NotesUser{}
	for _, u := range users {
		result[u.TelegramUser] = u
	}
	return result, nil
}

func (d *Database) AddUser(camera *model.NotesUser) error {
	return d.conn.Create(camera).Error
}

func (d *Database) RemoveUser(id int32) error {
	return d.conn.Model(&model.NotesUser{}).Unscoped().Delete(&model.NotesUser{}, id).Error
}
