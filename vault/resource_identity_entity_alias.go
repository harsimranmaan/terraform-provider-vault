package vault

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/vault/api"
)

const identityEntityAliasPath = "/identity/entity-alias"

func identityEntityAliasResource() *schema.Resource {
	return &schema.Resource{
		Create: identityEntityAliasCreate,
		Update: identityEntityAliasUpdate,
		Read:   identityEntityAliasRead,
		Delete: identityEntityAliasDelete,
		Exists: identityEntityAliasExists,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the entity alias.",
			},

			"mount_accessor": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Mount accessor to which this alias belongs toMount accessor to which this alias belongs to.",
			},

			"canonical_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "ID of the entity to which this is an alias.",
			},
			"custom_metadata": {
				Type:        schema.TypeMap,
				Optional:    true,
				Description: "Custom metadata to be associated with this alias.",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func identityEntityAliasCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	name := d.Get("name").(string)
	mountAccessor := d.Get("mount_accessor").(string)
	canonicalID := d.Get("canonical_id").(string)
	customMetadata := d.Get("custom_metadata").(map[string]interface{})

	path := identityEntityAliasPath

	data := map[string]interface{}{
		"name":            name,
		"mount_accessor":  mountAccessor,
		"canonical_id":    canonicalID,
		"custom_metadata": customMetadata,
	}

	resp, err := client.Logical().Write(path, data)

	if err != nil {
		return fmt.Errorf("error writing IdentityEntityAlias to %q: %s", name, err)
	}

	if resp == nil {
		aliasIDMsg := "Unable to determine alias id."

		if aliasID, err := findAliasID(client, canonicalID, name, mountAccessor); err == nil {
			aliasIDMsg = fmt.Sprintf("Alias resource ID %q may be imported.", aliasID)
		}

		return fmt.Errorf("IdentityEntityAlias %q already exists. %s", name, aliasIDMsg)
	}

	log.Printf("[DEBUG] Wrote IdentityEntityAlias %q", name)

	d.SetId(resp.Data["id"].(string))

	return identityEntityAliasRead(d, meta)
}

func identityEntityAliasUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)
	id := d.Id()

	log.Printf("[DEBUG] Updating IdentityEntityAlias %q", id)
	path := identityEntityAliasIDPath(id)

	resp, err := client.Logical().Read(path)
	if err != nil {
		return fmt.Errorf("error updating IdentityEntityAlias %q: %s", id, err)
	}

	data := map[string]interface{}{
		"name":           resp.Data["name"],
		"mount_accessor": resp.Data["mount_accessor"],
		"canonical_id":   resp.Data["canonical_id"],
	}

	if name, ok := d.GetOk("name"); ok {
		data["name"] = name
	}
	if mountAccessor, ok := d.GetOk("mount_accessor"); ok {
		data["mount_accessor"] = mountAccessor
	}
	if canonicalID, ok := d.GetOk("canonical_id"); ok {
		data["canonical_id"] = canonicalID
	}

	data["custom_metadata"] = d.Get("custom_metadata").(map[string]interface{})

	_, err = client.Logical().Write(path, data)

	if err != nil {
		return fmt.Errorf("error updating IdentityEntityAlias %q: %s", id, err)
	}
	log.Printf("[DEBUG] Updated IdentityEntityAlias %q", id)

	return identityEntityAliasRead(d, meta)
}

func identityEntityAliasRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)
	id := d.Id()

	path := identityEntityAliasIDPath(id)

	log.Printf("[DEBUG] Reading IdentityEntityAlias %s from %q", id, path)
	resp, err := client.Logical().Read(path)
	if err != nil {
		return fmt.Errorf("error reading IdentityEntityAlias %q: %s", id, err)
	}
	log.Printf("[DEBUG] Read IdentityEntityAlias %s", id)
	if resp == nil {
		log.Printf("[WARN] IdentityEntityAlias %q not found, removing from state", id)
		d.SetId("")
		return nil
	}

	d.SetId(resp.Data["id"].(string))
	for _, k := range []string{"name", "mount_accessor", "canonical_id", "custom_metadata"} {
		if err := d.Set(k, resp.Data[k]); err != nil {
			return fmt.Errorf("error setting state key %q on IdentityEntityAlias %q:  err=%q", k, id, err)
		}
	}
	return nil
}

func identityEntityAliasDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)
	id := d.Id()

	path := identityEntityAliasIDPath(id)

	log.Printf("[DEBUG] Deleting IdentityEntityAlias %q", id)
	_, err := client.Logical().Delete(path)
	if err != nil {
		return fmt.Errorf("error IdentityEntityAlias %q", id)
	}
	log.Printf("[DEBUG] Deleted IdentityEntityAlias %q", id)

	return nil
}

func identityEntityAliasExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	client := meta.(*api.Client)
	id := d.Id()

	path := identityEntityAliasIDPath(id)
	key := id

	// use the name if no ID is set
	if len(id) == 0 {
		key = d.Get("name").(string)
		path = identityEntityAliasNamePath(key)
	}

	log.Printf("[DEBUG] Checking if IdentityEntityAlias %q exists", key)
	resp, err := client.Logical().Read(path)
	if err != nil {
		return true, fmt.Errorf("error checking if IdentityEntityAlias %q exists: %s", key, err)
	}
	log.Printf("[DEBUG] Checked if IdentityEntityAlias %q exists", key)

	return resp != nil, nil
}

func identityEntityAliasNamePath(name string) string {
	return fmt.Sprintf("%s/name/%s", identityEntityAliasPath, name)
}

func identityEntityAliasIDPath(id string) string {
	return fmt.Sprintf("%s/id/%s", identityEntityAliasPath, id)
}

func findAliasID(client *api.Client, canonicalID, name, mountAccessor string) (string, error) {
	path := identityEntityIDPath(canonicalID)

	resp, err := client.Logical().Read(path)
	if err != nil {
		return "", fmt.Errorf("error reading entity aliases: %s", err)
	}

	if resp != nil {
		aliases := resp.Data["aliases"].([]interface{})
		for _, aliasRaw := range aliases {
			alias := aliasRaw.(map[string]interface{})
			if alias["name"] == name && alias["mount_accessor"] == mountAccessor {
				return alias["id"].(string), nil
			}
		}
	}

	return "", fmt.Errorf("unable to determine alias ID. canonical ID: %q  name: %q  mountAccessor: %q", canonicalID, name, mountAccessor)
}
