enum "priority" {
}
schema "public" {

}
  }
    columns = [column.expires_at]
  index "idx_sessions_expires_at" {
  }
    columns = [column.user_id]
  index "idx_sessions_user_id" {
  }
    on_delete   = CASCADE
    ref_columns = [table.users.column.id]
    columns     = [column.user_id]
  foreign_key "sessions_user_id_fkey" {
  }
    columns = [column.id]
  primary_key {
  }
    default = sql("CURRENT_TIMESTAMP")
    type = timestamptz
    null = true
  column "created_at" {
  }
    type = timestamptz
    null = false
  column "expires_at" {
  }
    type = text
    null = false
  column "token_hash" {
  }
    type = uuid
    null = false
  column "user_id" {
  }
    default = sql("gen_random_uuid()")
    type = uuid
    null = false
  column "id" {
  schema = public
table "sessions" {

}
  }
    columns = [column.service_name]
  index "idx_integrations_service_name" {
  }
    columns = [column.user_id]
  index "idx_integrations_user_id" {
  }
    columns = [column.user_id, column.service_name]
  unique "unique_user_service" {
  }
    on_delete   = CASCADE
    ref_columns = [table.users.column.id]
    columns     = [column.user_id]
  foreign_key "integrations_user_id_fkey" {
  }
    columns = [column.id]
  primary_key {
  }
    default = sql("CURRENT_TIMESTAMP")
    type = timestamptz
    null = true
  column "updated_at" {
  }
    default = sql("CURRENT_TIMESTAMP")
    type = timestamptz
    null = true
  column "created_at" {
  }
    type = timestamptz
    null = true
  column "last_sync" {
  }
    default = "active"
    type = varchar(20)
    null = true
  column "status" {
  }
    type = text
    null = false
  column "api_key_encrypted" {
  }
    type = varchar(100)
    null = false
  column "service_name" {
  }
    type = uuid
    null = false
  column "user_id" {
  }
    default = sql("gen_random_uuid()")
    type = uuid
    null = false
  column "id" {
  schema = public
table "integrations" {

}
  }
    columns = [column.latitude, column.longitude]
  index "idx_locations_coordinates" {
  }
    columns = [column.timestamp]
  index "idx_locations_timestamp" {
  }
    columns = [column.user_id]
  index "idx_locations_user_id" {
  }
    on_delete   = CASCADE
    ref_columns = [table.users.column.id]
    columns     = [column.user_id]
  foreign_key "locations_user_id_fkey" {
  }
    columns = [column.id]
  primary_key {
  }
    default = sql("CURRENT_TIMESTAMP")
    type = timestamptz
    null = true
  column "created_at" {
  }
    type = timestamptz
    null = false
  column "timestamp" {
  }
    type = text
    null = true
  column "address" {
  }
    type = decimal(11, 8)
    null = false
  column "longitude" {
  }
    type = decimal(10, 8)
    null = false
  column "latitude" {
  }
    type = uuid
    null = false
  column "user_id" {
  }
    default = sql("gen_random_uuid()")
    type = uuid
    null = false
  column "id" {
  schema = public
table "locations" {

}
  }
    columns = [column.priority]
  index "idx_tasks_priority" {
  }
    columns = [column.completed]
  index "idx_tasks_completed" {
  }
    columns = [column.due_date]
  index "idx_tasks_due_date" {
  }
    columns = [column.user_id]
  index "idx_tasks_user_id" {
  }
    on_delete   = CASCADE
    ref_columns = [table.users.column.id]
    columns     = [column.user_id]
  foreign_key "tasks_user_id_fkey" {
  }
    columns = [column.id]
  primary_key {
  }
    default = sql("CURRENT_TIMESTAMP")
    type = timestamptz
    null = true
  column "updated_at" {
  }
    default = sql("CURRENT_TIMESTAMP")
    type = timestamptz
    null = true
  column "created_at" {
  }
    // For now, keeping it varchar with check logic if needed, but Atlas HCL typically uses native enums.
    // or we can use the enum type if Postgres supports it natively and Atlas supports it.
    // Enum validation is usually handled by check constraint in raw SQL,
    type = varchar(20)
    null = true
  column "priority" {
  }
    default = false
    type = boolean
    null = true
  column "completed" {
  }
    type = timestamptz
    null = true
  column "due_date" {
  }
    type = text
    null = true
  column "description" {
  }
    type = varchar(255)
    null = false
  column "title" {
  }
    type = sql("text[]")
    null = true
  column "hashtag" {
  }
    type = uuid
    null = false
  column "user_id" {
  }
    default = sql("gen_random_uuid()")
    type = uuid
    null = false
  column "id" {
  schema = public
table "tasks" {

}
  }
    columns = [column.end_time]
  index "idx_events_end_time" {
  }
    columns = [column.start_time]
  index "idx_events_start_time" {
  }
    columns = [column.user_id]
  index "idx_events_user_id" {
  }
    on_delete   = CASCADE
    ref_columns = [table.users.column.id]
    columns     = [column.user_id]
  foreign_key "events_user_id_fkey" {
  }
    columns = [column.id]
  primary_key {
  }
    default = sql("CURRENT_TIMESTAMP")
    type = timestamptz
    null = true
  column "updated_at" {
  }
    default = sql("CURRENT_TIMESTAMP")
    type = timestamptz
    null = true
  column "created_at" {
  }
    type = varchar(255)
    null = true
  column "location" {
  }
    type = timestamptz
    null = false
  column "end_time" {
  }
    type = timestamptz
    null = false
  column "start_time" {
  }
    type = text
    null = true
  column "description" {
  }
    type = varchar(255)
    null = false
  column "title" {
  }
    type = sql("text[]")
    null = true
  column "hashtag" {
  }
    type = uuid
    null = false
  column "user_id" {
  }
    default = sql("gen_random_uuid()")
    type = uuid
    null = false
  column "id" {
  schema = public
table "events" {

}
  }
    columns = [column.email_verified]
  index "idx_users_email_verified" {
  }
    unique = true
    columns = [column.email]
  index "idx_users_email" {
  }
    columns = [column.id]
  primary_key {
  }
    default = sql("CURRENT_TIMESTAMP")
    type = timestamptz
    null = true
  column "updated_at" {
  }
    default = sql("CURRENT_TIMESTAMP")
    type = timestamptz
    null = true
  column "created_at" {
  }
    default = false
    type = boolean
    null = false
  column "email_verified" {
  }
    type = varchar(100)
    null = true
  column "last_name" {
  }
    type = varchar(100)
    null = true
  column "first_name" {
  }
    type = text
    null = false
  column "password_hash" {
  }
    type = varchar(255)
    null = false
  column "email" {
  }
    default = sql("gen_random_uuid()")
    type = uuid
    null = false
  column "id" {
  schema = public
table "users" {

}
  values = ["active", "inactive", "error"]
enum "integration_status" {

}
  values = ["low", "medium", "high", "urgent"]

