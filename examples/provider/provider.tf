terraform {
  required_providers {
    firebaseappdistribution = {
      source  = "cut0/firebaseappdistribution"
      version = "0.1.0"
    }
  }
}

provider "firebaseappdistribution" {
  project = "my-firebase-project"
}
