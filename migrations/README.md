# Database Migrations

This directory contains SQL migration files for the Orbit Core database schema.

## Migration Files

Migrations are numbered sequentially and should be applied in order:

1. `001_initial_schema.sql` - Creates the initial database schema

## Running Migrations

### Manual Method

Connect to your PostgreSQL database and run the migration files:

```bash
psql -U postgres -d orbit -f migrations/001_initial_schema.sql
```

### Using Docker

If using Docker Compose, migrations can be run from within the PostgreSQL container:

```bash
docker-compose exec postgres psql -U postgres -d orbit -f /migrations/001_initial_schema.sql
```

## Migration Tools (Future)

Consider using a migration tool like:
- [golang-migrate](https://github.com/golang-migrate/migrate)
- [goose](https://github.com/pressly/goose)
- [sql-migrate](https://github.com/rubenv/sql-migrate)

## Creating New Migrations

When creating new migration files:

1. Use sequential numbering: `00X_description.sql`
2. Include both UP and DOWN migrations if possible
3. Test migrations in a development environment first
4. Document any breaking changes

Example:
```sql
-- Migration: 002_add_user_preferences.sql
-- Description: Add user preferences table

CREATE TABLE IF NOT EXISTS user_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    theme VARCHAR(20) DEFAULT 'light',
    language VARCHAR(10) DEFAULT 'en',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

## Rollback

To rollback a migration, you'll need to manually execute the reverse operations. Document these in comments within the migration file.
