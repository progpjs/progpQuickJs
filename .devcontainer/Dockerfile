# Build and run:
#   docker build -t clion/ubuntu/cpp-env:1.0 -f Dockerfile.cpp-env-ubuntu .

FROM ubuntu:20.04

# ...
ENV PATH="/usr/local/go/bin:${PATH}"

RUN DEBIAN_FRONTEND="noninteractive" apt-get update && apt-get -y install tzdata

RUN apt-get install -y build-essential gcc g++ make ninja-build cmake tar wget \
    && apt-get clean

RUN wget https://go.dev/dl/go1.22.1.linux-arm64.tar.gz \
    && tar -C /usr/local -xzf go1.22.1.linux-arm64.tar.gz
