FROM golang:1.19.1 AS builder

RUN mkdir /lampy
ADD . /lampy
WORKDIR /lampy
RUN go build -o lampy

FROM gcr.io/distroless/base AS production
COPY --from=builder /lampy/lampy .
CMD ["./lampy"]