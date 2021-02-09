# docker build -t testsunny -f Dockerfile.test .
FROM golang:1.13.9 AS builder

WORKDIR /app
RUN git clone -b dev/main https://github.com/taesunny/gpu-resource-toleration-admission-controller.git
WORKDIR /app/gpu-resource-toleration-admission-controller
RUN go build


FROM ubuntu:18.04

LABEL maintainer "Taesun Lee <taesun_lee@tmax.co.kr>,Kwanghun Choi <kwanghun_choi@tmax.co.kr>"
COPY --from=builder /app/gpu-resource-toleration-admission-controller/gpu-resource-toleration-admission-controller /bin/
ENTRYPOINT ["gpu-resource-toleration-admission-controller"]