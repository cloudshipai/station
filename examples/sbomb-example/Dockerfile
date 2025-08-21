FROM alpine:3.12
RUN apk add --no-cache ca-certificates
COPY app /app
WORKDIR /app
CMD ["node", "index.js"]