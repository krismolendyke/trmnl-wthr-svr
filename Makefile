.PHONY: run build docker-build docker-run clean all

# Run the application locally using 1Password CLI
run:
	go run . server \
  --application-key $$(op read "op://Private/AmbientWeather/TRMNL Secrets/Application Key") \
  --api-key $$(op read "op://Private/AmbientWeather/TRMNL Secrets/API Key") \
  --device $$(op read "op://Private/AmbientWeather/Station MAC") \
  --webhook-url $$(op read "op://Private/AmbientWeather/TRMNL Secrets/Webhook URL")

# Default target is run
all: run

# Build the binary
build:
	go build -o trmnl-wthr-svr .

# Build the Docker container
docker-build:
	docker build -t trmnl-wthr-svr:latest .

# Run the container using 1Password CLI for secrets
docker-run: docker-build
	docker run \
  -e TRMNL_WTHR_SVR_APPLICATION_KEY=$$(op read "op://Private/AmbientWeather/TRMNL Secrets/Application Key") \
  -e TRMNL_WTHR_SVR_API_KEY=$$(op read "op://Private/AmbientWeather/TRMNL Secrets/API Key") \
  -e TRMNL_WTHR_SVR_DEVICE=$$(op read "op://Private/AmbientWeather/Station MAC") \
  -e TRMNL_WTHR_SVR_WEBHOOK_URL=$$(op read "op://Private/AmbientWeather/TRMNL Secrets/Webhook URL") \
  trmnl-wthr-svr:latest

# Set secrets on Fly.io using 1Password CLI
fly-set-secrets:
	fly secrets set TRMNL_WTHR_SVR_APPLICATION_KEY="$$(op read "op://Private/AmbientWeather/TRMNL Secrets/Application Key")"
	fly secrets set TRMNL_WTHR_SVR_API_KEY="$$(op read "op://Private/AmbientWeather/TRMNL Secrets/API Key")"
	fly secrets set TRMNL_WTHR_SVR_DEVICE="$$(op read "op://Private/AmbientWeather/Station MAC")"
	fly secrets set TRMNL_WTHR_SVR_WEBHOOK_URL="$$(op read "op://Private/AmbientWeather/TRMNL Secrets/Webhook URL")"
	@echo "Secrets have been set in fly.io"

# Clean up build artifacts
clean:
	rm -f trmnl-wthr-svr
