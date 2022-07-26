# stage 2: copy only the application binary file and necessary files to the alpine container
FROM alpine:3.16.2

WORKDIR /app

COPY output/ledokol .

# run the service on container startup.
CMD ["/app/ledokol"]