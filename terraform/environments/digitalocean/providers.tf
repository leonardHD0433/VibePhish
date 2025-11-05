terraform {
  required_version = ">= 1.0"

  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 2.43"
    }
  }
}

provider "digitalocean" {
  token = var.do_token

  # Optional: Use DigitalOcean Spaces for remote state (recommended for production)
  # spaces_access_id  = var.spaces_access_key_id
  # spaces_secret_key = var.spaces_secret_access_key
}
