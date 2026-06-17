#!/usr/bin/env sh
set -eu

make test

if [ -d web/node_modules ]; then
  (cd web && npm run typecheck)
else
  echo "web/node_modules missing; run make web-build to install frontend dependencies"
fi
