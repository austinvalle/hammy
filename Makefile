# Load .env so the migrate targets can build the database URL.
-include .env
export

# Variables
DockerFile := docker-compose-dev.yml
DockerGpu := docker-compose-dev-gpu.yml
ContainerName := ollama
ModelName := hammy

MigrationsDir := migrations
# Connects to the dev database published on localhost by docker-compose-dev.yml.
DbUrl := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:5432/$(POSTGRES_DB)?sslmode=disable

.PHONY: start start-gpu up down delete setup_models migrate-create migrate-up migrate-down migrate-version

# Start services and setup models for CPU
start: up setup_models
	@echo "Services started and models set up."

# Start services and setup models for GPU
start-gpu: up-gpu setup_models
	@echo "Services started with GPU and models set up."

# Bring up the CPU services
up:
	@echo "Starting services using ${DockerFile}..."
	docker compose -f ${DockerFile} up -d

# Bring up the GPU services
up-gpu:
	@echo "Starting services using ${DockerGpu}..."
	docker compose -f ${DockerGpu} up -d

# Bring down the CPU services
down:
	@echo "Stopping services using ${DockerFile}..."
	docker compose -f ${DockerFile} down

# Bring down the GPU services
down-gpu:
	@echo "Stopping services using ${DockerGpu}..."
	docker compose -f ${DockerGpu} down

# Delete a specific model
delete:
	@echo "Deleting model ${ModelName}..."
	docker exec ${ContainerName} ollama rm ${ModelName}

# Set up models in the container
setup_models:
	@echo "Setting up model ${ModelName}..."
	docker exec ${ContainerName} ollama create ${ModelName} -f /models/${ModelName}.modelfile

# Create a new pair of migration files: make migrate-create name=add_something
migrate-create:
	@test -n "$(name)" || (echo "Usage: make migrate-create name=<description>" && exit 1)
	docker run --rm -v $(PWD)/$(MigrationsDir):/migrations migrate/migrate \
		create -ext sql -dir /migrations -seq $(name)

# Apply all pending migrations against the dev database
migrate-up:
	docker run --rm --network host -v $(PWD)/$(MigrationsDir):/migrations migrate/migrate \
		-path /migrations -database "$(DbUrl)" up

# Roll back the most recent migration
migrate-down:
	docker run --rm --network host -v $(PWD)/$(MigrationsDir):/migrations migrate/migrate \
		-path /migrations -database "$(DbUrl)" down 1

# Print the current migration version
migrate-version:
	docker run --rm --network host -v $(PWD)/$(MigrationsDir):/migrations migrate/migrate \
		-path /migrations -database "$(DbUrl)" version
