package gridscale

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gridscale/gsclient-go/v3"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	errHandler "github.com/terraform-providers/terraform-provider-gridscale/gridscale/error-handler"

	"log"
)

const (
	k8sTemplateFlavourName         = "kubernetes"
	k8sLabelPrefix                 = "#gsk#"
	k8sRocketStorageSupportRelease = "1.26"
	k8sMultiNodePoolSupportRelease = "1.30"
)

// ResourceGridscaleK8sModeler struct represents a modeler of the gridscale k8s resource.
type ResourceGridscaleK8sModeler struct{}

// ResourceGridscaleK8sValidator struct represents a validator of the gridscale k8s resource.
type ResourceGridscaleK8sValidator struct{}

func resourceGridscaleK8s() *schema.Resource {
	var resourceModeler ResourceGridscaleK8sModeler
	return &schema.Resource{
		Create: resourceGridscaleK8sCreate,
		Read:   resourceGridscaleK8sRead,
		Delete: resourceGridscaleK8sDelete,
		Update: resourceGridscaleK8sUpdate,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		CustomizeDiff: func(ctx context.Context, d *schema.ResourceDiff, meta interface{}) error {
			template, err := deriveK8sTemplateFromResourceDiff(meta.(*gsclient.Client), d)

			if err != nil {
				return err
			}
			return validateK8sParameters(d, *template)
		},
		Schema: resourceModeler.buildInputSchema(),
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(45 * time.Minute),
			Update: schema.DefaultTimeout(45 * time.Minute),
			Delete: schema.DefaultTimeout(45 * time.Minute),
		},
	}
}

/*
checkIfTemplateSupportsMultiNodePools checks by ResourceGridscaleK8sValidator receiver
if given template of instance gsclient.PaaSTemplate is compatible with multi node pools support.
*/
func (rgk8sv *ResourceGridscaleK8sValidator) checkIfTemplateSupportsMultiNodePools(template gsclient.PaaSTemplate) error {
	errorPrefix := fmt.Sprintf(
		"check if k8s template (%v) release (%v) is compatible with multi node support release (%v)",
		template.Properties.ObjectUUID,
		template.Properties.Release,
		k8sMultiNodePoolSupportRelease,
	)
	templateRelease, err := NewRelease(template.Properties.Release)
	if err != nil {
		return fmt.Errorf("%s error: %v", errorPrefix, err)
	}
	return templateRelease.CheckIfK8SReleaseIsSupported()
}

// buildInputSchema models the k8s resource's input schema by ResourceGridscaleK8sModeler receiver.
func (rgk8sm *ResourceGridscaleK8sModeler) buildInputSchema() map[string]*schema.Schema {
	nodePoolSchema := map[string]*schema.Schema{
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "Name of node pool.",
		},
		"node_count": {
			Type:        schema.TypeInt,
			Required:    true,
			Description: "Number of worker nodes.",
		},
		"cores": {
			Type:        schema.TypeInt,
			Required:    true,
			Description: "Cores per worker node.",
		},
		"memory": {
			Type:        schema.TypeInt,
			Required:    true,
			Description: "Memory per worker node (in GiB).",
		},
		"storage": {
			Type:        schema.TypeInt,
			Required:    true,
			Description: "Storage per worker node (in GiB).",
		},
		"storage_type": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "Storage type.",
		},
		"rocket_storage": {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     0,
			Description: "Rocket storage per worker node (in GiB).",
		},
	}
	return map[string]*schema.Schema{
		"name": {
			Type:         schema.TypeString,
			Description:  "The human-readable name of the object. It supports the full UTF-8 character set, with a maximum of 64 characters.",
			Required:     true,
			ValidateFunc: validation.NoZeroValues,
		},
		"kubeconfig": {
			Type:        schema.TypeString,
			Description: "K8s config data",
			Computed:    true,
			Sensitive:   true,
		},
		"listen_port": {
			Type:        schema.TypeSet,
			Description: "The port number where this k8s service accepts connections.",
			Computed:    true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"name": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"port": {
						Type:     schema.TypeInt,
						Computed: true,
					},
				},
			},
		},
		"security_zone_uuid": {
			Type:        schema.TypeString,
			Description: "Security zone UUID linked to PaaS service.",
			Deprecated:  "GSK service does not support security zone.",
			Optional:    true,
			ForceNew:    true,
			Computed:    true,
		},
		"network_uuid": {
			Type:        schema.TypeString,
			Description: "Network UUID containing security zone",
			Deprecated: `network_uuid of a security zone is no more available for GSK.
				Please consider to use k8s_private_network_uuid for connecting external services to the cluster.`,
			Computed: true,
		},
		"k8s_private_network_uuid": {
			Type:        schema.TypeString,
			Description: "Private network UUID which k8s nodes are attached to. It can be used to attach other PaaS/VMs.",
			Computed:    true,
		},
		"release": {
			Type:        schema.TypeString,
			Description: "The k8s release of this instance.",
			Optional:    true,
		},
		"gsk_version": {
			Type:        schema.TypeString,
			Description: "The gridscale k8s PaaS version (issued by gridscale) of this instance.",
			Optional:    true,
		},
		"service_template_uuid": {
			Type:        schema.TypeString,
			Description: "PaaS service template identifier for this service.",
			Computed:    true,
		},
		"node_pool": {
			Type:        schema.TypeList,
			Required:    true,
			Description: `Define a node pool and its attributes.`,
			Elem: &schema.Resource{
				Schema: nodePoolSchema,
			},
		},
		"surge_node": {
			Type:        schema.TypeBool,
			Description: "Enable surge node to avoid resources shortage during the cluster upgrade.",
			Optional:    true,
			Computed:    true,
		},
		"cluster_cidr": {
			Type:        schema.TypeString,
			Description: "The cluster CIDR that will be used to generate the CIDR of nodes, services, and pods. The allowed CIDR prefix length is /16. If this field is empty, the default value is \"10.244.0.0/16\"",
			Optional:    true,
			Computed:    true,
		},
		"cluster_traffic_encryption": {
			Type:        schema.TypeBool,
			Description: "Enables cluster encryption via wireguard if true. Only available for GSK version 1.29 and above. Default is false.",
			Optional:    true,
			Default:     false,
		},
		"usage_in_minutes": {
			Type:        schema.TypeInt,
			Description: "Number of minutes that PaaS service is in use",
			Computed:    true,
		},
		"change_time": {
			Type:        schema.TypeString,
			Description: "Time of the last change",
			Computed:    true,
		},
		"create_time": {
			Type:        schema.TypeString,
			Description: "Time this service was created.",
			Computed:    true,
		},
		"status": {
			Type:        schema.TypeString,
			Description: "Current status of the service",
			Computed:    true,
		},
		"labels": {
			Type:        schema.TypeSet,
			Description: "List of labels.",
			Optional:    true,
			Elem:        &schema.Schema{Type: schema.TypeString},
		},
		"oidc_enabled": {
			Type:        schema.TypeBool,
			Description: "Disable or enable OIDC",
			Computed:    true,
			Optional:    true,
		},
		"oidc_issuer_url": {
			Type:        schema.TypeString,
			Description: "URL of the provider that allows the API server to discover public signing keys. Only URLs that use the https:// scheme are accepted.",
			Computed:    true,
			Optional:    true,
		},
		"oidc_client_id": {
			Type:        schema.TypeString,
			Description: "A client ID that all tokens must be issued for.",
			Computed:    true,
			Optional:    true,
		},
		"oidc_username_claim": {
			Type:        schema.TypeString,
			Description: "JWT claim to use as the user name.",
			Computed:    true,
			Optional:    true,
		},
		"oidc_groups_claim": {
			Type:        schema.TypeString,
			Description: "JWT claim to use as the user's group.",
			Computed:    true,
			Optional:    true,
		},
		"oidc_signing_algs": {
			Type:        schema.TypeString,
			Description: "The signing algorithms accepted. Default is 'RS256'. Other option is 'RS512'.",
			Computed:    true,
			Optional:    true,
		},
		"oidc_groups_prefix": {
			Type:        schema.TypeString,
			Description: "Prefix prepended to group claims to prevent clashes with existing names (such as system: groups). For example, the value oidc: will create group names like oidc:engineering and oidc:infra.",
			Computed:    true,
			Optional:    true,
		},
		"oidc_username_prefix": {
			Type:        schema.TypeString,
			Description: "Prefix prepended to username claims to prevent clashes with existing names (such as system: users). For example, the value oidc: will create usernames like oidc:jane.doe. If this flag isn't provided and --oidc-username-claim is a value other than email the prefix defaults to ( Issuer URL )# where ( Issuer URL ) is the value of --oidc-issuer-url. The value - can be used to disable all prefixing.",
			Computed:    true,
			Optional:    true,
		},
		"oidc_required_claim": {
			Type:        schema.TypeString,
			Description: "A key=value pair that describes a required claim in the ID Token. Multiple claims can be set like this: key1=value1,key2=value2",
			Computed:    true,
			Optional:    true,
		},
		"oidc_ca_pem": {
			Type:        schema.TypeString,
			Description: "Custom CA from customer in pem format as string.",
			Computed:    true,
			Optional:    true,
		},
		"hubble": {
			Type:        schema.TypeBool,
			Description: "Enable hubble integration.",
			Computed:    true,
			Optional:    true,
		},
		"kube_apiserver_log_enabled": {
			Type:        schema.TypeBool,
			Description: "Enable kube-apiserver logs.",
			Computed:    true,
			Optional:    true,
		},
		"audit_log_enabled": {
			Type:        schema.TypeBool,
			Description: "Enable Kubernetes audit logs.",
			Computed:    true,
			Optional:    true,
		},
		"audit_log_level": {
			Type:        schema.TypeString,
			Description: "Audit log level.",
			Computed:    true,
			Optional:    true,
		},
		"log_delivery": {
			Type:        schema.TypeBool,
			Description: "Enable control plane log delivery.",
			Computed:    true,
			Optional:    true,
		},
		"log_delivery_bucket": {
			Type:        schema.TypeString,
			Description: "Bucket to upload logs to.",
			Computed:    true,
			Optional:    true,
		},
		"log_delivery_access_key": {
			Type:        schema.TypeString,
			Description: "Access key used to authenticate against Object Storage endpoint.",
			Computed:    true,
			Optional:    true,
		},
		"log_delivery_secret_key": {
			Type:        schema.TypeString,
			Description: "Secret key used to authenticate against Object Storage endpoint.",
			Computed:    true,
			Optional:    true,
		},
		"log_delivery_interval": {
			Type:        schema.TypeInt,
			Description: "Time interval (in min), at which log files will be delivered, unless file size limit is reached first.",
			Computed:    true,
			Optional:    true,
		},
		"log_delivery_endpoint": {
			Type:        schema.TypeString,
			Description: "Object Storage endpoint URL the bucket is located on.",
			Computed:    true,
			Optional:    true,
		},
		"k8s_hubble": {
			Type:        schema.TypeBool,
			Description: "Enables Hubble Integration.",
			Computed:    true,
			Optional:    true,
		},
	}
}

