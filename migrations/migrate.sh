#!/bin/bash
# Migration helper script for model-router database
# Usage: ./migrate.sh [up|down|status|backup]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Database configuration
DB_DIR="${HOME}/.model-router"
DB_FILE="${DB_DIR}/data.db"
MIGRATIONS_DIR="$(dirname "$0")"

# Create database directory if it doesn't exist
mkdir -p "${DB_DIR}"

# Function to print colored messages
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to create backup
backup_database() {
    if [ -f "${DB_FILE}" ]; then
        local backup_file="${DB_FILE}.backup.$(date +%Y%m%d_%H%M%S)"
        print_info "Creating backup: ${backup_file}"
        cp "${DB_FILE}" "${backup_file}"
        print_info "Backup created successfully"
        return 0
    else
        print_warn "No existing database to backup"
        return 1
    fi
}

# Function to run up migration
run_up() {
    print_info "Running UP migration..."

    # Check if migration file exists
    local up_file="${MIGRATIONS_DIR}/001_destroy_rebuild.up.sql"
    if [ ! -f "${up_file}" ]; then
        print_error "Migration file not found: ${up_file}"
        exit 1
    fi

    # Backup database first
    backup_database

    # Run migration
    print_info "Applying migration: ${up_file}"
    sqlite3 "${DB_FILE}" < "${up_file}"

    if [ $? -eq 0 ]; then
        print_info "Migration completed successfully!"
        print_warn "⚠️  This was a DESTRUCTIVE migration - all previous data has been deleted"
    else
        print_error "Migration failed!"
        exit 1
    fi
}

# Function to run down migration
run_down() {
    print_info "Running DOWN migration (rollback)..."

    local down_file="${MIGRATIONS_DIR}/001_destroy_rebuild.down.sql"
    if [ ! -f "${down_file}" ]; then
        print_error "Rollback file not found: ${down_file}"
        exit 1
    fi

    print_warn "⚠️  This is a DESTRUCTIVE migration - rollback is a NO-OP"
    print_warn "Data cannot be recovered. Restore from backup if needed."

    read -p "Continue anyway? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Rollback cancelled"
        exit 0
    fi

    sqlite3 "${DB_FILE}" < "${down_file}"
    print_info "Rollback script executed (NO-OP for destructive migration)"
}

# Function to check migration status
check_status() {
    print_info "Database status:"

    if [ ! -f "${DB_FILE}" ]; then
        print_warn "Database does not exist yet"
        return 0
    fi

    echo ""
    echo "Database location: ${DB_FILE}"
    echo "Database size: $(du -h "${DB_FILE}" | cut -f1)"
    echo ""

    print_info "Tables in database:"
    sqlite3 "${DB_FILE}" "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name;" | sed 's/^/  - /'

    echo ""
    print_info "Row counts:"
    sqlite3 "${DB_FILE}" "
    SELECT
        name || ': ' || (SELECT COUNT(*) FROM pragma_table_info(name) || ' rows') as info
    FROM sqlite_master
    WHERE type='table'
    ORDER BY name;
    " | sed 's/^/  - /'
}

# Function to show help
show_help() {
    cat << EOF
Model Router Database Migration Helper

Usage: $0 [COMMAND]

Commands:
    up      Run forward migration (creates/recreates schema)
    down    Run rollback migration (NO-OP for destructive migration)
    status  Show database status and table information
    backup  Create a timestamped backup of the database
    help    Show this help message

Examples:
    $0 up          # Apply migration
    $0 status      # Check database status
    $0 backup      # Create backup

Database location: ${DB_FILE}
Migrations directory: ${MIGRATIONS_DIR}

⚠️  WARNING: Migration 001 is DESTRUCTIVE and will DELETE ALL DATA
EOF
}

# Main script logic
case "${1:-}" in
    up)
        run_up
        ;;
    down)
        run_down
        ;;
    status)
        check_status
        ;;
    backup)
        backup_database
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        print_error "Unknown command: ${1:-}"
        echo ""
        show_help
        exit 1
        ;;
esac
