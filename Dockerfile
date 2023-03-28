FROM golang:1.19 AS build

RUN apt-get update && apt-get install -y \
    curl \
    git \
    build-essential \
    libboost-all-dev \
    bzip2 \
    nano \
    jq

WORKDIR /app

RUN git clone https://github.com/mewmix/atomic-swap.git

WORKDIR /app/atomic-swap

RUN make build


FROM node:latest

WORKDIR /app

COPY --from=build /app/atomic-swap /app/atomic-swap

RUN npm install --global ganache

WORKDIR /app/atomic-swap



