.PHONY: dev dev-env-start

POSTGRES_CONTAINER ?= url-shortener-postgres
POSTGRES_IMAGE ?= postgres:17

dev:
	go run main.go

dev-env-start:
	@if docker container inspect "$(POSTGRES_CONTAINER)" >/dev/null 2>&1; then \
		docker start "$(POSTGRES_CONTAINER)"; \
	else \
		set -a; . ./.env; set +a; \
		POSTGRES_CONTAINER="$(POSTGRES_CONTAINER)" POSTGRES_IMAGE="$(POSTGRES_IMAGE)" \
		python3 -c 'import os, subprocess; from urllib.parse import urlparse, unquote; u = urlparse(os.environ["DATABASE_URL"]); env = dict(os.environ, POSTGRES_USER=unquote(u.username or "postgres"), POSTGRES_PASSWORD=unquote(u.password or ""), POSTGRES_DB=unquote(u.path.lstrip("/") or "postgres")); subprocess.run(["docker", "run", "--detach", "--name", os.environ["POSTGRES_CONTAINER"], "--publish", "127.0.0.1:%s:5432" % (u.port or 5432), "--env", "POSTGRES_USER", "--env", "POSTGRES_PASSWORD", "--env", "POSTGRES_DB", "--volume", "url-shortener-postgres-data:/var/lib/postgresql/data", os.environ["POSTGRES_IMAGE"]], env=env, check=True)'; \
	fi
