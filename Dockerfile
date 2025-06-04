# Minify client side assets (JavaScript)
FROM node:latest AS build-js

RUN npm install gulp gulp-cli -g

WORKDIR /build
COPY . .
RUN npm install --only=dev
RUN gulp


# Build Golang binary
# Change shtrv to your-github-username
FROM golang:1.21 AS build-golang

WORKDIR /go/src/github.com/shtrv/gophish
COPY . .
RUN go get -v && go build -v


# Runtime container
# Change shtrv to your-github-username
FROM debian:stable-slim

RUN useradd -m -d /opt/shtrv -s /bin/bash app

RUN apt-get update && \
	apt-get install --no-install-recommends -y jq libcap2-bin ca-certificates && \
	apt-get clean && \
	rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /opt/shtrv
COPY --from=build-golang /go/src/github.com/shtrv/gophish/ ./
COPY --from=build-js /build/static/js/dist/ ./static/js/dist/
COPY --from=build-js /build/static/css/dist/ ./static/css/dist/
COPY --from=build-golang /go/src/github.com/shtrv/gophish/config.json ./
RUN chown app. config.json

RUN setcap 'cap_net_bind_service=+ep' /opt/shtrv/gophish

USER app
RUN sed -i 's/127.0.0.1/0.0.0.0/g' config.json
RUN touch config.json.tmp

EXPOSE 3333 8080 8443 80

CMD ["./docker/run.sh"]
