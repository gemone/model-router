#!/bin/bash
# Simple backup script for model-router database
# Usage: ./backup.sh [optional_output_directory]

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Configuration
DB_DIR="${HOME}/.model-router"
DB_FILE="${DB_DIR}/data.db"
BACKUP_DIR="${1:-${DB_DIR}/backups}"

# Create backup directory
mkdir -p "${BACKUP_DIR}"

# Check if database exists
if [ ! -f "${DB_FILE}" ]; then
    echo -e "${YELLOW}[WARN]${NC} Database file not found: ${DB_FILE}"
    echo -e "${YELLOW}[WARN]${NC} No backup created"
    exit 0
fi

# Create timestamped backup
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/data.db.backup.${TIMESTAMP}"

echo -e "${GREEN}[INFO]${NC} Creating backup..."
cp "${DB_FILE}" "${BACKUP_FILE}"

if [ $? -eq 0 ]; then
    BACKUP_SIZE=$(du -h "${BACKUP_FILE}" | cut -f1)
    echo -e "${GREEN}[INFO]${NC} Backup created successfully!"
    echo -e "${GREEN}[INFO]${NC} Location: ${BACKUP_FILE}"
    echo -e "${GREEN}[INFO]${NC} Size: ${BACKUP_SIZE}"

    # Clean old backups (keep last 10)
    cd "${BACKUP_DIR}"
    ls -t data.db.backup.* 2>/dev/null | tail -n +11 | xargs -r rm --
    OLD_BACKUPS=$(ls data.db.backup.* 2>/dev/null | wc -l)
    echo -e "${GREEN}[INFO]${NC} Retaining ${OLD_BACKUPS} recent backups"
else
    echo -e "${RED}[ERROR]${NC} Backup failed!"
    exit 1
fi
