FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY bin/server /server
USER 65532:65532
ENTRYPOINT ["/server"]