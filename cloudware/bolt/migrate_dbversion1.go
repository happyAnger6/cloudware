package bolt

import (
	"github.com/boltdb/bolt"
	"cloudware/cloudware/api"
	"cloudware/cloudware/bolt/internal"
)

func (m *Migrator) updateResourceControlsToDBVersion2() error {
	legacyResourceControls, err := m.retrieveLegacyResourceControls()
	if err != nil {
		return err
	}

	for _, resourceControl := range legacyResourceControls {
		resourceControl.SubResourceIDs = []string{}
		resourceControl.TeamAccesses = []api.TeamResourceAccess{}

		owner, err := m.UserService.User(resourceControl.OwnerID)
		if err != nil {
			return err
		}

		if owner.Role == api.AdministratorRole {
			resourceControl.AdministratorsOnly = true
			resourceControl.UserAccesses = []api.UserResourceAccess{}
		} else {
			resourceControl.AdministratorsOnly = false
			userAccess := api.UserResourceAccess{
				UserID:      resourceControl.OwnerID,
				AccessLevel: api.ReadWriteAccessLevel,
			}
			resourceControl.UserAccesses = []api.UserResourceAccess{userAccess}
		}

		err = m.ResourceControlService.CreateResourceControl(&resourceControl)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Migrator) updateEndpointsToDBVersion2() error {
	legacyEndpoints, err := m.EndpointService.Endpoints()
	if err != nil {
		return err
	}

	for _, endpoint := range legacyEndpoints {
		endpoint.AuthorizedTeams = []api.TeamID{}
		err = m.EndpointService.UpdateEndpoint(endpoint.ID, &endpoint)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Migrator) retrieveLegacyResourceControls() ([]api.ResourceControl, error) {
	legacyResourceControls := make([]api.ResourceControl, 0)
	err := m.store.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("containerResourceControl"))
		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var resourceControl api.ResourceControl
			err := internal.UnmarshalResourceControl(v, &resourceControl)
			if err != nil {
				return err
			}
			resourceControl.Type = api.ContainerResourceControl
			legacyResourceControls = append(legacyResourceControls, resourceControl)
		}

		bucket = tx.Bucket([]byte("serviceResourceControl"))
		cursor = bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var resourceControl api.ResourceControl
			err := internal.UnmarshalResourceControl(v, &resourceControl)
			if err != nil {
				return err
			}
			resourceControl.Type = api.ServiceResourceControl
			legacyResourceControls = append(legacyResourceControls, resourceControl)
		}

		bucket = tx.Bucket([]byte("volumeResourceControl"))
		cursor = bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var resourceControl api.ResourceControl
			err := internal.UnmarshalResourceControl(v, &resourceControl)
			if err != nil {
				return err
			}
			resourceControl.Type = api.VolumeResourceControl
			legacyResourceControls = append(legacyResourceControls, resourceControl)
		}
		return nil
	})
	return legacyResourceControls, err
}
