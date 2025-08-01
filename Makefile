.PHONY: build build-15 build-16 build-17 build-all push push-15 push-16 push-17 push-all test test-simple test-compose test-all clean help status

# Docker registry and image configuration
REGISTRY ?= ghcr.io
IMAGE_BASE ?= flanksource/postgres-upgrade
IMAGE_TAG ?= latest

# Build operations
build:
	task build

build-15:
	docker build --build-arg TARGET_VERSION=15 -t $(REGISTRY)/$(IMAGE_BASE):to-15 -t $(REGISTRY)/$(IMAGE_BASE):to-15-$(IMAGE_TAG) .

build-16:
	docker build --build-arg TARGET_VERSION=16 -t $(REGISTRY)/$(IMAGE_BASE):to-16 -t $(REGISTRY)/$(IMAGE_BASE):to-16-$(IMAGE_TAG) .

build-17:
	docker build --build-arg TARGET_VERSION=17 -t $(REGISTRY)/$(IMAGE_BASE):to-17 -t $(REGISTRY)/$(IMAGE_BASE):to-17-$(IMAGE_TAG) .

build-all: build-15 build-16 build-17

# Push operations
push-15: build-15
	docker push $(REGISTRY)/$(IMAGE_BASE):to-15
	docker push $(REGISTRY)/$(IMAGE_BASE):to-15-$(IMAGE_TAG)

push-16: build-16
	docker push $(REGISTRY)/$(IMAGE_BASE):to-16
	docker push $(REGISTRY)/$(IMAGE_BASE):to-16-$(IMAGE_TAG)

push-17: build-17
	docker push $(REGISTRY)/$(IMAGE_BASE):to-17
	docker push $(REGISTRY)/$(IMAGE_BASE):to-17-$(IMAGE_TAG)

push-all: push-15 push-16 push-17

test-simple:
	task test-simple

test-dockerfile:
	task test-dockerfile

test-compose:
	task test-upgrades

test-all:
	task test-all

test:
	task test

# Development shortcuts
dev-setup:
	task dev-setup

dev-test-quick:
	task dev-test-quick

# Utility commands
clean:
	task clean

status:
	task status

help:
	task help

# Individual upgrade tests
test-14-to-15:
	@echo "Testing PostgreSQL 14 to 15 upgrade with TARGET_VERSION=15..."
	TARGET_VERSION=15 task upgrade-14-to-17

test-14-to-16:
	@echo "Testing PostgreSQL 14 to 16 upgrade with TARGET_VERSION=16..."
	TARGET_VERSION=16 task upgrade-14-to-17

test-15-to-16:
	@echo "Testing PostgreSQL 15 to 16 upgrade with TARGET_VERSION=16..."
	TARGET_VERSION=16 task upgrade-15-to-17

test-14-to-17:
	task upgrade-14-to-17

test-15-to-17:
	task upgrade-15-to-17

test-16-to-17:
	task upgrade-16-to-17

# Seeding tasks
seed-all:
	task seed-all

seed-14:
	task seed-pg14

seed-15:
	task seed-pg15

seed-16:
	task seed-pg16