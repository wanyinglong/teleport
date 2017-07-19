package ui

import "github.com/gravitational/teleport/lib/services"

type userContext struct {
	// Name is this user name
	Name string `json:"userName"`
	// Email is this user email
	Email string `json:"userEmail"`
	// ACL is this user access control list
	ACL RoleAccess `json:"userAcl"`
}

// NewUserContext return userContext
func NewUserContext(user services.User, allRoles []services.Role) userContext {
	userRoles := user.GetRoles()
	roleNamesMap := map[string]bool{}
	for _, name := range userRoles {
		roleNamesMap[name] = true
	}

	accessSet := []RoleAccess{}
	for _, item := range allRoles {
		if roleNamesMap[item.GetName()] {
			uiRole := NewRole(item)
			accessSet = append(accessSet, uiRole.Access)
		}
	}

	userACL := MergeAccessSet(accessSet)
	return userContext{
		Name: user.GetName(),
		ACL:  userACL,
	}
}
