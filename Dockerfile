FROM golang:1.22
WORKDIR /go/src/app
COPY . .

ENV HOUND_HTTP_PORT=9998
ENV HOUND_TEMPLATE_FILE=/go/src/app/index.html
ENV HOUND_SMTP_SERVER=postfix
ENV HOUND_SMTP_PORT=25
EXPOSE 9998
CMD ["go", "run", "-config", "/config.json"]

