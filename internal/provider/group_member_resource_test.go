package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"google.golang.org/api/firebaseappdistribution/v1"
)

func TestAccGroupMemberResource(t *testing.T) {
	groupID := acctest.RandomWithPrefix("tf-acc")
	address := "firebaseappdistribution_group_member.test"

	email := os.Getenv("TF_ACC_TESTER_EMAIL")
	removeTester := email == ""
	if removeTester {
		email = acctest.RandomWithPrefix("tf-acc") + "@example.com"
	}

	idRegexp := regexp.MustCompile(`^projects/[0-9]+/groups/` + regexp.QuoteMeta(groupID) + `/testers/` + regexp.QuoteMeta(email) + `$`)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTesterRemoved(t, email, removeTester),
		Steps: []resource.TestStep{
			{
				Config: testAccGroupMemberResourceConfig(groupID, email),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(address, tfjsonpath.New("id"), knownvalue.StringRegexp(idRegexp)),
					statecheck.ExpectKnownValue(address, tfjsonpath.New("project"), knownvalue.StringExact(testAccProject())),
					statecheck.ExpectKnownValue(address, tfjsonpath.New("group_id"), knownvalue.StringExact(groupID)),
					statecheck.ExpectKnownValue(address, tfjsonpath.New("email"), knownvalue.StringExact(email)),
					statecheck.ExpectKnownValue(address, tfjsonpath.New("create_missing_tester"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue(address, tfjsonpath.New("tester_name"), knownvalue.StringRegexp(regexp.MustCompile(`^projects/[0-9]+/testers/`))),
				},
			},
			{
				ResourceName:            address,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"create_missing_tester"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					attributes := s.RootModule().Resources[address].Primary.Attributes
					return groupMemberID(attributes["project"], attributes["group_id"], attributes["email"]), nil
				},
			},
		},
	})
}

func testAccGroupMemberResourceConfig(groupID, email string) string {
	return fmt.Sprintf(`
resource "firebaseappdistribution_group" "test" {
  group_id     = %q
  display_name = "Terraform acceptance test"
}

resource "firebaseappdistribution_group_member" "test" {
  group_id = firebaseappdistribution_group.test.group_id
  email    = %q
}
`, groupID, email)
}

func testAccCheckTesterRemoved(t *testing.T, email string, remove bool) resource.TestCheckFunc {
	t.Helper()

	return func(_ *terraform.State) error {
		if !remove {
			return nil
		}

		ctx := context.Background()
		apiClient, err := newClient(ctx, clientConfig{
			project:        testAccProject(),
			billingProject: os.Getenv("GOOGLE_BILLING_PROJECT"),
			userAgent:      "terraform-provider-firebaseappdistribution/test",
		})
		if err != nil {
			return err
		}

		projectNumber, err := apiClient.projectNumber(ctx, testAccProject())
		if err != nil {
			return err
		}

		_, err = apiClient.appDistribution.Projects.Testers.BatchRemove(projectParent(projectNumber), &firebaseappdistribution.GoogleFirebaseAppdistroV1BatchRemoveTestersRequest{
			Emails: []string{email},
		}).Context(ctx).Do()
		return err
	}
}
