FROM golang:1.19

WORKDIR /usr/src

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

RUN apt-get update && \
    apt-get install sudo

# duser setting
# ARG USER_ID
# ARG GROUP_ID
# RUN addgroup --gid $GROUP_ID duser && \
#     adduser --disabled-password --gecos '' --uid ${USER_ID} --gid ${GROUP_ID} duser && \
#     adduser duser sudo && \
#     echo '%sudo ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# USER duser