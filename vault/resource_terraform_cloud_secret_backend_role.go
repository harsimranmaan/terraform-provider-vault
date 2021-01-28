package vault

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/vault/api"
)

var (
	terraformCloudSecretBackendRoleBackendFromPathRegex = regexp.MustCompile("^(.+)/role/.+$")
	terraformCloudSecretBackendRoleNameFromPathRegex    = regexp.MustCompile("^.+/role/(.+$)")
)

func terraformCloudSecretBackendRoleResource() *schema.Resource {
	return &schema.Resource{
		Create: terraformCloudSecretBackendRoleWrite,
		Read:   terraformCloudSecretBackendRoleRead,
		Update: terraformCloudSecretBackendRoleWrite,
		Delete: terraformCloudSecretBackendRoleDelete,
		Exists: terraformCloudSecretBackendRoleExists,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of an existing role against which to create this Terraform Cloud credential",
			},
			"path": {
				Type:          schema.TypeString,
				Optional:      true,
				Description:   "The path of the Terraform Cloud Secret Backend the role belongs to.",
				Deprecated:    "use `backend` instead",
				ConflictsWith: []string{"backend"},
			},
			"backend": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				Description:   "The path of the Terraform Cloud Secret Backend the role belongs to.",
				ConflictsWith: []string{"path"},
			},
			"organization": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Terraform Cloud or Enterprise organization",
			},
			"team_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "ID of the Terraform Cloud or Enterprise team under organization (e.g., settings/teams/team-xxxxxxxxxxxxx)",
			},
			"user_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "ID of the Terraform Cloud or Enterprise user (e.g., user-xxxxxxxxxxxxxxxx)",
			},
			"max_ttl": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Default lease for generated credentials. If not set or set to 0, will use system default.",
				Default:     0,
			},
			"ttl": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Default lease for generated credentials. If not set or set to 0, will use system default.",
				Default:     0,
			},
		},
	}
}

func terraformCloudSecretBackendRoleGetBackend(d *schema.ResourceData) string {
	if v, ok := d.GetOk("backend"); ok {
		return v.(string)
	} else if v, ok := d.GetOk("path"); ok {
		return v.(string)
	} else {
		return ""
	}
}

func terraformCloudSecretBackendRoleWrite(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	name := d.Get("name").(string)

	backend := terraformCloudSecretBackendRoleGetBackend(d)
	if backend == "" {
		return fmt.Errorf("No backend specified for Terraform Cloud secret backend role %s", name)
	}

	path := terraformCloudSecretBackendRolePath(backend, name)

	payload := map[string]interface{}{}

	if v, ok := d.GetOkExists("max_ttl"); ok {
		payload["max_ttl"] = v
	}
	if v, ok := d.GetOkExists("ttl"); ok {
		payload["ttl"] = v
	}
	if v, ok := d.GetOkExists("organization"); ok {
		payload["organization"] = v
	}
	if v, ok := d.GetOkExists("team_id"); ok {
		payload["team_id"] = v
	}
	if v, ok := d.GetOkExists("user_id"); ok {
		payload["user_id"] = v
	}

	log.Printf("[DEBUG] Configuring Terraform Cloud secrets backend role at %q", path)

	if _, err := client.Logical().Write(path, payload); err != nil {
		return fmt.Errorf("error writing role configuration for %q: %s", path, err)
	}

	d.SetId(path)
	return terraformCloudSecretBackendRoleRead(d, meta)
}

func terraformCloudSecretBackendRoleRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	path := d.Id()
	name, err := terraformCloudSecretBackendRoleNameFromPath(path)
	if err != nil {
		log.Printf("[WARN] Removing terraform cloud role %q because its ID is invalid", path)
		d.SetId("")
		return fmt.Errorf("invalid role ID %q: %s", path, err)
	}

	backend, err := terraformCloudSecretBackendRoleBackendFromPath(path)
	if err != nil {
		log.Printf("[WARN] Removing terraform cloud role %q because its ID is invalid", path)
		d.SetId("")
		return fmt.Errorf("invalid role ID %q: %s", path, err)
	}

	log.Printf("[DEBUG] Reading Terraform Cloud secrets backend role at %q", path)

	secret, err := client.Logical().Read(path)
	if err != nil {
		return fmt.Errorf("error reading role configuration for %q: %s", path, err)
	}

	if secret == nil {
		return fmt.Errorf("resource not found")
	}

	data := secret.Data
	d.Set("name", name)
	if _, ok := d.GetOk("path"); ok {
		d.Set("path", backend)
	} else {
		d.Set("backend", backend)
	}
	d.Set("organization", data["organization"])
	d.Set("team_id", data["team_id"])
	d.Set("user_id", data["user_id"])
	d.Set("max_ttl", data["max_ttl"])
	d.Set("ttl", data["ttl"])

	return nil
}

func terraformCloudSecretBackendRoleDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*api.Client)

	path := d.Id()

	log.Printf("[DEBUG] Deleting Terraform Cloud backend role at %q", path)

	if _, err := client.Logical().Delete(path); err != nil {
		return fmt.Errorf("error deleting Terraform Cloud backend role at %q: %s", path, err)
	}
	log.Printf("[DEBUG] Deleted Terraform Cloud backend role at %q", path)
	return nil
}

func terraformCloudSecretBackendRoleExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	client := meta.(*api.Client)

	path := d.Id()

	log.Printf("[DEBUG] Checking Terraform Cloud secrets backend role at %q", path)

	secret, err := client.Logical().Read(path)
	if err != nil {
		return false, fmt.Errorf("error reading role configuration for %q: %s", path, err)
	}

	return secret != nil, nil
}

func terraformCloudSecretBackendRolePath(backend, name string) string {
	return strings.Trim(backend, "/") + "/role/" + name
}

func terraformCloudSecretBackendRoleNameFromPath(path string) (string, error) {
	if !terraformCloudSecretBackendRoleNameFromPathRegex.MatchString(path) {
		return "", fmt.Errorf("no name found")
	}
	res := terraformCloudSecretBackendRoleNameFromPathRegex.FindStringSubmatch(path)
	if len(res) != 2 {
		return "", fmt.Errorf("unexpected number of matches (%d) for name", len(res))
	}
	return res[1], nil
}

func terraformCloudSecretBackendRoleBackendFromPath(path string) (string, error) {
	if !terraformCloudSecretBackendRoleBackendFromPathRegex.MatchString(path) {
		return "", fmt.Errorf("no backend found")
	}
	res := terraformCloudSecretBackendRoleBackendFromPathRegex.FindStringSubmatch(path)
	if len(res) != 2 {
		return "", fmt.Errorf("unexpected number of matches (%d) for backend", len(res))
	}
	return res[1], nil
}
