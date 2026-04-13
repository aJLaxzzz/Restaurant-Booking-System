#!/usr/bin/env bash
# Юнит-дым по каждому сервису (cmd/*): один подтест на сервис, длительность настраивается.
# Пример для отчёта: перенаправьте вывод в файл или скопируйте блок PASS из терминала.
set -euo pipefail
cd "$(dirname "$0")/../backend"
exec go test ./test/servicereport/... -count=1 -v "$@"
