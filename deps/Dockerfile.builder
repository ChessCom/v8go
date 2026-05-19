FROM ubuntu:24.04

RUN apt-get update && apt-get install -y \
    git python3 curl pkg-config lsb-release sudo xz-utils \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /work
