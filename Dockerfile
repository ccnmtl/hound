FROM golang
RUN go get github.com/kelseyhightower/envconfig
ADD . /go/src/github.com/thraxil/hound
RUN go install github.com/thraxil/hound
RUN mkdir /etc/hound
ENV HOUND_HTTP_PORT=9998
ENV HOUND_TEMPLATE_FILE=/go/src/github.com/thraxil/hound/index.html
ENV HOUND_SMTP_SERVER=postfix
ENV HOUND_SMTP_PORT=25
EXPOSE 9998
CMD ["/go/bin/hound", "-config=/etc/hound/config.json"]
