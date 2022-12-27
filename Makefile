PROJECT_NAME=simple_runc
IMAGE_NAME=${USER}_${PROJECT_NAME}
CONTAINER_NAME=${USER}_${PROJECT_NAME}
SHM_SIZE=2g
FORCE_RM=true

build:
	docker build \
		-f docker/Dockerfile \
		-t $(IMAGE_NAME) \
		--no-cache \
		--force-rm=$(FORCE_RM) \
		--build-arg USER_ID=$(shell id -u) \
		--build-arg GROUP_ID=$(shell id -g) \
		.

run:
	docker run \
		-dit \
		-v $(PWD):/usr/src \
		--name $(CONTAINER_NAME) \
		--rm \
		--cap-add SYS_ADMIN \
		--shm-size $(SHM_SIZE) \
		$(IMAGE_NAME)

exec:
	docker exec \
		-it \
		$(CONTAINER_NAME) /bin/bash 

stop:
	docker stop $(IMAGE_NAME)

restart: stop run

install:
	go build .
	install -D -m0755 simple_runc /usr/local/bin/runc
	rm simple_runc