/*
deriveK8sTemplateFromResourceDiff derives the k8s service template
from given resource diff of instance *schema.ResourceDiff.
The derivation will base on respective resource's diff created by Terraform.
*/
func deriveK8sTemplateFromResourceDiff(client *gsclient.Client, d *schema.ResourceDiff) (*gsclient.PaaSTemplate, error) {
	derivationTypesRequested := 0
	derivationType := ""

	versionInterface, isVersionSet := d.GetOk("gsk_version")
	version := versionInterface.(string)
	releaseInterface, isReleaseSet := d.GetOk("release")
	release := releaseInterface.(string)

	if isVersionSet {
		derivationTypesRequested += 1
		derivationType = "version"
	}

	if isReleaseSet {
		derivationTypesRequested += 1
		derivationType = "release"
	}

	if derivationTypesRequested == 0 {
		return nil, errors.New("either \"release\" or \"gsk_version\" has to be defined")
	}
	if derivationTypesRequested > 1 {
		return nil, errors.New("\"release\" and \"gsk_version\" are not intended to be set at once.")
	}
	switch derivationType {
	case "version":
		return deriveK8sTemplateFromGSKVersion(client, version)
	case "release":
		currenTemplateUUID := d.Get("service_template_uuid").(string)
		return deriveK8sTemplateFromRelease(client, release, currenTemplateUUID)
	}
	return nil, nil
}

/*
deriveK8sTemplateFromResourceData derives the k8s service template
from given resource data of instance *schema.ResourceData.
The derivation will base on what gets requested.
*/
func deriveK8sTemplateFromResourceData(client *gsclient.Client, d *schema.ResourceData) (*gsclient.PaaSTemplate, error) {
	derivationTypesRequested := 0
	derivationType := ""

	versionInterface, isVersionSet := d.GetOk("gsk_version")
	version := versionInterface.(string)
	releaseInterface, isReleaseSet := d.GetOk("release")
	release := releaseInterface.(string)

	if !d.IsNewResource() { // case if update of resource is requested
		if isVersionSet && d.HasChange("gsk_version") {
			derivationTypesRequested += 1
			derivationType = "version"
		}

		if isReleaseSet && d.HasChange("release") {
			derivationTypesRequested += 1
			derivationType = "release"
		}
	} else { // case if creation of resource is requested
		if isVersionSet {
			derivationTypesRequested += 1
			derivationType = "version"
		}

		if isReleaseSet {
			derivationTypesRequested += 1
			derivationType = "release"
		}

		if derivationTypesRequested == 0 {
			return nil, errors.New("either \"release\" or \"gsk_version\" has to be defined")
		}
	}

	if derivationTypesRequested > 1 {
		return nil, errors.New("\"release\" and/or \"gsk_version\" are not intended to be set at once.")
	}
	switch derivationType {
	case "version":
		return deriveK8sTemplateFromGSKVersion(client, version)
	case "release":
		currenTemplateUUID := d.Get("service_template_uuid").(string)
		return deriveK8sTemplateFromRelease(client, release, currenTemplateUUID)
	}
	currentTemplateUUID := d.Get("service_template_uuid").(string)
	return deriveK8sTemplateFromUUID(client, currentTemplateUUID)
}

// deriveK8sTemplateFromUUID derives the k8s service template from given UUID.
func deriveK8sTemplateFromUUID(client *gsclient.Client, templateUUID string) (*gsclient.PaaSTemplate, error) {
	paasTemplates, err := client.GetPaaSTemplateList(context.Background())

	if err != nil {
		return nil, err
	}

	var derived bool
	var template gsclient.PaaSTemplate

	for _, paasTemplate := range paasTemplates {
		if paasTemplate.Properties.Flavour == k8sTemplateFlavourName {
			if paasTemplate.Properties.ObjectUUID == templateUUID {
				derived = true
				template = paasTemplate
				break
			}
		}
	}

	if !derived {
		return nil, fmt.Errorf("%v is an invalid gridscale Kubernetes (GSK) service template UUID", templateUUID)
	}
	return &template, nil
}

// deriveK8sTemplateFromGSKVersion derives the k8s service template from given GSK version.
func deriveK8sTemplateFromGSKVersion(client *gsclient.Client, version string) (*gsclient.PaaSTemplate, error) {
	paasTemplates, err := client.GetPaaSTemplateList(context.Background())

	if err != nil {
		return nil, err
	}

	var derived bool
	var isActive bool
	var versions []string
	var template gsclient.PaaSTemplate

	for _, paasTemplate := range paasTemplates {
		if paasTemplate.Properties.Flavour == k8sTemplateFlavourName {
			if paasTemplate.Properties.Active {
				versions = append(versions, template.Properties.Version)
			}

			if paasTemplate.Properties.Version == version {
				isActive = paasTemplate.Properties.Active
				derived = true
				template = paasTemplate
				break
			}
		}
	}

	if !derived {
		return nil, fmt.Errorf("%v is an invalid gridscale Kubernetes (GSK) version. Valid GSK versions are: %v", version, strings.Join(versions, ", "))
	}
	if !isActive {
		return nil, fmt.Errorf("%v is a deprecated gridscale Kubernetes (GSK) version. Valid GSK versions are: %v", version, strings.Join(versions, ", "))
	}
	return &template, nil
}

