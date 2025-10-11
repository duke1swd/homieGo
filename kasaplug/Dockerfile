FROM golang:latest AS build
RUN mkdir /app
WORKDIR /app
RUN git clone https://github.com/duke1swd/homieGo.git
WORKDIR /app/homieGo/kasaplug
ENV CGO_ENABLED=0
RUN go build

FROM scratch AS prod
COPY --from=build /app/homieGo/kasaplug/kasaplug /bin/kasaplug
ENV LOGDIR=-
ENV MQTTBROKER=tcp://DanielPi3:1883
ENV NETWORK=192.168.2.0/24


ENTRYPOINT [ "/bin/kasaplug" ]
#ENTRYPOINT [ "/bin/sh" ]
