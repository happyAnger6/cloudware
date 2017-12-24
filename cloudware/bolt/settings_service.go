package bolt

import (
	"cloudware/cloudware/api"
	"cloudware/cloudware/bolt/internal"

	"github.com/boltdb/bolt"
)

// SettingsService represents a service to manage application settings.
type SettingsService struct {
	store *Store
}

const (
	dbSettingsKey = "SETTINGS"
)

// Settings retrieve the settings object.
func (service *SettingsService) Settings() (*api.Settings, error) {
	var data []byte
	err := service.store.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(settingsBucketName))
		value := bucket.Get([]byte(dbSettingsKey))
		if value == nil {
			return api.ErrSettingsNotFound
		}

		data = make([]byte, len(value))
		copy(data, value)
		return nil
	})
	if err != nil {
		return nil, err
	}

	var settings api.Settings
	err = internal.UnmarshalSettings(data, &settings)
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

// StoreSettings persists a Settings object.
func (service *SettingsService) StoreSettings(settings *api.Settings) error {
	return service.store.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(settingsBucketName))

		data, err := internal.MarshalSettings(settings)
		if err != nil {
			return err
		}

		err = bucket.Put([]byte(dbSettingsKey), data)
		if err != nil {
			return err
		}
		return nil
	})
}
