---
layout: "gridscale"
page_title: "gridscale: gridscale_server"
sidebar_current: "docs-gridscale-resource-server"
description: |-
  Manages a server in gridscale.
---

# gridscale_server

Provides a server resource. This can be used to create, modify, and delete servers.

## Example

The following example shows how one might use this resource to add a server to gridscale:

```terraform
resource "gridscale_server" "terra-server-test" {
  name = "terra-server-test"
  cores = 2
  memory = 1
  storage {
    object_uuid = gridscale_storage.terra-storage-test.id
  }
  network {
    object_uuid = gridscale_network.terra-network-test.id
    bootdevice = true
  }
  network {
    object_uuid = gridscale_network.terra-network-test-2.id
  }
  ipv4 = gridscale_ipv4.terra-ipv4-test.id
  isoimage = "9be3e0a3-42ac-4207-8887-3383c405724d"
  timeouts {
      create="10m"
  }
}
```

**NOTE: If no firewall rules are set, the firewall is inactive. Only if at least one firewall rule is set the firewall is active.
Packets that do not match any rule are blocked. E.g:

```terraform
resource "gridscale_server" "terra-server-test" {
  name = "terra-server-test"
  cores = 2
  memory = 1
  storage {
    object_uuid = gridscale_storage.terra-storage-test.id
  }
  network {
    object_uuid = gridscale_network.terra-network-test.id
    rules_v4_in {
        order = 0
        protocol = "tcp"
        action = "accept"
        dst_port = "22"
        comment = "ssh"
    }
    rules_v6_in	{
        order = 1
        protocol = "tcp"
        action = "accept"
        dst_port = "22"
        comment = "ssh"
    }
  }
  network {
    object_uuid = gridscale_network.terra-network-test-2.id
  }
  ipv4 = gridscale_ipv4.terra-ipv4-test.id
  isoimage = "9be3e0a3-42ac-4207-8887-3383c405724d"
  timeouts {
      create="10m"
  }
}
```
In this case, the inbound packets that do not match 2 rules above will be blocked.

## Argument Reference

The following arguments are supported:

* `name` - (Required) The human-readable name of the object. It supports the full UTF-8 character set, with a maximum of 64 characters.

* `cores` - (Required) The number of server cores.

* `memory` - (Required) The amount of server memory in GB.

* `labels` - (Optional) List of labels in the format [ "label1", "label2" ].

* `auto_recovery` - (Optional) If the server should be auto-started in case of a failure (default=true).

* `hardware_profile` - (Optional, ForceNew) The hardware profile of the Server. Available options are default, legacy, nested, cisco_csr, sophos_utm, f5_bigip, and q35.

* `ipv4` - (Optional) The UUID of the IPv4 address of the server. (***NOTE: The server will NOT automatically be connected to the public network; to give it access to the internet, please add server to the public network.)

* `ipv6` - (Optional) The UUID of the IPv6 address of the server. (***NOTE: The server will NOT automatically be connected to the public network; to give it access to the internet, please add server to the public network.)

