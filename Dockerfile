FROM golang:1.13.9 AS builder

WORKDIR /app
RUN git clone https://github.com/tmax-cloud/gpu-resource-toleration-admission-controller.git
WORKDIR /app/gpu-resource-toleration-admission-controller
RUN go build


FROM ubuntu:18.04

LABEL maintainer "Minyoung Park <minyoung_park@tmax.co.kr>,Taesun Lee <taesun_lee@tmax.co.kr>"
COPY --from=builder /app/gpu-resource-toleration-admission-controller/gpu-resource-toleration-admission-controller /bin/
CMD ["gpu-resource-toleration-admission-controller"]