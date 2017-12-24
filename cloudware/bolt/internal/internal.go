package internal

import (
	"cloudware/cloudware/api"

	"encoding/binary"
	"encoding/json"
)

// MarshalUser encodes a user to binary format.
func MarshalUser(user *api.User) ([]byte, error) {
	return json.Marshal(user)
}

// UnmarshalUser decodes a user from a binary data.
func UnmarshalUser(data []byte, user *api.User) error {
	return json.Unmarshal(data, user)
}

// MarshalTeam encodes a team to binary format.
func MarshalTeam(team *api.Team) ([]byte, error) {
	return json.Marshal(team)
}

// UnmarshalTeam decodes a team from a binary data.
func UnmarshalTeam(data []byte, team *api.Team) error {
	return json.Unmarshal(data, team)
}

// MarshalTeamMembership encodes a team membership to binary format.
func MarshalTeamMembership(membership *api.TeamMembership) ([]byte, error) {
	return json.Marshal(membership)
}

// UnmarshalTeamMembership decodes a team membership from a binary data.
func UnmarshalTeamMembership(data []byte, membership *api.TeamMembership) error {
	return json.Unmarshal(data, membership)
}

// MarshalEndpoint encodes an endpoint to binary format.
func MarshalEndpoint(endpoint *api.Endpoint) ([]byte, error) {
	return json.Marshal(endpoint)
}

// UnmarshalEndpoint decodes an endpoint from a binary data.
func UnmarshalEndpoint(data []byte, endpoint *api.Endpoint) error {
	return json.Unmarshal(data, endpoint)
}

// MarshalStack encodes a stack to binary format.
func MarshalStack(stack *api.Stack) ([]byte, error) {
	return json.Marshal(stack)
}

// UnmarshalStack decodes a stack from a binary data.
func UnmarshalStack(data []byte, stack *api.Stack) error {
	return json.Unmarshal(data, stack)
}

// MarshalRegistry encodes a registry to binary format.
func MarshalRegistry(registry *api.Registry) ([]byte, error) {
	return json.Marshal(registry)
}

// UnmarshalRegistry decodes a registry from a binary data.
func UnmarshalRegistry(data []byte, registry *api.Registry) error {
	return json.Unmarshal(data, registry)
}

// MarshalResourceControl encodes a resource control object to binary format.
func MarshalResourceControl(rc *api.ResourceControl) ([]byte, error) {
	return json.Marshal(rc)
}

// UnmarshalResourceControl decodes a resource control object from a binary data.
func UnmarshalResourceControl(data []byte, rc *api.ResourceControl) error {
	return json.Unmarshal(data, rc)
}

// MarshalSettings encodes a settings object to binary format.
func MarshalSettings(settings *api.Settings) ([]byte, error) {
	return json.Marshal(settings)
}

// UnmarshalSettings decodes a settings object from a binary data.
func UnmarshalSettings(data []byte, settings *api.Settings) error {
	return json.Unmarshal(data, settings)
}

// MarshalDockerHub encodes a Dockerhub object to binary format.
func MarshalDockerHub(settings *api.DockerHub) ([]byte, error) {
	return json.Marshal(settings)
}

// UnmarshalDockerHub decodes a Dockerhub object from a binary data.
func UnmarshalDockerHub(data []byte, settings *api.DockerHub) error {
	return json.Unmarshal(data, settings)
}

// Itob returns an 8-byte big endian representation of v.
// This function is typically used for encoding integer IDs to byte slices
// so that they can be used as BoltDB keys.
func Itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}
