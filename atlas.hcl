env "local" {
  src = "file://schema.hcl"
  dev = "docker://postgres/15/dev"
  migration {
    dir = "file://migrations"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

variable "db_url" {
  type    = string
  # Since you use POSTGRES_DB: orbit and trust auth:
  default = "postgres://postgres@postgres:5432/orbit?sslmode=disable"
}

env "docker" {
  # Point to the schema file and migrations directory inside the container
  src = "file://schema.hcl"

  # The connection string for your 'orbit' database
  url = var.db_url

  # Migrations directory (mounted from the repo)
  migration {
    dir = "file://migrations"
  }

  # Atlas uses a 'dev' database to validate your schema.
  dev = "docker://postgres/15/dev"
}