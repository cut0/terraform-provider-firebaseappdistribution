data "firebaseappdistribution_group" "qa" {
  group_id = "qa"
}

output "qa_tester_count" {
  value = data.firebaseappdistribution_group.qa.tester_count
}
