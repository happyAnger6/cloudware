package bolt

import (
	"cloudware/cloudware/api"
	"cloudware/cloudware/bolt/internal"

	"github.com/boltdb/bolt"
)

// ResourceControlService represents a service for managing resource controls.
type ResourceControlService struct {
	store *Store
}

// ResourceControl returns a ResourceControl object by ID
func (service *ResourceControlService) ResourceControl(ID api.ResourceControlID) (*api.ResourceControl, error) {
	var data []byte
	err := service.store.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(resourceControlBucketName))
		value := bucket.Get(internal.Itob(int(ID)))
		if value == nil {
			return api.ErrResourceControlNotFound
		}

		data = make([]byte, len(value))
		copy(data, value)
		return nil
	})
	if err != nil {
		return nil, err
	}

	var resourceControl api.ResourceControl
	err = internal.UnmarshalResourceControl(data, &resourceControl)
	if err != nil {
		return nil, err
	}
	return &resourceControl, nil
}

// ResourceControlByResourceID returns a ResourceControl object by checking if the resourceID is equal
// to the main ResourceID or in SubResourceIDs
func (service *ResourceControlService) ResourceControlByResourceID(resourceID string) (*api.ResourceControl, error) {
	var resourceControl *api.ResourceControl

	err := service.store.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(resourceControlBucketName))
		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var rc api.ResourceControl
			err := internal.UnmarshalResourceControl(v, &rc)
			if err != nil {
				return err
			}
			if rc.ResourceID == resourceID {
				resourceControl = &rc
			}
			for _, subResourceID := range rc.SubResourceIDs {
				if subResourceID == resourceID {
					resourceControl = &rc
				}
			}
		}

		if resourceControl == nil {
			return api.ErrResourceControlNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resourceControl, nil
}

// ResourceControls returns all the ResourceControl objects
func (service *ResourceControlService) ResourceControls() ([]api.ResourceControl, error) {
	var rcs = make([]api.ResourceControl, 0)
	err := service.store.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(resourceControlBucketName))

		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var resourceControl api.ResourceControl
			err := internal.UnmarshalResourceControl(v, &resourceControl)
			if err != nil {
				return err
			}
			rcs = append(rcs, resourceControl)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return rcs, nil
}

// CreateResourceControl creates a new ResourceControl object
func (service *ResourceControlService) CreateResourceControl(resourceControl *api.ResourceControl) error {
	return service.store.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(resourceControlBucketName))
		id, _ := bucket.NextSequence()
		resourceControl.ID = api.ResourceControlID(id)
		data, err := internal.MarshalResourceControl(resourceControl)
		if err != nil {
			return err
		}

		err = bucket.Put(internal.Itob(int(resourceControl.ID)), data)
		if err != nil {
			return err
		}
		return nil
	})
}

// UpdateResourceControl saves a ResourceControl object.
func (service *ResourceControlService) UpdateResourceControl(ID api.ResourceControlID, resourceControl *api.ResourceControl) error {
	data, err := internal.MarshalResourceControl(resourceControl)
	if err != nil {
		return err
	}

	return service.store.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(resourceControlBucketName))
		err = bucket.Put(internal.Itob(int(ID)), data)

		if err != nil {
			return err
		}
		return nil
	})
}

// DeleteResourceControl deletes a ResourceControl object by ID
func (service *ResourceControlService) DeleteResourceControl(ID api.ResourceControlID) error {
	return service.store.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(resourceControlBucketName))
		err := bucket.Delete(internal.Itob(int(ID)))
		if err != nil {
			return err
		}
		return nil
	})
}
