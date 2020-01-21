package provider

import (
	"fmt"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/mrparkers/terraform-provider-keycloak/keycloak"
	"log"
	"strings"
)

func resourceKeycloakUserRoles() *schema.Resource {
	return &schema.Resource{
		Create: resourceKeycloakUserRolesCreate,
		Read:   resourceKeycloakUserRolesRead,
		Update: resourceKeycloakUserRolesUpdate,
		Delete: resourceKeycloakUserRolesDelete,
		// This resource can be imported using {{realm}}/{{userId}}.
		Importer: &schema.ResourceImporter{
			State: resourceKeycloakUserRolesImport,
		},
		Schema: map[string]*schema.Schema{
			"realm_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"user_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"role_ids": {
				Type:     schema.TypeSet,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
				Required: true,
			},
		},
	}
}

func userRolesId(realmId, userId string) string {
	return fmt.Sprintf("%s/%s", realmId, userId)
}

// given a user and a map of roles we already know about, fetch the roles we don't know about
// `localRoles` is used as a cache to avoid unnecessary http requests
func getMapOfRealmAndClientRolesFromUser(keycloakClient *keycloak.KeycloakClient, user *keycloak.User, localRoles map[string][]*keycloak.Role) (map[string][]*keycloak.Role, error) {
	roles := make(map[string][]*keycloak.Role)

	// realm roles
	if len(user.RealmRoles) != 0 {
		var realmRoles []*keycloak.Role

		for _, realmRoleName := range user.RealmRoles {
			found := false

			for _, localRealmRole := range localRoles["realm"] {
				if localRealmRole.Name == realmRoleName {
					found = true
					realmRoles = append(realmRoles, localRealmRole)

					break
				}
			}

			if !found {
				realmRole, err := keycloakClient.GetRoleByName(user.RealmId, "", realmRoleName)
				if err != nil {
					return nil, err
				}

				realmRoles = append(realmRoles, realmRole)
			}
		}

		roles["realm"] = realmRoles
	}

	// client roles
	if len(user.ClientRoles) != 0 {
		for clientName, clientRoleNames := range user.ClientRoles {
			client, err := keycloakClient.GetGenericClientByClientId(user.RealmId, clientName)
			if err != nil {
				return nil, err
			}

			var clientRoles []*keycloak.Role
			for _, clientRoleName := range clientRoleNames {
				found := false

				for _, localClientRole := range localRoles[client.Id] {
					if localClientRole.Name == clientRoleName {
						found = true
						clientRoles = append(clientRoles, localClientRole)

						break
					}
				}

				if !found {
					clientRole, err := keycloakClient.GetRoleByName(user.RealmId, client.Id, clientRoleName)
					if err != nil {
						return nil, err
					}

					clientRoles = append(clientRoles, clientRole)
				}
			}

			roles[client.Id] = clientRoles
		}
	}

	return roles, nil
}

func addRolesToUser(keycloakClient *keycloak.KeycloakClient, rolesToAdd map[string][]*keycloak.Role, user *keycloak.User) error {
	if realmRoles, ok := rolesToAdd["realm"]; ok && len(realmRoles) != 0 {
		err := keycloakClient.AddRealmRolesToUser(user.RealmId, user.Id, realmRoles)
		if err != nil {
			return err
		}
	}

	for k, roles := range rolesToAdd {
		if k == "realm" {
			continue
		}

		err := keycloakClient.AddClientRolesToUser(user.RealmId, user.Id, k, roles)
		if err != nil {
			return err
		}
	}

	return nil
}

func removeRolesFromUser(keycloakClient *keycloak.KeycloakClient, rolesToRemove map[string][]*keycloak.Role, user *keycloak.User) error {
	if realmRoles, ok := rolesToRemove["realm"]; ok && len(realmRoles) != 0 {
		err := keycloakClient.RemoveRealmRolesFromUser(user.RealmId, user.Id, realmRoles)
		if err != nil {
			return err
		}
	}

	for k, roles := range rolesToRemove {
		if k == "realm" {
			continue
		}

		err := keycloakClient.RemoveClientRolesFromUser(user.RealmId, user.Id, k, roles)
		if err != nil {
			return err
		}
	}

	return nil
}

func resourceKeycloakUserRolesCreate(data *schema.ResourceData, meta interface{}) error {
	keycloakClient := meta.(*keycloak.KeycloakClient)

	realmId := data.Get("realm_id").(string)
	userId := data.Get("user_id").(string)

	user, err := keycloakClient.GetUser(realmId, userId)
	if err != nil {
		return err
	}

	roleIds := interfaceSliceToStringSlice(data.Get("role_ids").(*schema.Set).List())
	rolesToAdd, err := getMapOfRealmAndClientRoles(keycloakClient, realmId, roleIds)
	if err != nil {
		return err
	}

	err = addRolesToUser(keycloakClient, rolesToAdd, user)
	if err != nil {
		return err
	}

	data.SetId(userRolesId(realmId, userId))

	return resourceKeycloakUserRolesRead(data, meta)
}

