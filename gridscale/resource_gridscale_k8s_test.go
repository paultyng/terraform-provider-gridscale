package gridscale

import (
	"fmt"

	"github.com/gridscale/gsclient-go/v3"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"

	"testing"
)

const oidcCAPEM = `-----BEGIN CERTIFICATE-----\nMIIFazCCA1OgAwIBAgIRAIIQz7DSQONZRGPgu2OCiwAwDQYJKoZIhvcNAQELBQAw\nTzELMAkGA1UEBhMCVVMxKTAnBgNVBAoTIEludGVybmV0IFNlY3VyaXR5IFJlc2Vh\ncmNoIEdyb3VwMRUwEwYDVQQDEwxJU1JHIFJvb3QgWDEwHhcNMTUwNjA0MTEwNDM4\nWhcNMzUwNjA0MTEwNDM4WjBPMQswCQYDVQQGEwJVUzEpMCcGA1UEChMgSW50ZXJu\nZXQgU2VjdXJpdHkgUmVzZWFyY2ggR3JvdXAxFTATBgNVBAMTDElTUkcgUm9vdCBY\nMTCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoCggIBAK3oJHP0FDfzm54rVygc\nh77ct984kIxuPOZXoHj3dcKi/vVqbvYATyjb3miGbESTtrFj/RQSa78f0uoxmyF+\n0TM8ukj13Xnfs7j/EvEhmkvBioZxaUpmZmyPfjxwv60pIgbz5MDmgK7iS4+3mX6U\nA5/TR5d8mUgjU+g4rk8Kb4Mu0UlXjIB0ttov0DiNewNwIRt18jA8+o+u3dpjq+sW\nT8KOEUt+zwvo/7V3LvSye0rgTBIlDHCNAymg4VMk7BPZ7hm/ELNKjD+Jo2FR3qyH\nB5T0Y3HsLuJvW5iB4YlcNHlsdu87kGJ55tukmi8mxdAQ4Q7e2RCOFvu396j3x+UC\nB5iPNgiV5+I3lg02dZ77DnKxHZu8A/lJBdiB3QW0KtZB6awBdpUKD9jf1b0SHzUv\nKBds0pjBqAlkd25HN7rOrFleaJ1/ctaJxQZBKT5ZPt0m9STJEadao0xAH0ahmbWn\nOlFuhjuefXKnEgV4We0+UXgVCwOPjdAvBbI+e0ocS3MFEvzG6uBQE3xDk3SzynTn\njh8BCNAw1FtxNrQHusEwMFxIt4I7mKZ9YIqioymCzLq9gwQbooMDQaHWBfEbwrbw\nqHyGO0aoSCqI3Haadr8faqU9GY/rOPNk3sgrDQoo//fb4hVC1CLQJ13hef4Y53CI\nrU7m2Ys6xt0nUW7/vGT1M0NPAgMBAAGjQjBAMA4GA1UdDwEB/wQEAwIBBjAPBgNV\nHRMBAf8EBTADAQH/MB0GA1UdDgQWBBR5tFnme7bl5AFzgAiIyBpY9umbbjANBgkq\nhkiG9w0BAQsFAAOCAgEAVR9YqbyyqFDQDLHYGmkgJykIrGF1XIpu+ILlaS/V9lZL\nubhzEFnTIZd+50xx+7LSYK05qAvqFyFWhfFQDlnrzuBZ6brJFe+GnY+EgPbk6ZGQ\n3BebYhtF8GaV0nxvwuo77x/Py9auJ/GpsMiu/X1+mvoiBOv/2X/qkSsisRcOj/KK\nNFtY2PwByVS5uCbMiogziUwthDyC3+6WVwW6LLv3xLfHTjuCvjHIInNzktHCgKQ5\nORAzI4JMPJ+GslWYHb4phowim57iaztXOoJwTdwJx4nLCgdNbOhdjsnvzqvHu7Ur\nTkXWStAmzOVyyghqpZXjFaH3pO3JLF+l+/+sKAIuvtd7u+Nxe5AW0wdeRlN8NwdC\njNPElpzVmbUq4JUagEiuTDkHzsxHpFKVK7q4+63SM1N95R1NbdWhscdCb+ZAJzVc\noyi3B43njTOQ5yOf+1CceWxG1bQVs5ZufpsMljq4Ui0/1lvh+wjChP4kqKOJ2qxq\n4RgqsahDYVvTH9w7jXbyLeiNdd8XM2w9U/t7y0Ff/9yi0GE44Za4rF2LN9d11TPA\nmRGunUHBcnWEvgJBQl9nJEiU0Zsnvgc/ubhPgXRR4Xq37Z0j4r7g1SgEEzwxA57d\nemyPxgcYxn/eR44/KJ4EBs+lVDR3veyJm+kXQ99b21/+jh5Xos1AnX5iItreGCc=\n-----END CERTIFICATE-----\n`

