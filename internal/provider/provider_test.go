package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"firebaseappdistribution": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	t.Helper()

	if testAccProject() == "" {
		t.Fatal("FIREBASE_PROJECT or GOOGLE_PROJECT must be set for acceptance tests")
	}
}

func testAccProject() string {
	return firstNonEmpty(os.Getenv("FIREBASE_PROJECT"), os.Getenv("GOOGLE_PROJECT"))
}
