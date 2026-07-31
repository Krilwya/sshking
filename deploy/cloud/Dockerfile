FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -S sshking && adduser -S -G sshking sshking
COPY sshking-api /usr/local/bin/sshking-api
USER sshking
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/sshking-api"]
