#!/bin/bash
set -e

echo "Starting ETL Worker..."
exec /app/etl-worker