// deriveK8sTemplateFromRelease derives the k8s service template from given release.
func deriveK8sTemplateFromRelease(client *gsclient.Client, release, currenTemplateUUID string) (*gsclient.PaaSTemplate, error) {
	paasTemplates, err := client.GetPaaSTemplateList(context.Background())
	if err != nil {
		return nil, err
	}

	// Sort paasTemplates by version (ascending)
	sort.Slice(paasTemplates, func(i, j int) bool {
		return paasTemplates[i].Properties.Version < paasTemplates[j].Properties.Version
	})

	// if the current template's release is the same as the requested release, return the current template
	// this is to prevent this function return a different template than the current one when the release is not changed
	// E.g. Template of 1.30.4-gs0 and 1.30.5-gs0 have the same release 1.30. If a cluster is created with 1.30.4-gs0
	// via setting release="1.29", the function should return the current template 1.30.4-gs0, not 1.30.5-gs0.
	if currenTemplateUUID != "" {
		for _, paasTemplate := range paasTemplates {
			if paasTemplate.Properties.ObjectUUID == currenTemplateUUID &&
				paasTemplate.Properties.Release == release {
				return &paasTemplate, nil
			}
		}
	}

	var derived bool
	var releases []string
	var template gsclient.PaaSTemplate

	for _, paasTemplate := range paasTemplates {
		if paasTemplate.Properties.Flavour == k8sTemplateFlavourName && paasTemplate.Properties.Active {
			releases = append(releases, paasTemplate.Properties.Release)

			if paasTemplate.Properties.Release == release {
				derived = true
				template = paasTemplate
			}
		}
	}
	if !derived {
		return nil, fmt.Errorf("%v is an invalid Kubernetes release. Valid releases are: %v", release, strings.Join(releases, ", "))
	}

	return &template, nil
}

