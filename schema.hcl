enum "integration_status" {
  values = ["active", "inactive", "error"]
}

enum "habit_status" {
  values = ["pending", "accepted", "rejected", "expired"]
}

enum "subscription_status" {
  values = ["pending", "sent", "cancelled", "failed"]
}

enum "priority" {
  values = ["low", "medium", "high", "urgent"]
}

schema "public" {

  table "users" {
    schema = public
    column "id" {
      type = uuid
      null = false
      default = sql("gen_random_uuid()")
    }
    column "email" {
      type = varchar(255)
      null = false
    }
    column "password_hash" {
      type = text
      null = false
    }
    column "first_name" {
      type = varchar(100)
      null = true
    }
    column "last_name" {
      type = varchar(100)
      null = true
    }
    column "email_verified" {
      type = boolean
      null = false
      default = false
    }
    column "username" {
      type = varchar(50)
      null = true
    }
    column "profile_picture" {
      type = text
      null = true
    }
    column "region" {
      type = varchar(100)
      null = true
    }
    column "timezone" {
      type = varchar(100)
      null = true
    }
    column "gender" {
      type = varchar(50)
      null = true
    }
    column "birth_date" {
      type = date
      null = true
    }
    column "created_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }
    column "updated_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }

    primary_key {
      columns = [column.id]
    }

    index "idx_users_email" {
      unique = true
      columns = [column.email]
    }

    index "idx_users_email_verified" {
      columns = [column.email_verified]
    }

    index "idx_users_username" {
      columns = [column.username]
    }

    index "idx_users_birth_date" {
      columns = [column.birth_date]
    }

    unique "unique_username" {
      columns = [column.username]
    }
  }

  table "events" {
    schema = public
    column "id" {
      type = uuid
      null = false
      default = sql("gen_random_uuid()")
    }
    column "user_id" {
      type = uuid
      null = false
    }
    column "title" {
      type = varchar(255)
      null = false
    }
    column "description" {
      type = text
      null = true
    }
    column "start_time" {
      type = timestamptz
      null = false
    }
    column "end_time" {
      type = timestamptz
      null = false
    }
    column "location" {
      type = varchar(255)
      null = true
    }
    column "is_recurring" {
      type = boolean
      null = true
      default = false
    }
    column "recurrence_rule" {
      type = text
      null = true
    }
    column "recurrence_exception" {
      type = text
      null = true
    }
    column "hashtags" {
      type = sql("text[]")
      null = true
      default = sql("'{}'::text[]")
    }
    column "created_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }
    column "updated_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }

    primary_key {
      columns = [column.id]
    }

    foreign_key "events_user_id_fkey" {
      columns = [column.user_id]
      ref_columns = [table.users.column.id]
      on_delete = CASCADE
    }

    index "idx_events_user_id" {
      columns = [column.user_id]
    }

    index "idx_events_start_time" {
      columns = [column.start_time]
    }

    index "idx_events_end_time" {
      columns = [column.end_time]
    }

    index "idx_events_user_time" {
      columns = [column.user_id, column.start_time, column.end_time]
    }

    index "idx_events_user_recurring" {
      columns = [column.user_id, column.start_time]
    }
  }

  table "tasks" {
    schema = public
    column "id" {
      type = uuid
      null = false
      default = sql("gen_random_uuid()")
    }
    column "user_id" {
      type = uuid
      null = false
    }
    column "title" {
      type = varchar(255)
      null = false
    }
    column "description" {
      type = text
      null = true
    }
    column "due_date" {
      type = timestamptz
      null = true
    }
    column "completed" {
      type = boolean
      null = true
      default = false
    }
    column "priority" {
      type = varchar(20)
      null = true
    }
    column "hashtags" {
      type = sql("text[]")
      null = true
      default = sql("'{}'::text[]")
    }
    column "created_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }
    column "updated_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }

    primary_key {
      columns = [column.id]
    }

    foreign_key "tasks_user_id_fkey" {
      columns = [column.user_id]
      ref_columns = [table.users.column.id]
      on_delete = CASCADE
    }

    index "idx_tasks_user_id" {
      columns = [column.user_id]
    }

    index "idx_tasks_due_date" {
      columns = [column.due_date]
    }

    index "idx_tasks_completed" {
      columns = [column.completed]
    }

    index "idx_tasks_priority" {
      columns = [column.priority]
    }
  }

  table "locations" {
    schema = public
    column "id" {
      type = uuid
      null = false
      default = sql("gen_random_uuid()")
    }
    column "user_id" {
      type = uuid
      null = false
    }
    column "latitude" {
      type = decimal(10, 8)
      null = false
    }
    column "longitude" {
      type = decimal(11, 8)
      null = false
    }
    column "address" {
      type = text
      null = true
    }
    column "timestamp" {
      type = timestamptz
      null = false
    }
    column "created_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }

    primary_key {
      columns = [column.id]
    }

    foreign_key "locations_user_id_fkey" {
      columns = [column.user_id]
      ref_columns = [table.users.column.id]
      on_delete = CASCADE
    }

    index "idx_locations_user_id" {
      columns = [column.user_id]
    }

    index "idx_locations_timestamp" {
      columns = [column.timestamp]
    }

    index "idx_locations_coordinates" {
      columns = [column.latitude, column.longitude]
    }
  }

  table "integrations" {
    schema = public
    column "id" {
      type = uuid
      null = false
      default = sql("gen_random_uuid()")
    }
    column "user_id" {
      type = uuid
      null = false
    }
    column "service_name" {
      type = varchar(100)
      null = false
    }
    column "api_key_encrypted" {
      type = text
      null = false
    }
    column "status" {
      type = varchar(20)
      null = true
      default = "active"
    }
    column "last_sync" {
      type = timestamptz
      null = true
    }
    column "created_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }
    column "updated_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }

    primary_key {
      columns = [column.id]
    }

    foreign_key "integrations_user_id_fkey" {
      columns = [column.user_id]
      ref_columns = [table.users.column.id]
      on_delete = CASCADE
    }

    index "idx_integrations_user_id" {
      columns = [column.user_id]
    }

    index "idx_integrations_service_name" {
      columns = [column.service_name]
    }

    unique "unique_user_service" {
      columns = [column.user_id, column.service_name]
    }
  }

  table "sessions" {
    schema = public
    column "id" {
      type = uuid
      null = false
      default = sql("gen_random_uuid()")
    }
    column "user_id" {
      type = uuid
      null = false
    }
    column "token_hash" {
      type = text
      null = false
    }
    column "expires_at" {
      type = timestamptz
      null = false
    }
    column "created_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }

    primary_key {
      columns = [column.id]
    }

    foreign_key "sessions_user_id_fkey" {
      columns = [column.user_id]
      ref_columns = [table.users.column.id]
      on_delete = CASCADE
    }

    index "idx_sessions_user_id" {
      columns = [column.user_id]
    }

    index "idx_sessions_expires_at" {
      columns = [column.expires_at]
    }
  }

  table "event_frequency" {
    schema = public
    column "id" {
      type = uuid
      null = false
      default = sql("gen_random_uuid()")
    }
    column "user_id" {
      type = uuid
      null = false
    }
    column "title" {
      type = varchar(255)
      null = false
    }
    column "description" {
      type = text
      null = true
    }
    column "location" {
      type = varchar(255)
      null = true
    }
    column "duration_minutes" {
      type = integer
      null = false
    }
    column "time_of_day" {
      type = integer
      null = false
    }
    column "day_of_week" {
      type = integer
      null = false
    }
    column "occurrence_count" {
      type = integer
      null = false
      default = 1
    }
    column "suggestion_threshold" {
      type = integer
      null = false
      default = 3
    }
    column "suggestion_shown" {
      type = boolean
      null = false
      default = false
    }
    column "habit_accepted" {
      type = boolean
      null = false
      default = false
    }
    column "occurrence_timestamps" {
      type = jsonb
      null = true
      default = sql("'[]'::jsonb")
    }
    column "created_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }
    column "updated_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }

    primary_key {
      columns = [column.id]
    }

    foreign_key "event_frequency_user_id_fkey" {
      columns = [column.user_id]
      ref_columns = [table.users.column.id]
      on_delete = CASCADE
    }

    index "idx_event_frequency_user_id" {
      columns = [column.user_id]
    }

    index "idx_event_frequency_occurrence_count" {
      columns = [column.occurrence_count]
    }

    index "idx_event_frequency_suggestion_shown" {
      columns = [column.suggestion_shown]
    }

    unique "unique_event_frequency_pattern" {
      columns = [column.user_id, column.title, column.duration_minutes, column.time_of_day, column.day_of_week]
    }
  }

  table "habit_suggestions" {
    schema = public
    column "id" {
      type = uuid
      null = false
      default = sql("gen_random_uuid()")
    }
    column "user_id" {
      type = uuid
      null = false
    }
    column "event_frequency_id" {
      type = uuid
      null = false
    }
    column "title" {
      type = varchar(255)
      null = false
    }
    column "description" {
      type = text
      null = true
    }
    column "location" {
      type = varchar(255)
      null = true
    }
    column "duration_minutes" {
      type = integer
      null = false
    }
    column "time_of_day" {
      type = integer
      null = false
    }
    column "day_of_week" {
      type = integer
      null = false
    }
    column "status" {
      type = varchar(20)
      null = true
      default = "pending"
    }
    column "recurrence_end_date" {
      type = timestamptz
      null = true
    }
    column "created_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }
    column "updated_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }
    column "expires_at" {
      type = timestamptz
      null = true
    }

    primary_key {
      columns = [column.id]
    }

    foreign_key "habit_suggestions_user_id_fkey" {
      columns = [column.user_id]
      ref_columns = [table.users.column.id]
      on_delete = CASCADE
    }

    foreign_key "habit_suggestions_event_frequency_id_fkey" {
      columns = [column.event_frequency_id]
      ref_columns = [table.event_frequency.column.id]
      on_delete = CASCADE
    }

    index "idx_habit_suggestions_user_id" {
      columns = [column.user_id]
    }

    index "idx_habit_suggestions_status" {
      columns = [column.status]
    }

    index "idx_habit_suggestions_event_frequency_id" {
      columns = [column.event_frequency_id]
    }
  }

  table "recurring_events" {
    schema = public
    column "id" {
      type = uuid
      null = false
      default = sql("gen_random_uuid()")
    }
    column "user_id" {
      type = uuid
      null = false
    }
    column "habit_suggestion_id" {
      type = uuid
      null = true
    }
    column "title" {
      type = varchar(255)
      null = false
    }
    column "description" {
      type = text
      null = true
    }
    column "location" {
      type = varchar(255)
      null = true
    }
    column "duration_minutes" {
      type = integer
      null = false
    }
    column "time_of_day" {
      type = integer
      null = false
    }
    column "day_of_week" {
      type = integer
      null = false
    }
    column "start_date" {
      type = timestamptz
      null = false
    }
    column "end_date" {
      type = timestamptz
      null = false
    }
    column "is_active" {
      type = boolean
      null = false
      default = true
    }
    column "created_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }
    column "updated_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }

    primary_key {
      columns = [column.id]
    }

    foreign_key "recurring_events_user_id_fkey" {
      columns = [column.user_id]
      ref_columns = [table.users.column.id]
      on_delete = CASCADE
    }

    foreign_key "recurring_events_habit_suggestion_id_fkey" {
      columns = [column.habit_suggestion_id]
      ref_columns = [table.habit_suggestions.column.id]
      on_delete = SET_NULL
    }

    index "idx_recurring_events_user_id" {
      columns = [column.user_id]
    }

    index "idx_recurring_events_is_active" {
      columns = [column.is_active]
    }
  }

  table "device_tokens" {
    schema = public
    column "id" {
      type = uuid
      null = false
      default = sql("gen_random_uuid()")
    }
    column "user_id" {
      type = uuid
      null = false
    }
    column "token" {
      type = varchar(512)
      null = false
    }
    column "platform" {
      type = varchar(10)
      null = false
    }
    column "updated_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }

    primary_key {
      columns = [column.id]
    }

    foreign_key "device_tokens_user_id_fkey" {
      columns = [column.user_id]
      ref_columns = [table.users.column.id]
      on_delete = CASCADE
    }

    index "idx_device_tokens_user_id" {
      columns = [column.user_id]
    }

    unique "device_tokens_token_unique" {
      columns = [column.token]
    }
  }

  table "event_subscriptions" {
    schema = public
    column "id" {
      type = uuid
      null = false
      default = sql("gen_random_uuid()")
    }
    column "user_id" {
      type = uuid
      null = false
    }
    column "event_id" {
      type = uuid
      null = false
    }
    column "trigger_time" {
      type = timestamptz
      null = false
    }
    column "is_sent" {
      type = boolean
      null = true
      default = false
    }
    column "job_id" {
      type = varchar(512)
      null = true
    }
    column "status" {
      type = varchar(20)
      null = false
      default = "pending"
    }
    column "created_at" {
      type = timestamptz
      null = true
      default = sql("CURRENT_TIMESTAMP")
    }

    primary_key {
      columns = [column.id]
    }

    foreign_key "event_subscriptions_user_id_fkey" {
      columns = [column.user_id]
      ref_columns = [table.users.column.id]
      on_delete = CASCADE
    }

    foreign_key "event_subscriptions_event_id_fkey" {
      columns = [column.event_id]
      ref_columns = [table.events.column.id]
      on_delete = CASCADE
    }

    index "idx_event_subscriptions_trigger_is_sent" {
      columns = [column.trigger_time, column.is_sent]
    }

    index "idx_event_subscriptions_user_id" {
      columns = [column.user_id]
    }

    index "idx_event_subscriptions_event_id" {
      columns = [column.event_id]
    }

    index "idx_event_subscriptions_job_id" {
      columns = [column.job_id]
    }

    index "idx_event_subscriptions_user_event_active" {
      columns = [column.user_id, column.event_id]
    }
  }
}