func resourceKeycloakUserRolesRead(data *schema.ResourceData, meta interface{}) error {
	keycloakClient := meta.(*keycloak.KeycloakClient)

	realmId := data.Get("realm_id").(string)
	userId := data.Get("user_id").(string)

	user, err := keycloakClient.GetUser(realmId, userId)
	if err != nil {
		return err
	}

	var roleIds []string

	if len(user.RealmRoles) != 0 {
		for _, realmRole := range user.RealmRoles {
			role, err := keycloakClient.GetRoleByName(realmId, "", realmRole)
			if err != nil {
				return err
			}

			roleIds = append(roleIds, role.Id)
		}
	}

	if len(user.ClientRoles) != 0 {
		for clientName, clientRoles := range user.ClientRoles {
			client, err := keycloakClient.GetGenericClientByClientId(realmId, clientName)
			if err != nil {
				return err
			}

			for _, clientRole := range clientRoles {
				role, err := keycloakClient.GetRoleByName(realmId, client.Id, clientRole)
				if err != nil {
					return err
				}

				roleIds = append(roleIds, role.Id)
			}
		}
	}

	data.Set("role_ids", roleIds)
	data.SetId(userRolesId(realmId, userId))

	return nil
}

func resourceKeycloakUserRolesUpdate(data *schema.ResourceData, meta interface{}) error {
	keycloakClient := meta.(*keycloak.KeycloakClient)

	realmId := data.Get("realm_id").(string)
	userId := data.Get("user_id").(string)

	user, err := keycloakClient.GetUser(realmId, userId)
	if err != nil {
		return err
	}

	roleIds := interfaceSliceToStringSlice(data.Get("role_ids").(*schema.Set).List())

	tfRoles, err := getMapOfRealmAndClientRoles(keycloakClient, realmId, roleIds)
	log.Printf("tfRoles length: %d", len(tfRoles))
	if err != nil {
		return err
	}

	remoteRoles, err := getMapOfRealmAndClientRolesFromUser(keycloakClient, user, tfRoles)
	if err != nil {
		return err
	}
	for key := range tfRoles {
		log.Printf("tfRoles: %s\n", key)
	}
	removeDuplicateRoles(&tfRoles, &remoteRoles)

	// `tfRoles` contains all roles that need to be added
	// `remoteRoles` contains all roles that need to be removed

	err = addRolesToUser(keycloakClient, tfRoles, user)
	if err != nil {
		return err
	}

	err = removeRolesFromUser(keycloakClient, remoteRoles, user)
	if err != nil {
		return err
	}

	return nil
}

func resourceKeycloakUserRolesDelete(data *schema.ResourceData, meta interface{}) error {
	keycloakClient := meta.(*keycloak.KeycloakClient)

	realmId := data.Get("realm_id").(string)
	userId := data.Get("user_id").(string)

	user, err := keycloakClient.GetUser(realmId, userId)

	roleIds := interfaceSliceToStringSlice(data.Get("role_ids").(*schema.Set).List())
	rolesToRemove, err := getMapOfRealmAndClientRoles(keycloakClient, realmId, roleIds)

	if err != nil {
		return err
	}

	err = removeRolesFromUser(keycloakClient, rolesToRemove, user)
	if err != nil {
		return err
	}

	return nil
}

func resourceKeycloakUserRolesImport(d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	parts := strings.Split(d.Id(), "/")

	if len(parts) != 2 {
		return nil, fmt.Errorf("Invalid import. Supported import format: {{realm}}/{{userId}}.")
	}

	d.Set("realm_id", parts[0])
	d.Set("user_id", parts[1])

	d.SetId(userRolesId(parts[0], parts[1]))

	return []*schema.ResourceData{d}, nil
}

// func removeRoleFromSlice(slice []*keycloak.Role, index int) []*keycloak.Role {
// 	slice[index] = slice[len(slice)-1]
// 	return slice[:len(slice)-1]
// }
//
// func removeDuplicateRoles(one, two *map[string][]*keycloak.Role) {
// 	for k := range *one {
// 		for i1 := 0; i1 < len((*one)[k]); i1++ {
// 			s1 := (*one)[k][i1]
//
// 			for i2 := 0; i2 < len((*two)[k]); i2++ {
// 				s2 := (*two)[k][i2]
//
// 				if s1.Id == s2.Id {
// 					(*one)[k] = removeRoleFromSlice((*one)[k], i1)
// 					(*two)[k] = removeRoleFromSlice((*two)[k], i2)
//
// 					i1--
// 					break
// 				}
// 			}
// 		}
// 	}
// }
