FROM golang:1.18 as builder
RUN mkdir /app
WORKDIR /app
RUN go install github.com/azyablov/nsrv@latest
RUN git clone https://github.com/azyablov/nsrv


FROM golang:1.18
RUN apt-get update -y && \ 
    apt-get upgrade -y && \
    mkdir /app && \
    mkdir /app/dyn-url-filtering
WORKDIR /app
ENV CONFIG=/app/nsrv.json
VOLUME [ "/app/dyn-url-filtering" ]
COPY --from=builder /go/bin/nsrv /app/
COPY --from=builder /app/nsrv/nsrv.json ${CONFIG}
ENTRYPOINT [ "/app/nsrv" ]
