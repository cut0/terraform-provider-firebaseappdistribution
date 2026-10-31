# Terraform Provider for Firebase App Distribution

Manage [Firebase App Distribution](https://firebase.google.com/docs/app-distribution) tester groups and their members with Terraform.

The official Google providers cover Firebase projects and apps but not App Distribution. This provider fills that gap through the [App Distribution REST API](https://firebase.google.com/docs/reference/app-distribution/rest).

## Resources

| Name | Description |
| --- | --- |
| `firebaseappdistribution_group` | A tester group (`projects/{project}/groups/{alias}`) |
| `firebaseappdistribution_group_member` | Membership of one tester in a group |

## Data sources

| Name | Description |
| --- | --- |
| `firebaseappdistribution_group` | Reads an existing group by alias |

Groups and testers are scoped to a Firebase project, not to an individual app. Which app releases are distributed to a group is decided when a release is distributed, for example by the Firebase CLI or the `releases:distribute` API in your delivery pipeline.

## Usage

```hcl
terraform {
  required_providers {
    firebaseappdistribution = {
      source  = "cut0/firebaseappdistribution"
      version = "~> 0.1"
    }
  }
}

provider "firebaseappdistribution" {
  project = "my-firebase-project"
}

resource "firebaseappdistribution_group" "qa" {
  group_id     = "qa"
  display_name = "QA team"
}

resource "firebaseappdistribution_group_member" "qa" {
  for_each = toset(["alice@example.com", "bob@example.com"])

  group_id = firebaseappdistribution_group.qa.group_id
  email    = each.value
}
```

Existing groups can be imported:

```sh
terraform import firebaseappdistribution_group.qa projects/my-firebase-project/groups/qa
terraform import 'firebaseappdistribution_group_member.qa["alice@example.com"]' projects/my-firebase-project/groups/qa/testers/alice@example.com
```

See the [documentation](docs/) for all arguments and attributes.

## Authentication

The provider uses [Application Default Credentials](https://cloud.google.com/docs/authentication/application-default-credentials), so it works out of the box with `gcloud auth application-default login` locally and with Workload Identity Federation in CI. No additional secrets are required. A service account key file can still be used by pointing `GOOGLE_APPLICATION_CREDENTIALS` at it, although short-lived credentials are preferred.

| Argument | Environment variable | Description |
| --- | --- | --- |
| `project` | `FIREBASE_PROJECT`, `GOOGLE_PROJECT` | Default Firebase project ID |
| `access_token` | `GOOGLE_OAUTH_ACCESS_TOKEN` | Use a pre-issued OAuth 2.0 access token instead of Application Default Credentials |
| `impersonate_service_account` | `GOOGLE_IMPERSONATE_SERVICE_ACCOUNT` | Impersonate a service account for all API calls |
| `billing_project` | `GOOGLE_BILLING_PROJECT` | Quota project sent as `x-goog-user-project` |

Projects can be referenced by ID or by number. The App Distribution API requires project numbers for write operations, so IDs are resolved through the [Firebase Management API](https://firebase.google.com/docs/reference/firebase-management/rest), which must be enabled on the project.

The identity needs `firebase.projects.get` plus the `firebaseappdistro.groups.*` and `firebaseappdistro.testers.*` permissions on the project. `roles/editor` grants all of them; otherwise combine `roles/firebase.viewer` with `roles/firebaseappdistro.admin`.

## Development

Requirements: [Go](https://go.dev/doc/install) 1.26 or later and [Terraform](https://developer.hashicorp.com/terraform/install) 1.5 or later.

```sh
make build      # compile
make test       # unit tests
make lint       # golangci-lint
make generate   # regenerate docs/ from the schema and examples/
```

Acceptance tests create and delete real groups in a Firebase project:

```sh
export FIREBASE_PROJECT=my-firebase-project
make testacc
```

Set `TF_ACC_TESTER_EMAIL` to join an existing tester instead of creating a throwaway `@example.com` tester that is removed afterwards.

To use a local build from Terraform, add a [development override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers) to your `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "cut0/firebaseappdistribution" = "/path/to/go/bin"
  }
  direct {}
}
```

## Releasing

Releases are built by GoReleaser from tags of the form `vX.Y.Z` and published to the Terraform Registry. The workflow expects the `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` repository secrets for signing the checksums.

## License

[Mozilla Public License 2.0](LICENSE)
