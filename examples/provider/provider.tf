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
