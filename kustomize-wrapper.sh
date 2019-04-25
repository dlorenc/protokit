#!/bin/bash
set -e

for environment in staging prod; do
    gsutil cp gs://protokit-secrets/$environment/mysql-secret.env ./config/$environment/
done

# Execute remaining command
"$@"

# Cleanup

for environment in staging prod; do
    rm ./config/$environment/mysql-secret.env
done