func resourceGridscaleK8sRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*gsclient.Client)
	errorPrefix := fmt.Sprintf("read k8s (%s) resource -", d.Id())
	paas, err := client.GetPaaSService(context.Background(), d.Id())

	if err != nil {
		if requestError, ok := err.(gsclient.RequestError); ok {
			if requestError.StatusCode == 404 {
				d.SetId("")
				return nil
			}
		}
		return fmt.Errorf("%s error: %v", errorPrefix, err)
	}
	props := paas.Properties
	creds := props.Credentials
	if err = d.Set("name", props.Name); err != nil {
		return fmt.Errorf("%s error setting name: %v", errorPrefix, err)
	}
	if len(creds) > 0 {
		// if expiration_time of kubeconfig is reached, renew it and get new kubeconfig
		if creds[0].ExpirationTime.Before(time.Now()) {
			err = client.RenewK8sCredentials(context.Background(), d.Id())
			if err != nil {
				return fmt.Errorf("%s error renewing k8s kubeconfig: %v", errorPrefix, err)
			}
			paas, err = client.GetPaaSService(context.Background(), d.Id())
			if err != nil {
				return fmt.Errorf("%s error: %v", errorPrefix, err)
			}
			props = paas.Properties
			creds = props.Credentials
		}
		if err = d.Set("kubeconfig", creds[0].KubeConfig); err != nil {
			return fmt.Errorf("%s error setting kubeconfig: %v", errorPrefix, err)
		}
	}
	template, err := deriveK8sTemplateFromUUID(client, props.ServiceTemplateUUID)
	if err != nil {
		return fmt.Errorf("%s error: %v", errorPrefix, err)
	}
	// if version is set, set it with the version of the template
	if _, isVersionSet := d.GetOk("gsk_version"); isVersionSet {
		if err = d.Set("gsk_version", template.Properties.Version); err != nil {
			return fmt.Errorf("%s error setting gsk_version: %v", errorPrefix, err)
		}
	}
	// if release is set, set it with the release of the template
	if _, isReleaseSet := d.GetOk("release"); isReleaseSet {
		if err = d.Set("release", template.Properties.Release); err != nil {
			return fmt.Errorf("%s error setting release: %v", errorPrefix, err)
		}
	}
	if err = d.Set("security_zone_uuid", props.SecurityZoneUUID); err != nil {
		return fmt.Errorf("%s error setting security_zone_uuid: %v", errorPrefix, err)
	}

	if err = d.Set("usage_in_minutes", props.UsageInMinutes); err != nil {
		return fmt.Errorf("%s error setting usage_in_minutes: %v", errorPrefix, err)
	}
	if err = d.Set("change_time", props.ChangeTime.String()); err != nil {
		return fmt.Errorf("%s error setting change_time: %v", errorPrefix, err)
	}
	if err = d.Set("create_time", props.CreateTime.String()); err != nil {
		return fmt.Errorf("%s error setting create_time: %v", errorPrefix, err)
	}
	if err = d.Set("status", props.Status); err != nil {
		return fmt.Errorf("%s error setting status: %v", errorPrefix, err)
	}
	if err = d.Set("service_template_uuid", props.ServiceTemplateUUID); err != nil {
		return fmt.Errorf("%s error setting service_template_uuid: %v", errorPrefix, err)
	}

	// Set flag telling if enabled or not
	if enabled, ok := props.Parameters["k8s_oidc_enabled"].(bool); ok {
		if err = d.Set("oidc_enabled", enabled); err != nil {
			return fmt.Errorf("%s error setting oidc_enabled: %v", errorPrefix, err)
		}
	}

	// Set issuer URL if it is set
	if issuerURL, isIssuerURLSet := props.Parameters["k8s_oidc_issuer_url"]; isIssuerURLSet {
		if err = d.Set("oidc_issuer_url", issuerURL); err != nil {
			return fmt.Errorf("%s error setting oidc_issuer_url: %v", errorPrefix, err)
		}
	}

	// Set client ID if it is set
	if clientID, isClientIDSet := props.Parameters["k8s_oidc_client_id"]; isClientIDSet {
		if err = d.Set("oidc_client_id", clientID); err != nil {
			return fmt.Errorf("%s error setting oidc_client_id: %v", errorPrefix, err)
		}
	}

	// Set username claim if it is set
	if usernameClaimSet, isUsernameClaimSet := props.Parameters["k8s_oidc_username_claim"]; isUsernameClaimSet {
		if err = d.Set("oidc_username_claim", usernameClaimSet); err != nil {
			return fmt.Errorf("%s error setting oidc_username_claim: %v", errorPrefix, err)
		}
	}

	// Set groups claim if it is set
	if groupsClain, isGroupsClaimSet := props.Parameters["k8s_oidc_groups_claim"]; isGroupsClaimSet {
		if err = d.Set("oidc_groups_claim", groupsClain); err != nil {
			return fmt.Errorf("%s error setting oidc_groups_claim: %v", errorPrefix, err)
		}
	}

	// Set signing algs if it is set
	if signingAlgs, isSigningAlgsSet := props.Parameters["k8s_oidc_signing_algs"]; isSigningAlgsSet {
		if err = d.Set("oidc_signing_algs", signingAlgs); err != nil {
			return fmt.Errorf("%s error setting oidc_signing_algs: %v", errorPrefix, err)
		}
	}

	// Set groups prefix if it is set
	if groupsPrefix, isGroupsPrefixSet := props.Parameters["k8s_oidc_groups_prefix"]; isGroupsPrefixSet {
		if err = d.Set("oidc_groups_prefix", groupsPrefix); err != nil {
			return fmt.Errorf("%s error setting oidc_groups_prefix: %v", errorPrefix, err)
		}
	}

	// Set username prefix if it is set
	if usernamePrefix, isUsernamePrefixSet := props.Parameters["k8s_oidc_username_prefix"]; isUsernamePrefixSet {
		if err = d.Set("oidc_username_prefix", usernamePrefix); err != nil {
			return fmt.Errorf("%s error setting oidc_username_prefix: %v", errorPrefix, err)
		}
	}

	// Set required claim if it is set
	if requiredClain, isRequiredClaimSet := props.Parameters["k8s_oidc_required_claim"]; isRequiredClaimSet {
		if err = d.Set("oidc_required_claim", requiredClain); err != nil {
			return fmt.Errorf("%s error setting oidc_required_claim: %v", errorPrefix, err)
		}
	}

	// Set CA PEM if it is set
	if caPEM, isCAPEMSet := props.Parameters["k8s_oidc_ca_pem"]; isCAPEMSet {
		if err = d.Set("oidc_ca_pem", caPEM); err != nil {
			return fmt.Errorf("%s error setting oidc_ca_pem: %v", errorPrefix, err)
		}
	}

	// Set hubble if it is set
	if hubble, isHubbleSet := props.Parameters["k8s_hubble"].(bool); isHubbleSet {
		if err = d.Set("hubble", hubble); err != nil {
			return fmt.Errorf("%s error setting hubble: %v", errorPrefix, err)
		}
	}

	// Set kube API server enabling if it is set
	if kubeAPIServerLogEnabled, isKubeAPIServerLogEnabledSet := props.Parameters["k8s_kube_apiserver_log_enabled"].(bool); isKubeAPIServerLogEnabledSet {
		if err = d.Set("kube_apiserver_log_enabled", kubeAPIServerLogEnabled); err != nil {
			return fmt.Errorf("%s error setting kube_apiserver_log_enabled: %v", errorPrefix, err)
		}
	}

	// Set audit log enabling if it is set
	if auditLogEnabled, isAuditLogEnabledSet := props.Parameters["k8s_audit_log_enabled"].(bool); isAuditLogEnabledSet {
		if err = d.Set("audit_log_enabled", auditLogEnabled); err != nil {
			return fmt.Errorf("%s error setting audit_log_enabled: %v", errorPrefix, err)
		}
	}

	// Set audit log level if it is set
	if auditLogLevel, isAuditLogLevelSet := props.Parameters["k8s_audit_log_level"]; isAuditLogLevelSet {
		if err = d.Set("audit_log_level", auditLogLevel); err != nil {
			return fmt.Errorf("%s error setting audit_log_level: %v", errorPrefix, err)
		}
	}

	// Set log delivery if it is set
	if logDelivery, isLogDeliverySet := props.Parameters["k8s_log_delivery"].(bool); isLogDeliverySet {
		if err = d.Set("log_delivery", logDelivery); err != nil {
			return fmt.Errorf("%s error setting log_delivery: %v", errorPrefix, err)
		}
	}

	// Set log delivery bucket if it is set
	if logDeliveryBucket, isLogDeliveryBucketSet := props.Parameters["k8s_log_delivery_bucket"]; isLogDeliveryBucketSet {
		if err = d.Set("log_delivery_bucket", logDeliveryBucket); err != nil {
			return fmt.Errorf("%s error setting log_delivery_bucket: %v", errorPrefix, err)
		}
	}

	// Set log delivery access key if it is set
	if logDeliveryAccessKey, isLogDeliveryAccessKeySet := props.Parameters["k8s_log_delivery_access_key"]; isLogDeliveryAccessKeySet {
		if err = d.Set("log_delivery_access_key", logDeliveryAccessKey); err != nil {
			return fmt.Errorf("%s error setting log_delivery_access_key: %v", errorPrefix, err)
		}
	}

	// Set log delivery secret key if it is set
	if logDeliverySecretKey, isLogDeliverySecretKeySet := props.Parameters["k8s_log_delivery_secret_key"]; isLogDeliverySecretKeySet {
		if err = d.Set("log_delivery_secret_key", logDeliverySecretKey); err != nil {
			return fmt.Errorf("%s error setting log_delivery_secret_key: %v", errorPrefix, err)
		}
	}

	// Set log delivery interval if it is set
	if logDeliveryInterval, isLogDeliveryIntervalSet := props.Parameters["k8s_log_delivery_interval"]; isLogDeliveryIntervalSet {
		if err = d.Set("log_delivery_interval", logDeliveryInterval); err != nil {
			return fmt.Errorf("%s error setting log_delivery_interval: %v", errorPrefix, err)
		}
	}

	// Set log delivery endpoint if it is set
	if logDeliveryEndpoint, isLogDeliveryEndpointSet := props.Parameters["k8s_log_delivery_endpoint"]; isLogDeliveryEndpointSet {
		if err = d.Set("log_delivery_endpoint", logDeliveryEndpoint); err != nil {
			return fmt.Errorf("%s error setting log_delivery_endpoint: %v", errorPrefix, err)
		}
	}

	// Set hubble if it is set
	if hubble, isHubbleSet := props.Parameters["k8s_hubble"].(bool); isHubbleSet {
		if err = d.Set("k8s_hubble", hubble); err != nil {
			return fmt.Errorf("%s error setting k8s_hubble: %v", errorPrefix, err)
		}
	}

	//Get listen ports
	listenPorts := make([]interface{}, 0)
	for _, value := range props.ListenPorts {
		for k, portValue := range value {
			port := map[string]interface{}{
				"name": k,
				"port": portValue,
			}
			listenPorts = append(listenPorts, port)
		}
	}
	if err = d.Set("listen_port", listenPorts); err != nil {
		return fmt.Errorf("%s error setting listen ports: %v", errorPrefix, err)
	}

	// Iterate over node pools fetched from source to collect them
	nodePoolsSetInterface, isNodePoolsSet := props.Parameters["pools"]
	nodePools := make([]map[string]interface{}, 0)

	if isNodePoolsSet {
		nodePoolsSet := nodePoolsSetInterface.([]interface{})

		for _, nodePoolSetInterface := range nodePoolsSet {
			nodePoolRead := make(map[string]interface{}, 0)
			nodePoolSet := nodePoolSetInterface.(map[string]interface{})

			if name, isNameSet := nodePoolSet["name"]; isNameSet {
				nodePoolRead["name"] = name
			}
			if nodeCount, isNodeCountSet := nodePoolSet["count"]; isNodeCountSet {
				nodePoolRead["node_count"] = nodeCount
			}
			if memory, isMemorySet := nodePoolSet["ram"]; isMemorySet {
				nodePoolRead["memory"] = memory
			}
			if cores, isCoresSet := nodePoolSet["cores"]; isCoresSet {
				nodePoolRead["cores"] = cores
			}
			if storage, isStorageSet := nodePoolSet["storage"]; isStorageSet {
				nodePoolRead["storage"] = storage
			}
			if storageType, isStorageTypeSet := nodePoolSet["storage_type"]; isStorageTypeSet {
				nodePoolRead["storage_type"] = storageType
			}
			if rocketStorage, isRocketStorageSet := nodePoolSet["rocket_storage"]; isRocketStorageSet {
				nodePoolRead["rocket_storage"] = rocketStorage
			}
			nodePools = append(nodePools, nodePoolRead)
		}
	}
	// Set node pools
	if err = d.Set("node_pool", nodePools); err != nil {
		return fmt.Errorf("%s error setting node_pool: %v", errorPrefix, err)
	}
	// Set cluster CIDR if it is set
	if clusterCIDR, isClusterCIDRSet := props.Parameters["k8s_cluster_cidr"]; isClusterCIDRSet {
		if err = d.Set("cluster_cidr", clusterCIDR); err != nil {
			return fmt.Errorf("%s error setting cluster_cidr: %v", errorPrefix, err)
		}
	}
	// Surge node feature is enable if k8s_surge_node_count > 0
	if surgeNodeCount, ok := props.Parameters["k8s_surge_node_count"].(float64); ok {
		d.Set("surge_node", surgeNodeCount > 0)
	}
	// Cluster traffic encryption feature is enabled if k8s_cluster_traffic_encryption is true
	if clusterTrafficEncryption, ok := props.Parameters["k8s_cluster_traffic_encryption"].(bool); ok {
		d.Set("cluster_traffic_encryption", clusterTrafficEncryption)
	}
	//Set labels
	if err = d.Set("labels", props.Labels); err != nil {
		return fmt.Errorf("%s error setting labels: %v", errorPrefix, err)
	}
	//Get all available networks
	networks, err := client.GetNetworkList(context.Background())
	if err != nil {
		return fmt.Errorf("%s error getting networks: %v", errorPrefix, err)
	}
	//look for a network that the PaaS service is in
	for _, network := range networks {
		securityZones := network.Properties.Relations.PaaSSecurityZones
		//Each network can contain only one security zone
		if len(securityZones) >= 1 {
			if securityZones[0].ObjectUUID == props.SecurityZoneUUID {
				if err = d.Set("network_uuid", network.Properties.ObjectUUID); err != nil {
					return fmt.Errorf("%s error setting network_uuid: %v", errorPrefix, err)
				}
			}
		}
	}

	k8sLabel := fmt.Sprintf("%s%s", k8sLabelPrefix, d.Id())
	// look for a network having the defined k8sLabel.
