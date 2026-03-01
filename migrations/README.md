# Database Migrations

This directory contains SQL migration scripts for the model-router database.

## ⚠️ WARNING

**Migration 001 is DESTRUCTIVE and will DELETE ALL DATA.**
Always backup your database before running migrations!

## Migration Files

### 001_destroy_rebuild
- **up.sql**: Drops all existing tables and recreates the complete schema
- **down.sql**: Rollback script (NO-OP - data cannot be recovered after destructive migration)

## Database Location

Default database location: `~/.model-router/data.db`

## Usage

### Manual Migration (SQLite CLI)

```bash
# Backup database first!
cp ~/.model-router/data.db ~/.model-router/data.db.backup.$(date +%Y%m%d_%H%M%S)

# Run up migration
sqlite3 ~/.model-router/data.db < migrations/001_destroy_rebuild.up.sql

# Run down migration (rollback) - NO-OP for destructive migration
sqlite3 ~/.model-router/data.db < migrations/001_destroy_rebuild.down.sql
```

### Using the Helper Script

```bash
# Run up migration
./migrations/migrate.sh up

# Run down migration
./migrations/migrate.sh down

# Check migration status
./migrations/migrate.sh status
```

## Schema Overview

### Core Tables
- **settings**: System configuration key-value store
- **profiles**: Routing configuration profiles
- **providers**: Model provider configurations
- **models**: Individual model configurations

### Advanced Tables
- **compression_model_groups**: Named groups for compression tasks
- **composite_auto_models**: Composite models with auto-routing
- **route_rules**: Routing rules for model selection

### Session Tables
- **sessions**: Long-term session contexts
- **session_messages**: Messages within sessions

### Analytics Tables
- **request_logs**: API request logs
- **stats**: Aggregated statistics
- **test_results**: Model health check results

### Security Tables
- **api_keys**: Client API key management

## Indexes

The schema includes indexes for:
- Profile lookups by name, path, and enabled status
- Provider lookups by name, type, and enabled status
- Model lookups by profile, provider, name, and enabled status
- Compression group composite indexes (profile_id, name)
- Composite auto model composite indexes (profile_id, name)
- Request log queries by model, provider, status, and time
- Session lookups by API key, profile, and session key
- Stats aggregation by date, hour, provider, and model

## Default Settings

After migration, these default settings are created:
- `admin_token`: admin-token-change-me
- `jwt_secret`: jwt-secret-change-me
- `log_retention_days`: 30
- `max_request_size_mb`: 10
- `enable_stats`: true
- `stats_retention_days`: 90
- `default_profile`: default
- `language`: en

## Next Steps After Migration

1. **Change default credentials**: Update admin_token and jwt_secret
2. **Create providers**: Add your model providers via API or admin UI
3. **Create profiles**: Set up routing profiles
4. **Create models**: Configure individual models
5. **Test routing**: Verify routes work correctly

## Backup Strategy

Always backup before migrations:

```bash
# Timestamped backup
cp ~/.model-router/data.db ~/.model-router/data.db.backup.$(date +%Y%m%d_%H%M%S)

# Or use the helper script
./migrations/backup.sh
```

## Migration Best Practices

1. **Always backup first** - Migrations can fail
2. **Test on staging** - Never run untested migrations in production
3. **Use transactions** - Wrap migrations in transactions when possible
4. **Version control** - Keep migrations in git
5. **Document breaking changes** - Note any API or schema changes
6. **Plan rollbacks** - Have a rollback strategy ready

## Troubleshooting

### Migration fails partway through
- Restore from backup and investigate the error
- Check SQLite version compatibility: `sqlite3 --version`
- Verify SQL syntax in the migration file

### Foreign key errors
- Ensure tables are dropped in correct order (dependencies first)
- Check ON DELETE CASCADE settings

### Permission errors
- Verify write permissions on database file and directory
- Check disk space available

## Future Migrations

Future migrations should follow this naming convention:
```
XXX_description.{up,down}.sql
```

Where:
- `XXX`: Sequential migration number (001, 002, 003, ...)
- `description`: Short description of migration purpose
- `up.sql`: Forward migration SQL
- `down.sql`: Rollback SQL (when possible)

Example non-destructive migration:
```
002_add_user_preferences.up.sql
002_add_user_preferences.down.sql
```
