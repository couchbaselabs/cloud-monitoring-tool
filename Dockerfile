FROM golang:1.15-alpine AS build
WORKDIR /src/
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/cloud-monitoring-tool
FROM alpine
COPY --from=build /bin/cloud-monitoring-tool /bin/cloud-monitoring-tool
ENTRYPOINT ["/bin/cloud-monitoring-tool"]