func TestAccResourceGridscaleK8sBasic(t *testing.T) {
	var object gsclient.PaaSService
	name := fmt.Sprintf("k8s-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckResourceGridscalePaaSDestroyCheck,
		Steps: []resource.TestStep{
			{
				Config: testAccCheckResourceGridscaleK8sConfigBasic(name),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceGridscalePaaSExists("gridscale_k8s.foopaas", &object),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "name", name),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "release", "1.30"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.name", "my-node-pool"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.node_count", "1"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.cores", "1"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.memory", "2"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.storage", "30"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.storage_type", "storage_insane"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.rocket_storage", "90"),
				),
			},
			{
				Config: testAccCheckResourceGridscaleK8sConfigBasicUpdate(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceGridscalePaaSExists("gridscale_k8s.foopaas", &object),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "name", "newname"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "release", "1.30"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.name", "my-node-pool"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.node_count", "1"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.cores", "1"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.memory", "2"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.storage", "30"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.storage_type", "storage_insane"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.rocket_storage", "90"),
				),
			},
			{
				Config: testAccCheckResourceGridscaleK8sConfigNodePoolSpecsUpdate(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceGridscalePaaSExists("gridscale_k8s.foopaas", &object),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.name", "my-node-pool"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.node_count", "1"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.cores", "2"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.memory", "4"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.storage", "50"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.storage_type", "storage_insane"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.rocket_storage", "90"),
				),
			},
			{
				Config: testAccCheckResourceGridscaleK8sConfigNodeCountIncrease(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceGridscalePaaSExists("gridscale_k8s.foopaas", &object),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.name", "my-node-pool"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.node_count", "2"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.cores", "2"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.memory", "4"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.storage", "50"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.storage_type", "storage_insane"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.rocket_storage", "90"),
				),
			},
			{
				Config: testAccCheckResourceGridscaleK8sConfigNodeCountDecrease(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceGridscalePaaSExists("gridscale_k8s.foopaas", &object),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.name", "my-node-pool"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.node_count", "1"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.cores", "2"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.memory", "4"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.storage", "50"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.storage_type", "storage_insane"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.rocket_storage", "90"),
				),
			},
			{
				Config: testAccCheckResourceGridscaleK8sConfigAddNodePool(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceGridscalePaaSExists("gridscale_k8s.foopaas", &object),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.name", "my-node-pool"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.node_count", "1"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.cores", "2"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.memory", "4"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.storage", "50"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.storage_type", "storage_insane"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.rocket_storage", "90"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.1.name", "my-node-pool-2"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.1.node_count", "1"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.1.cores", "1"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.1.memory", "2"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.1.storage", "30"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.1.storage_type", "storage_insane"),
				),
			},
			{
				Config: testAccCheckResourceGridscaleK8sConfigRemoveNodePool(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceGridscalePaaSExists("gridscale_k8s.foopaas", &object),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.name", "my-node-pool"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.node_count", "1"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.cores", "2"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.memory", "4"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.storage", "50"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.storage_type", "storage_insane"),
					resource.TestCheckResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.0.rocket_storage", "90"),
					resource.TestCheckNoResourceAttr(
						"gridscale_k8s.foopaas", "node_pool.1",
					),
				),
			},
		},
	})
}

func testAccCheckResourceGridscaleK8sConfigBasic(name string) string {
	return fmt.Sprintf(`
resource "gridscale_k8s" "foopaas" {
	name   = "%s"
	release = "1.30"
	node_pool {
		name = "my-node-pool"
		node_count = 1
		cores = 1
		memory = 2
		storage = 30
		storage_type = "storage_insane"
		rocket_storage = 90
	}
}
`, name)
}

func testAccCheckResourceGridscaleK8sConfigBasicUpdate() string {
	return `
resource "gridscale_k8s" "foopaas" {
	name   = "newname"
	release = "1.30"
	node_pool {
		name = "my-node-pool"
		node_count = 1
		cores = 1
		memory = 2
		storage = 30
		storage_type = "storage_insane"
		rocket_storage = 90
	}
}
`
}

func testAccCheckResourceGridscaleK8sConfigNodePoolSpecsUpdate() string {
	return `
	resource "gridscale_k8s" "foopaas" {
		name   = "newname"
		release = "1.30"
		node_pool {
			name = "my-node-pool"
			node_count = 1
			cores = 2
			memory = 4
			storage = 50
			storage_type = "storage_insane"
			rocket_storage = 90
		}
	}
	`
}

func testAccCheckResourceGridscaleK8sConfigNodeCountIncrease() string {
	return `
	resource "gridscale_k8s" "foopaas" {
		name   = "newname"
		release = "1.30"
		node_pool {
			name = "my-node-pool"
			node_count = 2
			cores = 2
			memory = 4
			storage = 50
			storage_type = "storage_insane"
			rocket_storage = 90
		}
	}
	`
}

func testAccCheckResourceGridscaleK8sConfigNodeCountDecrease() string {
	return `
	resource "gridscale_k8s" "foopaas" {
		name   = "newname"
		release = "1.30"
		node_pool {
			name = "my-node-pool"
			node_count = 1
			cores = 2
			memory = 4
			storage = 50
			storage_type = "storage_insane"
			rocket_storage = 90
		}
	}
	`
}

func testAccCheckResourceGridscaleK8sConfigAddNodePool() string {
	return `
	resource "gridscale_k8s" "foopaas" {
		name   = "newname"
		release = "1.30"
		node_pool {
			name = "my-node-pool"
			node_count = 1
			cores = 2
			memory = 4
			storage = 50
			storage_type = "storage_insane"
			rocket_storage = 90
		}
		node_pool {
			name = "my-node-pool-2"
			node_count = 1
			cores = 1
			memory = 2
			storage = 30
			storage_type = "storage_insane"
		}
	}
	`
}

func testAccCheckResourceGridscaleK8sConfigRemoveNodePool() string {
	return `
	resource "gridscale_k8s" "foopaas" {
		name   = "newname"
		release = "1.30"
		node_pool {
			name = "my-node-pool"
			node_count = 1
			cores = 2
			memory = 4
			storage = 50
			storage_type = "storage_insane"
			rocket_storage = 90
		}
	}
	`
}