NETWORK_LOOOP:
	for _, network := range networks {
		for _, label := range network.Properties.Labels {
			if label == k8sLabel {
				if err = d.Set("k8s_private_network_uuid", network.Properties.ObjectUUID); err != nil {
					return fmt.Errorf("%s error setting k8s_private_network_uuid: %v", errorPrefix, err)
				}
				break NETWORK_LOOOP
			}
		}
	}
	return nil
}

func resourceGridscaleK8sCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*gsclient.Client)
	validator := &ResourceGridscaleK8sValidator{}
	errorPrefix := fmt.Sprintf("create k8s (%s) resource -", d.Id())
	template, err := deriveK8sTemplateFromResourceData(client, d)

	if err != nil {
		return fmt.Errorf("1: %s error: %v", errorPrefix, err)
	}
	err = validator.checkIfTemplateSupportsMultiNodePools(*template)

	if err != nil {
		return fmt.Errorf("2: %s error: %v", errorPrefix, err)
	}
	requestBody := gsclient.PaaSServiceCreateRequest{
		Name:                    d.Get("name").(string),
		PaaSServiceTemplateUUID: template.Properties.ObjectUUID,
		Labels:                  convSOStrings(d.Get("labels").(*schema.Set).List()),
		PaaSSecurityZoneUUID:    d.Get("security_zone_uuid").(string),
	}
	parameters := make(map[string]interface{})

	// Iterate over requested node pools to set them
	if nodePoolsRequestedInterface, ok := d.GetOk("node_pool"); ok {
		nodePoolsRequested := nodePoolsRequestedInterface.([]interface{})
		nodePools := make([]map[string]interface{}, 0)

		for index, _ := range nodePoolsRequested {
			nodePool := make(map[string]interface{}, 0)
			nodePool["name"] = d.Get(fmt.Sprintf("node_pool.%d.name", index))
			nodePool["count"] = d.Get(fmt.Sprintf("node_pool.%d.node_count", index))
			nodePool["ram"] = d.Get(fmt.Sprintf("node_pool.%d.memory", index))
			nodePool["cores"] = d.Get(fmt.Sprintf("node_pool.%d.cores", index))
			nodePool["storage"] = d.Get(fmt.Sprintf("node_pool.%d.storage", index))
			nodePool["storage_type"] = d.Get(fmt.Sprintf("node_pool.%d.storage_type", index))
			nodePool["rocket_storage"] = d.Get(fmt.Sprintf("node_pool.%d.rocket_storage", index))

			nodePools = append(nodePools, nodePool)
		}
		parameters["pools"] = nodePools
	}

	// Set cluster CIDR if it is set
	if clusterCIDR, isClusterCIDRSet := d.GetOk("cluster_cidr"); isClusterCIDRSet {
		parameters["k8s_cluster_cidr"] = clusterCIDR
	}
	// Set surge node count
	isSurgeNodeEnabled := d.Get("surge_node").(bool)
	if isSurgeNodeEnabled {
		parameters["k8s_surge_node_count"] = 1
	} else {
		parameters["k8s_surge_node_count"] = 0
	}
	// Set cluster traffic encryption if it is set
	if clusterTrafficEncryption, isSet := d.GetOk("cluster_traffic_encryption"); isSet {
		parameters["k8s_cluster_traffic_encryption"] = clusterTrafficEncryption
	}
	// Set OIDC enabled flag if it is set
	if oidcEnabled, isOIDCEnabledSet := d.GetOk("oidc_enabled"); isOIDCEnabledSet {
		parameters["k8s_oidc_enabled"] = oidcEnabled
	}
	// Set OIDC issuer URL if it is set
	if oidcIssuerURL, isOIDCIssuerURLSet := d.GetOk("oidc_issuer_url"); isOIDCIssuerURLSet {
		parameters["k8s_oidc_issuer_url"] = oidcIssuerURL
	}
	// Set OIDC client ID if it is set
	if oidcClientID, isOIDCClientIDSet := d.GetOk("oidc_client_id"); isOIDCClientIDSet {
		parameters["k8s_oidc_client_id"] = oidcClientID
	}
	// Set OIDC username claim if it is set
	if oidcUsernameClaim, isOIDCUsernameClaimSet := d.GetOk("oidc_username_claim"); isOIDCUsernameClaimSet {
		parameters["k8s_oidc_username_claim"] = oidcUsernameClaim
	}
	// Set OIDC groups claim if it is set
	if oidcGroupsClaim, isOIDCGroupsClaimSet := d.GetOk("oidc_groups_claim"); isOIDCGroupsClaimSet {
		parameters["k8s_oidc_groups_claim"] = oidcGroupsClaim
	}
	// Set signing algs if it is set
	if oidcSigningAlgs, isOIDCSigningAlgsSet := d.GetOk("oidc_signing_algs"); isOIDCSigningAlgsSet {
		parameters["k8s_oidc_signing_algs"] = oidcSigningAlgs
	}
	// Set groups prefix if it is set
	if oidcGroupsPrefix, isOIDCGroupsPrefixSet := d.GetOk("oidc_groups_prefix"); isOIDCGroupsPrefixSet {
		parameters["k8s_oidc_groups_prefix"] = oidcGroupsPrefix
	}
	// Set username prefix if it is set
	if oidcUsernamePrefix, isOIDCUsernamePrefixSet := d.GetOk("oidc_username_prefix"); isOIDCUsernamePrefixSet {
		parameters["k8s_oidc_username_prefix"] = oidcUsernamePrefix
	}
	// Set OIDC required claim if it is set
	if oidcRequiredClaim, isOIDCRequiredClaimSet := d.GetOk("oidc_required_claim"); isOIDCRequiredClaimSet {
		parameters["k8s_oidc_required_claim"] = oidcRequiredClaim
	}
	// Set OIDC CA PEM if it is set
	if oidcCAPEM, isOIDCCAPEMSet := d.GetOk("oidc_ca_pem"); isOIDCCAPEMSet {
		parameters["k8s_oidc_ca_pem"] = oidcCAPEM
	}

	// Set hubble if it is set
	if hubble, isHubbleSet := d.GetOk("hubble"); isHubbleSet {
		parameters["k8s_hubble"] = hubble.(bool)
	}

	// Set kube API server enabling if it is set
	if kubeAPIServerLogEnabled, isKubeAPIServerLogEnabledSet := d.GetOk("kube_apiserver_log_enabled"); isKubeAPIServerLogEnabledSet {
		parameters["k8s_kube_apiserver_log_enabled"] = kubeAPIServerLogEnabled.(bool)
	}

	// Set audit log enabling if it is set
	if auditLogEnabled, isAuditLogEnabledSet := d.GetOk("audit_log_enabled"); isAuditLogEnabledSet {
		parameters["k8s_audit_log_enabled"] = auditLogEnabled.(bool)
	}

	// Set audit log level if it is set
	if auditLogLevel, isAuditLogLevelSet := d.GetOk("audit_log_level"); isAuditLogLevelSet {
		parameters["k8s_audit_log_level"] = auditLogLevel
	}

	// Set log delivery if it is set
	if logDelivery, isLogDeliverySet := d.GetOk("log_delivery"); isLogDeliverySet {
		parameters["k8s_log_delivery"] = logDelivery.(bool)
	}

	// Set log delivery bucket if it is set
	if logDeliveryBucket, isLogDeliveryBucketSet := d.GetOk("log_delivery_bucket"); isLogDeliveryBucketSet {
		parameters["k8s_log_delivery_bucket"] = logDeliveryBucket
	}

	// Set log delivery access key if it is set
	if logDeliveryAccessKey, isLogDeliveryAccessKeySet := d.GetOk("log_delivery_access_key"); isLogDeliveryAccessKeySet {
		parameters["k8s_log_delivery_access_key"] = logDeliveryAccessKey
	}

	// Set log delivery secret key if it is set
	if logDeliverySecretKey, isLogDeliverySecretKeySet := d.GetOk("log_delivery_secret_key"); isLogDeliverySecretKeySet {
		parameters["k8s_log_delivery_secret_key"] = logDeliverySecretKey
	}

	// Set log delivery interval if it is set
	if logDeliveryInterval, isLogDeliveryIntervalSet := d.GetOk("log_delivery_interval"); isLogDeliveryIntervalSet {
		parameters["k8s_log_delivery_interval"] = logDeliveryInterval
	}

	// Set log delivery endpoint if it is set
	if logDeliveryEndpoint, isLogDeliveryEndpointSet := d.GetOk("log_delivery_endpoint"); isLogDeliveryEndpointSet {
		parameters["k8s_log_delivery_endpoint"] = logDeliveryEndpoint
	}

	// Set hubble if it is set
	if hubble, isHubbleSet := d.GetOk("hubble"); isHubbleSet {
		parameters["k8s_hubble"] = hubble.(bool)
	}

	requestBody.Parameters = parameters

	ctx, cancel := context.WithTimeout(context.Background(), d.Timeout(schema.TimeoutCreate))
	defer cancel()
	response, err := client.CreatePaaSService(ctx, requestBody)
	if err != nil {
		return fmt.Errorf("3: %s error: %v", errorPrefix, err)
	}
	d.SetId(response.ObjectUUID)
	log.Printf("The id for PaaS service %s has been set to %v", requestBody.Name, response.ObjectUUID)
	return resourceGridscaleK8sRead(d, meta)
}

