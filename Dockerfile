#sudo docker build -t cache .
#sudo docker run -dt --network="host" --name cache cache

FROM golang:1.16-alpine
RUN apk add --no-cache make protoc # WARNING: do not confuse --no-cache with ./build/cache
COPY . /app
WORKDIR /app
RUN make
EXPOSE 6589
ENTRYPOINT ./build/cache