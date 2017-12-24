package proxy

import "cloudware/cloudware/api"

type (
	// ExtendedStack represents a stack combined with its associated access control
	ExtendedStack struct {
		api.Stack
		ResourceControl api.ResourceControl `json:"ResourceControl"`
	}
)

// applyResourceAccessControl returns an optionally decorated object as the first return value and the
// access level for the user (granted or denied) as the second return value.
// It will retrieve an identifier from the labels object. If an identifier exists, it will check for
// an existing resource control associated to it.
// Returns a decorated object and authorized access (true) when a resource control is found and the user can access the resource.
// Returns the original object and authorized access (true) when no resource control is found.
// Returns the original object and denied access (false) when a resource control is found and the user cannot access the resource.
func applyResourceAccessControlFromLabel(labelsObject, resourceObject map[string]interface{}, labelIdentifier string,
	context *restrictedOperationContext) (map[string]interface{}, bool) {

	if labelsObject != nil && labelsObject[labelIdentifier] != nil {
		resourceIdentifier := labelsObject[labelIdentifier].(string)
		return applyResourceAccessControl(resourceObject, resourceIdentifier, context)
	}
	return resourceObject, true
}

// applyResourceAccessControl returns an optionally decorated object as the first return value and the
// access level for the user (granted or denied) as the second return value.
// Returns a decorated object and authorized access (true) when a resource control is found to the specified resource
// identifier and the user can access the resource.
// Returns the original object and authorized access (true) when no resource control is found for the specified
// resource identifier.
// Returns the original object and denied access (false) when a resource control is associated to the resource
// and the user cannot access the resource.
func applyResourceAccessControl(resourceObject map[string]interface{}, resourceIdentifier string,
	context *restrictedOperationContext) (map[string]interface{}, bool) {

	authorizedAccess := true

	resourceControl := getResourceControlByResourceID(resourceIdentifier, context.resourceControls)
	if resourceControl != nil {
		if context.isAdmin || canUserAccessResource(context.userID, context.userTeamIDs, resourceControl) {
			resourceObject = decorateObject(resourceObject, resourceControl)
		} else {
			authorizedAccess = false
		}
	}

	return resourceObject, authorizedAccess
}

// decorateResourceWithAccessControlFromLabel will retrieve an identifier from the labels object. If an identifier exists,
// it will check for an existing resource control associated to it. If a resource control is found, the resource object will be
// decorated. If no identifier can be found in the labels or no resource control is associated to the identifier, the resource
// object will not be changed.
func decorateResourceWithAccessControlFromLabel(labelsObject, resourceObject map[string]interface{}, labelIdentifier string,
	resourceControls []api.ResourceControl) map[string]interface{} {

	if labelsObject != nil && labelsObject[labelIdentifier] != nil {
		resourceIdentifier := labelsObject[labelIdentifier].(string)
		resourceObject = decorateResourceWithAccessControl(resourceObject, resourceIdentifier, resourceControls)
	}

	return resourceObject
}

// decorateResourceWithAccessControl will check if a resource control is associated to the specified resource identifier.
// If a resource control is found, the resource object will be decorated, otherwise it will not be changed.
func decorateResourceWithAccessControl(resourceObject map[string]interface{}, resourceIdentifier string,
	resourceControls []api.ResourceControl) map[string]interface{} {

	resourceControl := getResourceControlByResourceID(resourceIdentifier, resourceControls)
	if resourceControl != nil {
		return decorateObject(resourceObject, resourceControl)
	}
	return resourceObject
}

func canUserAccessResource(userID api.UserID, userTeamIDs []api.TeamID, resourceControl *api.ResourceControl) bool {
	for _, authorizedUserAccess := range resourceControl.UserAccesses {
		if userID == authorizedUserAccess.UserID {
			return true
		}
	}

	for _, authorizedTeamAccess := range resourceControl.TeamAccesses {
		for _, userTeamID := range userTeamIDs {
			if userTeamID == authorizedTeamAccess.TeamID {
				return true
			}
		}
	}

	return false
}

func decorateObject(object map[string]interface{}, resourceControl *api.ResourceControl) map[string]interface{} {
	metadata := make(map[string]interface{})
	metadata["ResourceControl"] = resourceControl
	object["Portainer"] = metadata
	return object
}

func getResourceControlByResourceID(resourceID string, resourceControls []api.ResourceControl) *api.ResourceControl {
	for _, resourceControl := range resourceControls {
		if resourceID == resourceControl.ResourceID {
			return &resourceControl
		}
		for _, subResourceID := range resourceControl.SubResourceIDs {
			if resourceID == subResourceID {
				return &resourceControl
			}
		}
	}
	return nil
}

// CanAccessStack checks if a user can access a stack
func CanAccessStack(stack *api.Stack, resourceControl *api.ResourceControl, userID api.UserID, memberships []api.TeamMembership) bool {
	userTeamIDs := make([]api.TeamID, 0)
	for _, membership := range memberships {
		userTeamIDs = append(userTeamIDs, membership.TeamID)
	}

	if canUserAccessResource(userID, userTeamIDs, resourceControl) {
		return true
	}

	return false
}

// FilterStacks filters stacks based on user role and resource controls.
func FilterStacks(stacks []api.Stack, resourceControls []api.ResourceControl, isAdmin bool,
	userID api.UserID, memberships []api.TeamMembership) []ExtendedStack {

	filteredStacks := make([]ExtendedStack, 0)

	userTeamIDs := make([]api.TeamID, 0)
	for _, membership := range memberships {
		userTeamIDs = append(userTeamIDs, membership.TeamID)
	}

	for _, stack := range stacks {
		extendedStack := ExtendedStack{stack, api.ResourceControl{}}
		resourceControl := getResourceControlByResourceID(stack.Name, resourceControls)
		if resourceControl == nil {
			filteredStacks = append(filteredStacks, extendedStack)
		} else if resourceControl != nil && (isAdmin || canUserAccessResource(userID, userTeamIDs, resourceControl)) {
			extendedStack.ResourceControl = *resourceControl
			filteredStacks = append(filteredStacks, extendedStack)
		}
	}

	return filteredStacks
}
