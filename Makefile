GO          := $(shell which go)
AIR         := $(shell which air)
SWAG        := $(shell which swag)
PSQL        := $(shell which psql)

FILE_ENV    := ./.env

MIGRATIONS_DIR := db/migrations

.PHONY: run
run:
	set -a; . $(FILE_ENV); set +a; $(AIR)

.PHONY: api
api:
	set -a; . $(FILE_ENV); set +a; $(GO) run ./cmd/server

# Shell prelude for migrate and seed. It connects like the app: DATABASE_URL when set,
# otherwise the DB_* parts, all from $(FILE_ENV). It defines run_psql, which stops at the
# first failing statement.
PSQL_PRELUDE = set -a; . $(FILE_ENV); set +a; \
	export PGHOST="$$DB_HOST" PGPORT="$$DB_PORT" PGUSER="$$DB_USER" PGPASSWORD="$$DB_PASSWORD" PGDATABASE="$$DB_NAME"; \
	if [ -n "$$DB_SSLMODE" ]; then export PGSSLMODE="$$DB_SSLMODE"; fi; \
	export PGOPTIONS="-c client_min_messages=warning"; \
	run_psql() { $(PSQL) $${DATABASE_URL:+"$$DATABASE_URL"} -X -q -v ON_ERROR_STOP=1 "$$@"; }

# 000001-init.sql runs only on an empty database: on an older one it would index columns
# the dated migrations have not added yet. The dated migrations then run in name order;
# each is re-runnable, so the target is safe to run after every pull.
.PHONY: migrate
migrate:
	@$(PSQL_PRELUDE); \
	has_schema=$$(run_psql -Atc "SELECT to_regclass('public.leagues') IS NOT NULL") || exit 1; \
	if [ "$$has_schema" = "f" ]; then \
		echo "migrate: empty database, applying 000001-init.sql"; \
		run_psql -f $(MIGRATIONS_DIR)/000001-init.sql || exit 1; \
	fi; \
	for file in $(MIGRATIONS_DIR)/[0-9][0-9][0-9][0-9]/*.sql; do \
		echo "migrate: $$file"; \
		run_psql -f "$$file" || exit 1; \
	done; \
	echo "migrate: done"

# Loads the latest seed, the last file by name in seeds/; older seeds match older schemas.
# It replaces all league data with the dev data and keeps real accounts, so it also
# resets a dev database. It migrates first, because the seed needs the current schema.
.PHONY: seed
seed: migrate
	@$(PSQL_PRELUDE); \
	seed_file=$$(ls $(MIGRATIONS_DIR)/seeds/*.sql | sort | tail -n 1); \
	echo "seed: $$seed_file"; \
	run_psql -f "$$seed_file" || exit 1; \
	echo "seed: done"

.PHONY: swagger
swagger:
	$(SWAG) fmt
	$(SWAG) init --parseDependency --dir ./cmd/server,internal --output ./docs/swagger

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: mockgen
mockgen:
	$(GO) generate ./...

.PHONY: test
test: vet mockgen
	$(GO) test -v -count=1 -failfast ./...

.PHONY: coverage
coverage:
	$(GO) test ./... -coverprofile=coverage.out && $(GO) tool cover -html=coverage.out

# +-------------------+
# | image / registry  |
# +-------------------+
# --platform linux/amd64 is required, not optional: this machine is arm64 and
# the GKE nodes are e2-medium (amd64). A natively-built image would fail on the
# node with "exec format error".
DOCKER    := $(shell which docker)
AR_HOST   := asia-southeast1-docker.pkg.dev
AR_IMAGE  := $(AR_HOST)/vincent-mini-apps-project/league-repository/league-service
TAG       ?= $(shell git rev-parse --short HEAD)

.PHONY: ar-login
ar-login:
	gcloud auth configure-docker $(AR_HOST)

.PHONY: image-push
image-push:
	$(DOCKER) buildx build --platform linux/amd64 --target runtime \
	  -t $(AR_IMAGE):$(TAG) --push .

.PHONY: image
image:
	$(DOCKER) buildx build --platform linux/amd64 --target runtime \
	  -t $(AR_IMAGE):$(TAG) --load .
