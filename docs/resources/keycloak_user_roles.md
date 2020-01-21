# keycloak_user_roles

Allows you to manage roles assigned to a Keycloak user.

Note that this resource attempts to be an **authoritative** source over
user roles. When this resource takes control over a user's roles,
roles that are manually added to the user will be removed, and roles
that are manually removed from the user will be added upon the next run
of `terraform apply`.

Note that when assigning composite roles to a user, you may see a
non-empty plan following a `terraform apply` if you assign a role and a
composite that includes that role to the same user.

### Example Usage

```hcl
resource "keycloak_realm" "realm" {
  realm   = "my-realm"
  enabled = true
}

resource "keycloak_role" "realm_role" {
  realm_id    = "${keycloak_realm.realm.id}"
  name        = "my-realm-role"
  description = "My Realm Role"
}

resource "keycloak_openid_client" "client" {
  realm_id  = "${keycloak_realm.realm.id}"
  client_id = "client"
  name      = "client"

  enabled = true

  access_type = "BEARER-ONLY"
}

resource "keycloak_role" "client_role" {
  realm_id    = "${keycloak_realm.realm.id}"
  client_id   = "${keycloak_client.client.id}"
  name        = "my-client-role"
  description = "My Client Role"
}

resource "keycloak_user" "user" {
  realm_id = "${keycloak_realm.realm.id}"
  username     = "my-user"
}

resource "keycloak_user_roles" "user_roles" {
  realm_id = "${keycloak_realm.realm.id}"
  user_id = "${keycloak_user.user.id}"

  role_ids = [
    "${keycloak_role.realm_role.id}",
    "${keycloak_role.client_role.id}",
  ]
}
```

### Argument Reference

The following arguments are supported:

- `realm_id` - (Required) The realm this user exists in.
- `user_id` - (Required) The ID of the user this resource should
  manage roles for.
- `role_ids` - (Required) A list of role IDs to map to the user

### Import

This resource can be imported using the format
`{{realm_id}}/{{user_id}}`, where `user_id` is the unique ID that
Keycloak assigns to the user upon creation. This value can be found in
the URI when editing this user in the GUI, and is typically a GUID.

Example:

```bash
$ terraform import keycloak_user_roles.user_roles my-realm/18cc6b87-2ce7-4e59-bdc8-b9d49ec98a94
```
