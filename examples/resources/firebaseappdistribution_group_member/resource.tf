locals {
  qa_testers = [
    "alice@example.com",
    "bob@example.com",
  ]
}

resource "firebaseappdistribution_group" "qa" {
  group_id     = "qa"
  display_name = "QA team"
}

resource "firebaseappdistribution_group_member" "qa" {
  for_each = toset(local.qa_testers)

  group_id = firebaseappdistribution_group.qa.group_id
  email    = each.value
}