func resourceGridscaleK8sUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*gsclient.Client)
	errorPrefix := fmt.Sprintf("update k8s (%s) resource -", d.Id())

	labels := convSOStrings(d.Get("labels").(*schema.Set).List())
	requestBody := gsclient.PaaSServiceUpdateRequest{
		Name:   d.Get("name").(string),
		Labels: &labels,
	}
	currentTemplateUUID := d.Get("service_template_uuid")
	templateRequested, err := deriveK8sTemplateFromResourceData(client, d)
	if err != nil {
		return fmt.Errorf("%s error: %v", errorPrefix, err)
	}

	if templateRequested.Properties.ObjectUUID != currentTemplateUUID.(string) {
		requestBody.PaaSServiceTemplateUUID = templateRequested.Properties.ObjectUUID
	}
	parameters := make(map[string]interface{})

	// Iterate over requested node pools to set them
	if nodePoolsRequestedInterface, ok := d.GetOk("node_pool"); ok {
		nodePoolsRequested := nodePoolsRequestedInterface.([]interface{})
		nodePools := make([]map[string]interface{}, 0)

		for index, _ := range nodePoolsRequested {
			nodePool := make(map[string]interface{}, 0)
			nodePool["name"] = d.Get(fmt.Sprintf("node_pool.%d.name", index))
			nodePool["count"] = d.Get(fmt.Sprintf("node_pool.%d.node_count", index))
			nodePool["ram"] = d.Get(fmt.Sprintf("node_pool.%d.memory", index))
			nodePool["cores"] = d.Get(fmt.Sprintf("node_pool.%d.cores", index))
			nodePool["storage"] = d.Get(fmt.Sprintf("node_pool.%d.storage", index))
			nodePool["storage_type"] = d.Get(fmt.Sprintf("node_pool.%d.storage_type", index))
			nodePool["rocket_storage"] = d.Get(fmt.Sprintf("node_pool.%d.rocket_storage", index))

			nodePools = append(nodePools, nodePool)
		}
		parameters["pools"] = nodePools
	}

	// Set cluster CIDR if it is set
	if clusterCIDR, isClusterCIDRSet := d.GetOk("cluster_cidr"); isClusterCIDRSet {
		parameters["k8s_cluster_cidr"] = clusterCIDR
	}
	// Set surge node count
	isSurgeNodeEnabled := d.Get("surge_node").(bool)
	if isSurgeNodeEnabled {
		parameters["k8s_surge_node_count"] = 1
	} else {
		parameters["k8s_surge_node_count"] = 0
	}
	// Set cluster traffic encryption if it is set
	if clusterTrafficEncryption, isSet := d.GetOk("cluster_traffic_encryption"); isSet {
		parameters["k8s_cluster_traffic_encryption"] = clusterTrafficEncryption
	}
	// Set OIDC enabled flag if it is set
	if oidcEnabled, isOIDCEnabledSet := d.GetOk("oidc_enabled"); isOIDCEnabledSet {
		parameters["k8s_oidc_enabled"] = oidcEnabled
	}
	// Set OIDC issuer URL if it is set
	if oidcIssuerURL, isOIDCIssuerURLSet := d.GetOk("oidc_issuer_url"); isOIDCIssuerURLSet {
		parameters["k8s_oidc_issuer_url"] = oidcIssuerURL
	}
	// Set OIDC client ID if it is set
	if oidcClientID, isOIDCClientIDSet := d.GetOk("oidc_client_id"); isOIDCClientIDSet {
		parameters["k8s_oidc_client_id"] = oidcClientID
	}
	// Set OIDC username claim if it is set
	if oidcUsernameClaim, isOIDCUsernameClaimSet := d.GetOk("oidc_username_claim"); isOIDCUsernameClaimSet {
		parameters["k8s_oidc_username_claim"] = oidcUsernameClaim
	}
	// Set OIDC groups claim if it is set
	if oidcGroupsClaim, isOIDCGroupsClaimSet := d.GetOk("oidc_groups_claim"); isOIDCGroupsClaimSet {
		parameters["k8s_oidc_groups_claim"] = oidcGroupsClaim
	}
	// Set signing algs if it is set
	if oidcSigningAlgs, isOIDCSigningAlgsSet := d.GetOk("oidc_signing_algs"); isOIDCSigningAlgsSet {
		parameters["k8s_oidc_signing_algs"] = oidcSigningAlgs
	}
	// Set groups prefix if it is set
	if oidcGroupsPrefix, isOIDCGroupsPrefixSet := d.GetOk("oidc_groups_prefix"); isOIDCGroupsPrefixSet {
		parameters["k8s_oidc_groups_prefix"] = oidcGroupsPrefix
	}
	// Set username prefix if it is set
	if oidcUsernamePrefix, isOIDCUsernamePrefixSet := d.GetOk("oidc_username_prefix"); isOIDCUsernamePrefixSet {
		parameters["k8s_oidc_username_prefix"] = oidcUsernamePrefix
	}
	// Set OIDC required claim if it is set
	if oidcRequiredClaim, isOIDCRequiredClaimSet := d.GetOk("oidc_required_claim"); isOIDCRequiredClaimSet {
		parameters["k8s_oidc_required_claim"] = oidcRequiredClaim
	}
	// Set OIDC CA PEM if it is set
	if oidcCAPEM, isOIDCCAPEMSet := d.GetOk("oidc_ca_pem"); isOIDCCAPEMSet {
		parameters["k8s_oidc_ca_pem"] = oidcCAPEM
	}
	// Set hubble if it is set
	if hubble, isHubbleSet := d.GetOk("hubble"); isHubbleSet {
		parameters["k8s_hubble"] = hubble
	}

	// Set kube API server enabling if it is set
	if kubeAPIServerLogEnabled, isKubeAPIServerLogEnabledSet := d.GetOk("kube_apiserver_log_enabled"); isKubeAPIServerLogEnabledSet {
		parameters["k8s_kube_apiserver_log_enabled"] = kubeAPIServerLogEnabled
	}

	// Set audit log enabling if it is set
	if auditLogEnabled, isAuditLogEnabledSet := d.GetOk("audit_log_enabled"); isAuditLogEnabledSet {
		parameters["k8s_audit_log_enabled"] = auditLogEnabled
	}

	// Set audit log level if it is set
	if auditLogLevel, isAuditLogLevelSet := d.GetOk("audit_log_level"); isAuditLogLevelSet {
		parameters["k8s_audit_log_level"] = auditLogLevel
	}

	// Set log delivery if it is set
	if logDelivery, isLogDeliverySet := d.GetOk("log_delivery"); isLogDeliverySet {
		parameters["k8s_log_delivery"] = logDelivery
	}

	// Set log delivery bucket if it is set
	if logDeliveryBucket, isLogDeliveryBucketSet := d.GetOk("log_delivery_bucket"); isLogDeliveryBucketSet {
		parameters["k8s_log_delivery_bucket"] = logDeliveryBucket
	}

	// Set log delivery access key if it is set
	if logDeliveryAccessKey, isLogDeliveryAccessKeySet := d.GetOk("log_delivery_access_key"); isLogDeliveryAccessKeySet {
		parameters["k8s_log_delivery_access_key"] = logDeliveryAccessKey
	}

	// Set log delivery secret key if it is set
	if logDeliverySecretKey, isLogDeliverySecretKeySet := d.GetOk("log_delivery_secret_key"); isLogDeliverySecretKeySet {
		parameters["k8s_log_delivery_secret_key"] = logDeliverySecretKey
	}

	// Set log delivery interval if it is set
	if logDeliveryInterval, isLogDeliveryIntervalSet := d.GetOk("log_delivery_interval"); isLogDeliveryIntervalSet {
		parameters["k8s_log_delivery_interval"] = logDeliveryInterval
	}

	// Set log delivery endpoint if it is set
	if logDeliveryEndpoint, isLogDeliveryEndpointSet := d.GetOk("log_delivery_endpoint"); isLogDeliveryEndpointSet {
		parameters["k8s_log_delivery_endpoint"] = logDeliveryEndpoint
	}

	// Set hubble if it is set
	if hubble, isHubbleSet := d.GetOk("hubble"); isHubbleSet {
		parameters["k8s_hubble"] = hubble
	}
	requestBody.Parameters = parameters

	ctx, cancel := context.WithTimeout(context.Background(), d.Timeout(schema.TimeoutUpdate))
	defer cancel()
	err = client.UpdatePaaSService(ctx, d.Id(), requestBody)
	if err != nil {
		return fmt.Errorf("%s error: %v", errorPrefix, err)
	}
	return resourceGridscaleK8sRead(d, meta)
}

