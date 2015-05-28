FROM golang
RUN go get github.com/kelseyhightower/envconfig
ADD . /go/src/github.com/thraxil/hound
RUN go install github.com/thraxil/hound
RUN mkdir /etc/hound
ENV HOUND_HTTP_PORT=9998
EXPOSE 9998
CMD ["/go/bin/hound", "-config=/etc/hound/config.json"]
