package bolt

import "cloudware/cloudware/api"

func (m *Migrator) updateSettingsToDBVersion3() error {
	legacySettings, err := m.SettingsService.Settings()
	if err != nil {
		return err
	}

	legacySettings.AuthenticationMethod = api.AuthenticationInternal
	legacySettings.LDAPSettings = api.LDAPSettings{
		TLSConfig: api.TLSConfiguration{},
		SearchSettings: []api.LDAPSearchSettings{
			api.LDAPSearchSettings{},
		},
	}

	err = m.SettingsService.StoreSettings(legacySettings)
	if err != nil {
		return err
	}

	return nil
}
