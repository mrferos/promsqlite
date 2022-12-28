FROM ubuntu:22.04

ENV DEBIAN_FRONTEND noninteractive
ENV DEBCONF_NONINTERACTIVE_SEEN true

ADD https://go.dev/dl/go1.19.4.linux-amd64.tar.gz /opt/go.tar.gz

# Setup go
RUN tar -xvf /opt/go.tar.gz -C /opt && \
    mv /opt/go/bin/go /usr/local/bin/go && \
    mv /opt/go /usr/local/go

# Setup node
RUN apt update && \
    apt-get install -y curl && \
    curl -fsSL https://deb.nodesource.com/setup_18.x | bash - && \
    apt-get update && \
    apt-get install -y nodejs build-essential


# Install the stuff we need
RUN npm i -g nodemon