* `isoimage` - (Optional) The UUID of an ISO image in gridscale. The server will automatically boot from the ISO if one was added. The UUIDs of ISO images can be found in [the expert panel](https://my.gridscale.io/Expert/ISOImage).

* `power` - (Optional, Computed) The power state of the server. Set this to true to will boot the server, false will shut it down.

* `availability_zone` - (Optional, Computed) Defines which Availability-Zone the Server is placed.

* `storage` - (Optional) Connects a storage to the server. **NOTE: The first storage is always the boot device.

    * `object_uuid` - (Required) The object UUID or id of the storage.

* `network` - (Optional) Connects a network to the server. **NOTE: Due to a known issue ([issue #142](https://github.com/gridscale/terraform-provider-gridscale/issues/142)) about network ordering, the argument `ordering` should be set and the network list should be sorted by their name. Another workaround is to use [lifecycle.ignore_changes](https://www.terraform.io/docs/language/meta-arguments/lifecycle.html#ignore_changes) temporarily to prevent the server from being rebooted.

    * `object_uuid` - (Required) The object UUID or id of the network.

    * `ordering` - (Optional) Defines the ordering of the network interfaces. Lower numbers have lower PCI-IDs.

    * `bootdevice` - (Optional, Computed) Make this network the boot device. This can only be set for one network.

    * `firewall_template_uuid` - (Optional) The UUID of firewall template.

    * `rules_v4_in` - (Optional) Firewall template rules for inbound traffic - covers ipv4 addresses.

        * `order` - (Required) The order at which the firewall will compare packets against its rules. A packet will be compared against the first rule, it will either allow it to pass or block it and it won't be matched against any other rules. However, if it does no match the rule, then it will proceed onto rule 2. Packets that do not match any rules are blocked by default (Only for inbound).

        * `action` - (Required) This defines what the firewall will do. Either accept or drop.

        * `protocol` - (Required) Either 'udp' or 'tcp'.

        * `dst_port` - (Optional) A Number between 1 and 65535, port ranges are separated by a colon for FTP.

        * `src_port` - (Optional) A Number between 1 and 65535, port ranges are separated by a colon for FTP.

        * `src_cidr` - (Optional) Either an IPv4/6 address or and IP Network in CIDR format. If this field is empty then this service has access to all IPs.

        * `dst_cidr` - (Optional) Either an IPv4/6 address or and IP Network in CIDR format. If this field is empty then this service has access to all IPs.

        * `comment` - (Optional) Comment.

    * `rules_v4_out` - (Optional) Firewall template rules for outbound traffic - covers ipv4 addresses.

        * `order` - (Required) The order at which the firewall will compare packets against its rules. A packet will be compared against the first rule, it will either allow it to pass or block it and it won't be matched against any other rules. However, if it does no match the rule, then it will proceed onto rule 2.

        * `action` - (Required) This defines what the firewall will do. Either accept or drop.

        * `protocol` - (Required) Either 'udp' or 'tcp'.

        * `dst_port` - (Optional) A Number between 1 and 65535, port ranges are separated by a colon for FTP.

        * `src_port` - (Optional) A Number between 1 and 65535, port ranges are separated by a colon for FTP.

        * `src_cidr` - (Optional) Either an IPv4/6 address or and IP Network in CIDR format. If this field is empty then this service has access to all IPs.

        * `dst_cidr` - (Optional) Either an IPv4/6 address or and IP Network in CIDR format. If this field is empty then this service has access to all IPs.

        * `comment` - (Optional) Comment.

    * `rules_v6_in` - (Optional) Firewall template rules for inbound traffic - covers ipv6 addresses.

        * `order` - (Required) The order at which the firewall will compare packets against its rules. A packet will be compared against the first rule, it will either allow it to pass or block it and it won't be matched against any other rules. However, if it does no match the rule, then it will proceed onto rule 2. Packets that do not match any rules are blocked by default (Only for inbound).

        * `action` - (Required) This defines what the firewall will do. Either accept or drop.

        * `protocol` - (Required) Either 'udp' or 'tcp'.

        * `dst_port` - (Optional) A Number between 1 and 65535, port ranges are separated by a colon for FTP.

        * `src_port` - (Optional) A Number between 1 and 65535, port ranges are separated by a colon for FTP.

        * `src_cidr` - (Optional) Either an IPv4/6 address or and IP Network in CIDR format. If this field is empty then this service has access to all IPs.

        * `dst_cidr` - (Optional) Either an IPv4/6 address or and IP Network in CIDR format. If this field is empty then this service has access to all IPs.

        * `comment` - (Optional) Comment.

    * `rules_v6_out` - (Optional) Firewall template rules for outbound traffic - covers ipv6 addresses.

        * `order` - (Required) The order at which the firewall will compare packets against its rules. A packet will be compared against the first rule, it will either allow it to pass or block it and it won't be matched against any other rules. However, if it does no match the rule, then it will proceed onto rule 2.

        * `action` - (Required) This defines what the firewall will do. Either accept or drop.

        * `protocol` - (Required) Either 'udp' or 'tcp'.

        * `dst_port` - (Optional) A Number between 1 and 65535, port ranges are separated by a colon for FTP.

        * `src_port` - (Optional) A Number between 1 and 65535, port ranges are separated by a colon for FTP.

        * `src_cidr` - (Optional) Either an IPv4/6 address or and IP Network in CIDR format. If this field is empty then this service has access to all IPs.

        * `dst_cidr` - (Optional) Either an IPv4/6 address or and IP Network in CIDR format. If this field is empty then this service has access to all IPs.

        * `comment` - (Optional) Comment.

## Timeouts

Timeouts configuration options (in seconds):
More info: [terraform.io/docs/configuration/resources.html#operation-timeouts](https://www.terraform.io/docs/configuration/resources.html#operation-timeouts)

* `create` - (Default value is "5m" - 5 minutes) Used for creating a resource.
* `update` - (Default value is "5m" - 5 minutes) Used for updating a resource.
* `delete` - (Default value is "5m" - 5 minutes) Used for deleting a resource.

## Attributes

This resource exports the following attributes:

* `id` - UUID of the server.
* `name` - The name of the server.
* `cores` - The number of server cores.
* `memory` - The amount of server memory in GB.
* `location_uuid` - The location this server is placed. The location of a resource is determined by it's project.
* `labels` - List of labels in the format [ "label1", "label2" ].
* `hardware_profile` - The hardware profile of the server.
* `storage` - Connects a storage to the server.
    * `object_uuid` - The object UUID or id of the storage.
    * `storage_type` - Indicates the speed of the storage. This may be (storage, storage_high or storage_insane).
    * `bootdevice` - True is the storage is a boot device.
    * `object_name` - Name of the storage.
    * `create_time` - Defines the date and time the object was initially created.
    * `capacity` - Capacity of the storage (GB).
    * `controller` - Defines the SCSI controller id. The SCSI defines transmission routes such as Serial Attached SCSI (SAS), Fibre Channel and iSCSI.
    * `bus` - The SCSI bus id. The SCSI defines transmission routes like Serial Attached SCSI (SAS), Fibre Channel and iSCSI. Each SCSI device is addressed via a specific number. Each SCSI bus can have multiple SCSI devices connected to it.
    * `target` - Defines the SCSI target ID. The target ID is a device (e.g. disk).
    * `lun` - Is the common SCSI abbreviation of the Logical Unit Number. A lun is a unique identifier for a single disk or a composite of disks.
    * `license_product_no` - If a template has been used that requires a license key (e.g. Windows Servers) this shows the product_no of the license (see the /prices endpoint for more details).
    * `last_used_template` - Indicates the UUID of the last used template on this storage (inherited from snapshots).
* `network` - Connects a network to the server.
    * `object_uuid` - The object UUID or id of the network.
    * `bootdevice` - Make this network the boot device. This can only be set for one network.
    * `object_name` - Name of the network.
    * `ordering` - Defines the ordering of the network interfaces. Lower numbers have lower PCI-IDs.
    * `create_time` - Defines the date and time the object was initially created.
    * `network_type` - One of network, network_high, network_insane.
    * `mac` - network_mac defines the MAC address of the network interface.
    * `firewall_template_uuid` - The UUID of firewall template.
    * `rules_v4_in` - Firewall template rules for inbound traffic - covers IPv4 addresses.
        * `order` - The order at which the firewall will compare packets against its rules. A packet will be compared against the first rule, it will either allow it to pass or block it and it won't be matched against any other rules. However, if it does no match the rule, then it will proceed onto rule 2. Packets that do not match any rules are blocked by default (Only for inbound).
        * `action` - This defines what the firewall will do. Either accept or drop.
        * `protocol` - Either 'udp' or 'tcp'.
        * `dst_port` - A Number between 1 and 65535, port ranges are separated by a colon for FTP.
        * `src_port` - A Number between 1 and 65535, port ranges are separated by a colon for FTP.
        * `src_cidr` - Either an IPv4/6 address or and IP Network in CIDR format. If this field is empty then this service has access to all IPs.
        * `dst_cidr` - Either an IPv4/6 address or and IP Network in CIDR format. If this field is empty then this service has access to all IPs.
        * `comment` - Comment.
    * `rules_v4_out` - Firewall template rules for outbound traffic - covers IPv4 addresses.
        * `order` - The order at which the firewall will compare packets against its rules. A packet will be compared against the first rule, it will either allow it to pass or block it and it won't be matched against any other rules. However, if it does no match the rule, then it will proceed onto rule 2.
        * `action` - This defines what the firewall will do. Either accept or drop.
        * `protocol` - Either 'udp' or 'tcp'.
        * `dst_port` - A Number between 1 and 65535, port ranges are separated by a colon for FTP.
        * `src_port` - A Number between 1 and 65535, port ranges are separated by a colon for FTP.
        * `src_cidr` - Either an IPv4/6 address or and IP Network in CIDR format. If this field is empty then this service has access to all IPs.
        * `dst_cidr` - Either an IPv4/6 address or and IP Network in CIDR format. If this field is empty then this service has access to all IPs.
        * `comment` - Comment.
    * `rules_v6_in` - Firewall template rules for inbound traffic - covers IPv6 addresses.
        * `order` - The order at which the firewall will compare packets against its rules. A packet will be compared against the first rule, it will either allow it to pass or block it and it won't be matched against any other rules. However, if it does no match the rule, then it will proceed onto rule 2. Packets that do not match any rules are blocked by default (Only for inbound).
        * `action` - This defines what the firewall will do. Either accept or drop.
        * `protocol` - Either 'udp' or 'tcp'.
        * `dst_port` - A Number between 1 and 65535, port ranges are separated by a colon for FTP.
        * `src_port` - A Number between 1 and 65535, port ranges are separated by a colon for FTP.
        * `src_cidr` - Either an IPv4/6 address or and IP Network in CIDR format. If this field is empty then this service has access to all IPs.
        * `dst_cidr` - Either an IPv4/6 address or and IP Network in CIDR format. If this field is empty then this service has access to all IPs.
        * `comment` - Comment.
    * `rules_v6_out` - Firewall template rules for outbound traffic - covers IPv6 addresses.
        * `order` - The order at which the firewall will compare packets against its rules. A packet will be compared against the first rule, it will either allow it to pass or block it and it won't be matched against any other rules. However, if it does no match the rule, then it will proceed onto rule 2.
        * `action` - This defines what the firewall will do. Either accept or drop.
        * `protocol` - Either 'udp' or 'tcp'.
        * `dst_port` - A Number between 1 and 65535, port ranges are separated by a colon for FTP.
        * `src_port` - A Number between 1 and 65535, port ranges are separated by a colon for FTP.
        * `src_cidr` - Either an IPv4/6 address or and IP Network in CIDR format. If this field is empty then this service has access to all IPs.
        * `dst_cidr` - Either an IPv4/6 address or and IP Network in CIDR format. If this field is empty then this service has access to all IPs.
        * `comment` - Comment.
* `ipv4` - The UUID of the IPv4 address of the server.
* `ipv6` - The UUID of the IPv6 address of the server.
* `isoimage` - The UUID of an ISO image in gridscale.
* `power` - The power state of the server.
* `availability_zone` - Defines which Availability-Zone the Server is placed.
* `auto_recovery` - If the server should be auto-started in case of a failure.
* `console_token` - The token used by the panel to open the websocket VNC connection to the server console.
* `legacy` - Legacy-Hardware emulation instead of virtio hardware. If enabled, hot-plugging cores, memory, storage, network, etc. will not work, but the server will most likely run every x86 compatible operating system. This mode comes with a performance penalty, as emulated hardware does not benefit from the virtio driver infrastructure.
* `status` - Status indicates the status of the object.
* `usage_in_minutes_memory` - Total minutes of memory used.
* `usage_in_minutes_cores` - Total minutes of cores used.
* `create_time` - Defines the date and time the object was initially created.
* `change_time` - Defines the date and time of the last object change.
* `current_price` - The price for the current period since the last bill.
