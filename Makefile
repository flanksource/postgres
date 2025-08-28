.PHONY: build build-15 build-16 build-17 build-all push push-15 push-16 push-17 push-all test test-simple test-compose test-all clean help status

# Docker registry and image configuration
REGISTRY ?= ghcr.io
IMAGE_BASE ?= flanksource/postgres-upgrade
IMAGE_TAG ?= latest

# Build operations
build:
	task build

build-15:
	task build:build-15

build-16:
	task build:build-16

build-17:
	task build:build-17

build-all:
	task build:build-all

# Push operations
push-15:
	REGISTRY=$(REGISTRY) IMAGE_BASE=$(IMAGE_BASE) IMAGE_TAG=$(IMAGE_TAG) task build:push-15

push-16:
	REGISTRY=$(REGISTRY) IMAGE_BASE=$(IMAGE_BASE) IMAGE_TAG=$(IMAGE_TAG) task build:push-16

push-17:
	REGISTRY=$(REGISTRY) IMAGE_BASE=$(IMAGE_BASE) IMAGE_TAG=$(IMAGE_TAG) task build:push-17

push-all:
	REGISTRY=$(REGISTRY) IMAGE_BASE=$(IMAGE_BASE) IMAGE_TAG=$(IMAGE_TAG) task build:push-all

test-simple:


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
	@echo "Testing PostgreSQL 14 to 15 upgrade..."
	task test:upgrade-14-to-15

test-14-to-16:
	@echo "Testing PostgreSQL 14 to 16 upgrade..."
	task test:upgrade-14-to-16

test-15-to-16:
	@echo "Testing PostgreSQL 15 to 16 upgrade..."
	task test:upgrade-15-to-16

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
