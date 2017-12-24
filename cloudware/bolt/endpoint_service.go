package bolt

import (
	"cloudware/cloudware/api"
	"cloudware/cloudware/bolt/internal"

	"github.com/boltdb/bolt"
)

// EndpointService represents a service for managing endpoints.
type EndpointService struct {
	store *Store
}

// Endpoint returns an endpoint by ID.
func (service *EndpointService) Endpoint(ID api.EndpointID) (*api.Endpoint, error) {
	var data []byte
	err := service.store.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(endpointBucketName))
		value := bucket.Get(internal.Itob(int(ID)))
		if value == nil {
			return api.ErrEndpointNotFound
		}

		data = make([]byte, len(value))
		copy(data, value)
		return nil
	})
	if err != nil {
		return nil, err
	}

	var endpoint api.Endpoint
	err = internal.UnmarshalEndpoint(data, &endpoint)
	if err != nil {
		return nil, err
	}
	return &endpoint, nil
}

// Endpoints return an array containing all the endpoints.
func (service *EndpointService) Endpoints() ([]api.Endpoint, error) {
	var endpoints = make([]api.Endpoint, 0)
	err := service.store.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(endpointBucketName))

		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			var endpoint api.Endpoint
			err := internal.UnmarshalEndpoint(v, &endpoint)
			if err != nil {
				return err
			}
			endpoints = append(endpoints, endpoint)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return endpoints, nil
}

// Synchronize creates, updates and deletes endpoints inside a single transaction.
func (service *EndpointService) Synchronize(toCreate, toUpdate, toDelete []*api.Endpoint) error {
	return service.store.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(endpointBucketName))

		for _, endpoint := range toCreate {
			err := storeNewEndpoint(endpoint, bucket)
			if err != nil {
				return err
			}
		}

		for _, endpoint := range toUpdate {
			err := marshalAndStoreEndpoint(endpoint, bucket)
			if err != nil {
				return err
			}
		}

		for _, endpoint := range toDelete {
			err := bucket.Delete(internal.Itob(int(endpoint.ID)))
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// CreateEndpoint assign an ID to a new endpoint and saves it.
func (service *EndpointService) CreateEndpoint(endpoint *api.Endpoint) error {
	return service.store.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(endpointBucketName))
		err := storeNewEndpoint(endpoint, bucket)
		if err != nil {
			return err
		}
		return nil
	})
}

// UpdateEndpoint updates an endpoint.
func (service *EndpointService) UpdateEndpoint(ID api.EndpointID, endpoint *api.Endpoint) error {
	data, err := internal.MarshalEndpoint(endpoint)
	if err != nil {
		return err
	}

	return service.store.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(endpointBucketName))
		err = bucket.Put(internal.Itob(int(ID)), data)
		if err != nil {
			return err
		}
		return nil
	})
}

// DeleteEndpoint deletes an endpoint.
func (service *EndpointService) DeleteEndpoint(ID api.EndpointID) error {
	return service.store.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(endpointBucketName))
		err := bucket.Delete(internal.Itob(int(ID)))
		if err != nil {
			return err
		}
		return nil
	})
}

func marshalAndStoreEndpoint(endpoint *api.Endpoint, bucket *bolt.Bucket) error {
	data, err := internal.MarshalEndpoint(endpoint)
	if err != nil {
		return err
	}

	err = bucket.Put(internal.Itob(int(endpoint.ID)), data)
	if err != nil {
		return err
	}
	return nil
}

func storeNewEndpoint(endpoint *api.Endpoint, bucket *bolt.Bucket) error {
	id, _ := bucket.NextSequence()
	endpoint.ID = api.EndpointID(id)
	return marshalAndStoreEndpoint(endpoint, bucket)
}
