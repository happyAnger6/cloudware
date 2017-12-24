package security

import "cloudware/cloudware/api"

// FilterUserTeams filters teams based on user role.
// non-administrator users only have access to team they are member of.
func FilterUserTeams(teams []api.Team, context *RestrictedRequestContext) []api.Team {
	filteredTeams := teams

	if !context.IsAdmin {
		filteredTeams = make([]api.Team, 0)
		for _, membership := range context.UserMemberships {
			for _, team := range teams {
				if team.ID == membership.TeamID {
					filteredTeams = append(filteredTeams, team)
					break
				}
			}
		}
	}

	return filteredTeams
}

// FilterLeaderTeams filters teams based on user role.
// Team leaders only have access to team they lead.
func FilterLeaderTeams(teams []api.Team, context *RestrictedRequestContext) []api.Team {
	filteredTeams := teams

	if context.IsTeamLeader {
		filteredTeams = make([]api.Team, 0)
		for _, membership := range context.UserMemberships {
			for _, team := range teams {
				if team.ID == membership.TeamID && membership.Role == api.TeamLeader {
					filteredTeams = append(filteredTeams, team)
					break
				}
			}
		}
	}

	return filteredTeams
}

// FilterUsers filters users based on user role.
// Non-administrator users only have access to non-administrator users.
func FilterUsers(users []api.User, context *RestrictedRequestContext) []api.User {
	filteredUsers := users

	if !context.IsAdmin {
		filteredUsers = make([]api.User, 0)

		for _, user := range users {
			if user.Role != api.AdministratorRole {
				filteredUsers = append(filteredUsers, user)
			}
		}
	}

	return filteredUsers
}

// FilterRegistries filters registries based on user role and team memberships.
// Non administrator users only have access to authorized registries.
func FilterRegistries(registries []api.Registry, context *RestrictedRequestContext) ([]api.Registry, error) {

	filteredRegistries := registries
	if !context.IsAdmin {
		filteredRegistries = make([]api.Registry, 0)

		for _, registry := range registries {
			if isRegistryAccessAuthorized(&registry, context.UserID, context.UserMemberships) {
				filteredRegistries = append(filteredRegistries, registry)
			}
		}
	}

	return filteredRegistries, nil
}

// FilterEndpoints filters endpoints based on user role and team memberships.
// Non administrator users only have access to authorized endpoints.
func FilterEndpoints(endpoints []api.Endpoint, context *RestrictedRequestContext) ([]api.Endpoint, error) {
	filteredEndpoints := endpoints

	if !context.IsAdmin {
		filteredEndpoints = make([]api.Endpoint, 0)

		for _, endpoint := range endpoints {
			if isEndpointAccessAuthorized(&endpoint, context.UserID, context.UserMemberships) {
				filteredEndpoints = append(filteredEndpoints, endpoint)
			}
		}
	}

	return filteredEndpoints, nil
}

func isRegistryAccessAuthorized(registry *api.Registry, userID api.UserID, memberships []api.TeamMembership) bool {
	for _, authorizedUserID := range registry.AuthorizedUsers {
		if authorizedUserID == userID {
			return true
		}
	}
	for _, membership := range memberships {
		for _, authorizedTeamID := range registry.AuthorizedTeams {
			if membership.TeamID == authorizedTeamID {
				return true
			}
		}
	}
	return false
}

func isEndpointAccessAuthorized(endpoint *api.Endpoint, userID api.UserID, memberships []api.TeamMembership) bool {
	for _, authorizedUserID := range endpoint.AuthorizedUsers {
		if authorizedUserID == userID {
			return true
		}
	}
	for _, membership := range memberships {
		for _, authorizedTeamID := range endpoint.AuthorizedTeams {
			if membership.TeamID == authorizedTeamID {
				return true
			}
		}
	}
	return false
}
