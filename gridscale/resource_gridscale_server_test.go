package gridscale

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	"github.com/gridscale/gsclient-go/v3"
)

func TestAccResourceGridscaleServer_Basic(t *testing.T) {
	var object gsclient.Server
	name := fmt.Sprintf("object-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckResourceGridscaleServerDestroyCheck,
		Steps: []resource.TestStep{
			{
				Config: testAccCheckResourceGridscaleServerConfig_basic(name),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceGridscaleServerExists("gridscale_server.foo", &object),
					resource.TestCheckResourceAttr(
						"gridscale_server.foo", "name", name),
					resource.TestCheckResourceAttr(
						"gridscale_server.foo", "cores", "2"),
					resource.TestCheckResourceAttr(
						"gridscale_server.foo", "memory", "2"),
					resource.TestCheckResourceAttr(
						"gridscale_server.foo", "power", "true"),
				),
			},
			{
				Config: testAccCheckResourceGridscaleServerConfig_basic_update(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceGridscaleServerExists("gridscale_server.foo", &object),
					resource.TestCheckResourceAttr(
						"gridscale_server.foo", "name", "newname"),
					resource.TestCheckResourceAttr(
						"gridscale_server.foo", "cores", "1"),
					resource.TestCheckResourceAttr(
						"gridscale_server.foo", "memory", "1"),
					resource.TestCheckResourceAttr(
						"gridscale_server.foo", "power", "true"),
				),
			},
		},
	})
}

func testAccCheckResourceGridscaleServerExists(n string, object *gsclient.Server) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No object UUID is set")
		}

		client := testAccProvider.Meta().(*gsclient.Client)

		id := rs.Primary.ID

		foundObject, err := client.GetServer(context.Background(), id)

		if err != nil {
			return err
		}

		if foundObject.Properties.ObjectUUID != id {
			return fmt.Errorf("Object not found")
		}

		*object = foundObject

		return nil
	}
}

func testAccCheckResourceGridscaleServerDestroyCheck(s *terraform.State) error {
	client := testAccProvider.Meta().(*gsclient.Client)
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "gridscale_server" {
			continue
		}

		_, err := client.GetServer(context.Background(), rs.Primary.ID)
		if err != nil {
			if requestError, ok := err.(gsclient.RequestError); ok {
				if requestError.StatusCode != 404 {
					return fmt.Errorf("Object %s still exists", rs.Primary.ID)
				}
			} else {
				return fmt.Errorf("Unable to fetching object %s", rs.Primary.ID)
			}
		} else {
			return fmt.Errorf("Object %s still exists", rs.Primary.ID)
		}
	}

	return nil
}

func testAccCheckResourceGridscaleServerConfig_basic(name string) string {
	return fmt.Sprintf(`
resource "gridscale_ipv4" "foo" {
  name   = "ip-%s"
}
resource "gridscale_network" "foo" {
  name   = "net-%s"
}
resource "gridscale_storage" "foo" {
  name   = "storage- %s"
  capacity = 1
}
resource "gridscale_server" "foo" {
  name   = "%s"
  cores = 2
  memory = 2
  power = true
  ipv4 = gridscale_ipv4.foo.id
  network {
		object_uuid = gridscale_network.foo.id
		ordering = 0
		rules_v4_in {
				order = 0
				protocol = "tcp"
				action = "drop"
				dst_port = "20:80"
				comment = "test"
		}
		rules_v4_out {
				order = 1
				protocol = "tcp"
				action = "drop"
				dst_port = "80:443"
				comment = "test1"
		}
		rules_v6_in	{
				order = 2
				protocol = "tcp"
				action = "drop"
				dst_port = "100:500"
				comment = "test2"
		}
  	}
  storage {
  	object_uuid = gridscale_storage.foo.id
  }
}
`, name, name, name, name)
}

func testAccCheckResourceGridscaleServerConfig_basic_update() string {
	return fmt.Sprintf(`
resource "gridscale_ipv4" "foo1" {
  name   = "newname"
}
resource "gridscale_network" "foo" {
  name   = "newname"
}
resource "gridscale_storage" "foo1" {
  name   = "newname"
  capacity = 1
}
resource "gridscale_server" "foo" {
  name   = "newname"
  cores = 1
  memory = 1
  power = true
  ipv4 = gridscale_ipv4.foo1.id
  network {
		object_uuid = gridscale_network.foo.id
		ordering = 0
		rules_v4_in {
				order = 0
				protocol = "tcp"
				action = "drop"
				dst_port = "20:80"
				comment = "test"
		}
		rules_v6_in	{
				order = 1
				protocol = "tcp"
				action = "drop"
				dst_port = "10:20"
				comment = "test1"
		}
  	}
  storage {
  	object_uuid = gridscale_storage.foo1.id
  }
}
`)
}