func resourceGridscaleK8sDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*gsclient.Client)
	errorPrefix := fmt.Sprintf("delete k8s (%s) resource -", d.Id())

	ctx, cancel := context.WithTimeout(context.Background(), d.Timeout(schema.TimeoutDelete))
	defer cancel()
	err := errHandler.SuppressHTTPErrorCodes(
		client.DeletePaaSService(ctx, d.Id()),
		http.StatusNotFound,
	)
	if err != nil {
		return fmt.Errorf("%s error: %v", errorPrefix, err)
	}
	return nil
}

func validateK8sParameters(d *schema.ResourceDiff, template gsclient.PaaSTemplate) error {
	var errorMessages []string
	validator := &ResourceGridscaleK8sValidator{}
	err := validator.checkIfTemplateSupportsMultiNodePools(template)

	if err != nil {
		return err
	}
	templateParameterNodePools, templateParameterNodePoolsFound := template.Properties.ParametersSchema["pools"]
	nodePoolsRequestedInterface, isNodePoolsRequested := d.GetOk("node_pool")

	if templateParameterNodePoolsFound && isNodePoolsRequested {
		nodePoolsRequested := nodePoolsRequestedInterface.([]interface{})
		for index, _ := range nodePoolsRequested {
			nodePoolParameterMemory, mem_ok := templateParameterNodePools.Schema.Schema["ram"]
			if memory, ok := d.GetOk(fmt.Sprintf("node_pool.%d.memory", index)); ok && mem_ok {
				if memory.(int) < nodePoolParameterMemory.Min || memory.(int) > nodePoolParameterMemory.Max {
					errorMessages = append(
						errorMessages,
						fmt.Sprintf(
							"Invalid 'node_pool.%d.memory' value. Value must stay between %d and %d\n",
							index,
							nodePoolParameterMemory.Min,
							nodePoolParameterMemory.Max,
						),
					)
				}
			}

			nodePoolParameterCores, core_ok := templateParameterNodePools.Schema.Schema["cores"]
			if core, ok := d.GetOk(fmt.Sprintf("node_pool.%d.cores", index)); ok && core_ok {
				if core.(int) < nodePoolParameterCores.Min || core.(int) > nodePoolParameterCores.Max {
					errorMessages = append(
						errorMessages,
						fmt.Sprintf(
							"Invalid 'node_pool.%d.cores' value. Value must stay between %d and %d\n",
							index,
							nodePoolParameterCores.Min,
							nodePoolParameterCores.Max,
						),
					)
				}
			}

			nodePoolParameterCount, worker_count_ok := templateParameterNodePools.Schema.Schema["count"]
			if node_count, ok := d.GetOk(fmt.Sprintf("node_pool.%d.node_count", index)); ok && worker_count_ok {
				if node_count.(int) < nodePoolParameterCount.Min || node_count.(int) > nodePoolParameterCount.Max {
					errorMessages = append(
						errorMessages,
						fmt.Sprintf(
							"Invalid 'node_pool.%d.node_count' value. Value must stay between %d and %d\n",
							index,
							nodePoolParameterCount.Min,
							nodePoolParameterCount.Max,
						),
					)
				}
			}

			nodePoolParameterStorage, storage_ok := templateParameterNodePools.Schema.Schema["storage"]
			if storage, ok := d.GetOk(fmt.Sprintf("node_pool.%d.storage", index)); ok && storage_ok {
				if storage.(int) < nodePoolParameterStorage.Min || storage.(int) > nodePoolParameterStorage.Max {
					errorMessages = append(
						errorMessages,
						fmt.Sprintf(
							"Invalid 'node_pool.%d.storage' value. Value must stay between %d and %d\n",
							index,
							nodePoolParameterStorage.Min,
							nodePoolParameterStorage.Max,
						),
					)
				}
			}

			nodePoolParameterRocketStorage, rocket_storage_ok := templateParameterNodePools.Schema.Schema["rocket_storage"]
			if rocket_storage, ok := d.GetOk(fmt.Sprintf("node_pool.%d.rocket_storage", index)); ok && rocket_storage_ok {
				rocketStorageValidation := true
				featureReleaseCompabilityValidation := true
				supportedRelease, err := NewRelease(k8sRocketStorageSupportRelease)
				if err != nil {
					panic("Something went wrong at backend side parsing of version string expected for support of rocket storage at k8s.")
				}
				requestedRelease, err := NewRelease(template.Properties.Release)
				if err != nil {
					errorMessages = append(errorMessages, "The release doesn't match a valid version string.")
					featureReleaseCompabilityValidation = false
				}
				if featureReleaseCompabilityValidation {
					err := requestedRelease.CheckIfFeatureIsKnown(&Feature{Description: "rocket storage", Release: *supportedRelease})
					if err != nil {
						errorMessages = append(errorMessages, err.Error())
						rocketStorageValidation = false
					}
				}
				if rocketStorageValidation && (rocket_storage.(int) < nodePoolParameterRocketStorage.Min || rocket_storage.(int) > nodePoolParameterRocketStorage.Max) {
					errorMessages = append(
						errorMessages,
						fmt.Sprintf(
							"Invalid 'node_pool.%d.rocket_storage' value. Value must stay between %d and %d\n",
							index,
							nodePoolParameterRocketStorage.Min,
							nodePoolParameterRocketStorage.Max,
						),
					)
				}
			}

			nodePoolParameterStorageType, storage_type_ok := templateParameterNodePools.Schema.Schema["storage_type"]
			if storage_type, ok := d.GetOk(fmt.Sprintf("node_pool.%d.storage_type", index)); ok && storage_type_ok {
				var isValid bool
				for _, allowedValue := range nodePoolParameterStorageType.Allowed {
					if storage_type.(string) == allowedValue {
						isValid = true
					}
				}
				if !isValid {
					errorMessages = append(
						errorMessages,
						fmt.Sprintf(
							"Invalid 'node_pool.%d.storage_type' value. Value must be one of these:\n\t%s",
							index,
							strings.Join(nodePoolParameterStorageType.Allowed, "\n\t"),
						),
					)
				}
			}
		}
	}

	templateParameterClusterCIDR, templateParameterFound := template.Properties.ParametersSchema["k8s_cluster_cidr"]
	if clusterCIDR, ok := d.GetOk("cluster_cidr"); ok {
		// if the template doesn't support cluster_cidr, return error if it is set
		if !templateParameterFound {
			errorMessages = append(errorMessages, "The template doesn't support cluster_cidr. Please remove it from your configuration.\n")
		} else {
			// if the template supports cluster_cidr, validate the value
			if clusterCIDR.(string) != "" {
				_, _, err := net.ParseCIDR(clusterCIDR.(string))
				if err != nil {
					errorMessages = append(errorMessages, "Invalid value for PaaS template release. Value must be a valid CIDR.\n")
				}
			}
			// if cluster_cidr_template is immutable, return error if it is set during k8s creation
			// and it is changed during k8s update
			if templateParameterClusterCIDR.Immutable {
				oldClusterCIDR, _ := d.GetChange("cluster_cidr")
				if oldClusterCIDR != "" && d.HasChange("cluster_cidr") {
					errorMessages = append(errorMessages, "Cannot change parameter cluster_cidr, because it is immutable.\n")
				}
			}
		}
	}

	if interfaceOIDCIssuerURL, ok := d.GetOk("oidc_issuer_url"); ok {
		if _, ok := template.Properties.ParametersSchema["k8s_oidc_issuer_url"]; ok {
			validMode := regexp.MustCompile(`^https:\/\/.*`)
			if !validMode.MatchString(interfaceOIDCIssuerURL.(string)) {
				errorMessages = append(errorMessages, fmt.Sprintf("Invalid OIDC 'issuer_url' value. Example value: '%s'\n", "https://example.io"))
			}
		}
	}

	templateParameterOIDCSigningAlgs, templateParameterFound := template.Properties.ParametersSchema["k8s_oidc_signing_algs"]
	if oidcSigningAlgs, ok := d.GetOk("oidc_signing_algs"); ok && templateParameterFound {
		var isValid bool
		for _, allowedValue := range templateParameterOIDCSigningAlgs.Allowed {
			if oidcSigningAlgs.(string) == allowedValue {
				isValid = true
			}
		}
		if !isValid {
			errorMessages = append(errorMessages,
				fmt.Sprintf("Invalid OIDC 'signing_algs' value. Value must be one of these:\n\t%s",
					strings.Join(templateParameterOIDCSigningAlgs.Allowed, "\n\t"),
				),
			)
		}
	}

	if interfaceOIDCCAPEM, ok := d.GetOk("oidc_ca_pem"); ok {
		if _, ok := template.Properties.ParametersSchema["k8s_oidc_ca_pem"]; ok {
			block, _ := pem.Decode([]byte(interfaceOIDCCAPEM.(string)))
			if block == nil {
				return fmt.Errorf("invalid OIDC 'ca_pem' value, failed to parse to CA PEM")
			}
		}
	}

	templateParameterAuditLogLevel, templateParameterFound := template.Properties.ParametersSchema["k8s_audit_log_level"]
	if auditLogLevel, ok := d.GetOk("audit_log_level"); ok && templateParameterFound {
		var isValid bool
		for _, allowedValue := range templateParameterAuditLogLevel.Allowed {
			if auditLogLevel.(string) == allowedValue {
				isValid = true
			}
		}
		if !isValid {
			errorMessages = append(errorMessages,
				fmt.Sprintf("Invalid 'audit_log_level' value. Value must be one of these:\n\t%s",
					strings.Join(templateParameterAuditLogLevel.Allowed, "\n\t"),
				),
			)
		}
	}

	if interfaceLogDeliveryBucket, ok := d.GetOk("log_delivery_bucket"); ok {
		if templateParameterLogDeliveryBucket, ok := template.Properties.ParametersSchema["k8s_log_delivery_bucket"]; ok {
			validMode := regexp.MustCompile(templateParameterLogDeliveryBucket.Regex)
			if !validMode.MatchString(interfaceLogDeliveryBucket.(string)) {
				errorMessages = append(errorMessages, fmt.Sprintf("Invalid 'log_delivery_bucket' value. Example value: '%s'\n", "foo"))
			}
		}
	}

	if interfaceLogDeliveryAccessKey, ok := d.GetOk("log_delivery_access_key"); ok {
		if templateParameterLogDeliveryAccessKey, ok := template.Properties.ParametersSchema["k8s_log_delivery_access_key"]; ok {
			validMode := regexp.MustCompile(templateParameterLogDeliveryAccessKey.Regex)
			if !validMode.MatchString(interfaceLogDeliveryAccessKey.(string)) {
				errorMessages = append(errorMessages, fmt.Sprintf("Invalid 'log_delivery_access_key' value. Example value: '%s'\n", "011000010110001101100011011001010111001101110011"))
			}
		}
	}

	if interfaceLogDeliverySecretKey, ok := d.GetOk("log_delivery_secret_key"); ok {
		if templateParameterLogDeliverySecretKey, ok := template.Properties.ParametersSchema["k8s_log_delivery_secret_key"]; ok {
			validMode := regexp.MustCompile(templateParameterLogDeliverySecretKey.Regex)
			if !validMode.MatchString(interfaceLogDeliverySecretKey.(string)) {
				errorMessages = append(errorMessages, fmt.Sprintf("Invalid 'log_delivery_secret_key' value. Example value: '%s'\n", "011100110110010101100011011100100110010101110100"))
			}
		}
	}

	if interfaceLogDeliveryInterval, ok := d.GetOk("log_delivery_interval"); ok {
		if templateParameterLogDeliveryInterval, ok := template.Properties.ParametersSchema["k8s_log_delivery_interval"]; ok {
			if interfaceLogDeliveryInterval.(int) < templateParameterLogDeliveryInterval.Min || interfaceLogDeliveryInterval.(int) > templateParameterLogDeliveryInterval.Max {
				errorMessages = append(
					errorMessages,
					fmt.Sprintf(
						"Invalid 'log_delivery_interval' value. Value must stay between %d and %d\n",
						templateParameterLogDeliveryInterval.Min,
						templateParameterLogDeliveryInterval.Max,
					),
				)
			}
		}
	}

	if interfaceLogDeliveryEndpoint, ok := d.GetOk("log_delivery_endpoint"); ok {
		if _, ok := template.Properties.ParametersSchema["k8s_log_delivery_endpoint"]; ok {
			validMode := regexp.MustCompile(`^https:\/\/.*`)
			if !validMode.MatchString(interfaceLogDeliveryEndpoint.(string)) {
				errorMessages = append(errorMessages, fmt.Sprintf("Invalid 'log_delivery_endpoint' value. Example value: '%s'\n", "https://gos3.io"))
			}
		}
	}

	if len(errorMessages) != 0 {
		return errors.New(strings.Join(errorMessages, ""))
	}
	return nil
}
