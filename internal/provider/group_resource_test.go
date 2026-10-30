package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccGroupResource(t *testing.T) {
	groupID := acctest.RandomWithPrefix("tf-acc")
	address := "firebaseappdistribution_group.test"
	nameRegexp := regexp.MustCompile(`^projects/[0-9]+/groups/` + regexp.QuoteMeta(groupID) + `$`)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGroupResourceConfig(groupID, "Terraform acceptance test"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(address, tfjsonpath.New("id"), knownvalue.StringRegexp(nameRegexp)),
					statecheck.ExpectKnownValue(address, tfjsonpath.New("name"), knownvalue.StringRegexp(nameRegexp)),
					statecheck.ExpectKnownValue(address, tfjsonpath.New("project"), knownvalue.StringExact(testAccProject())),
					statecheck.ExpectKnownValue(address, tfjsonpath.New("group_id"), knownvalue.StringExact(groupID)),
					statecheck.ExpectKnownValue(address, tfjsonpath.New("display_name"), knownvalue.StringExact("Terraform acceptance test")),
					statecheck.ExpectKnownValue(address, tfjsonpath.New("tester_count"), knownvalue.Int64Exact(0)),
				},
			},
			{
				ResourceName:      address,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					attributes := s.RootModule().Resources[address].Primary.Attributes
					return groupName(attributes["project"], attributes["group_id"]), nil
				},
			},
			{
				Config: testAccGroupResourceConfig(groupID, "Terraform acceptance test (updated)"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(address, tfjsonpath.New("display_name"), knownvalue.StringExact("Terraform acceptance test (updated)")),
				},
			},
		},
	})
}

func testAccGroupResourceConfig(groupID, displayName string) string {
	return fmt.Sprintf(`
resource "firebaseappdistribution_group" "test" {
  group_id     = %q
  display_name = %q
}
`, groupID, displayName)
}
