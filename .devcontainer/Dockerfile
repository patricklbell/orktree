FROM golang:1.23-bullseye

RUN apt-get update && apt-get install -y \
    fuse-overlayfs \
    sudo \
    pandoc \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m devuser && echo "devuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers
USER devuser
WORKDIR /workspace

ENV GO111MODULE=on \
    GOPATH=/home/devuser/go \
    PATH=$GOPATH/bin:/usr/local/go/bin:$PATH

COPY --chown=devuser:devuser go.mod go.sum* ./
RUN go mod download

COPY --chown=devuser:devuser . .