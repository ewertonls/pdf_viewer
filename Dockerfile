FROM golang:1.22.3-alpine as build
WORKDIR /app

COPY . /app
RUN go build -o /app/pdf_viewer .

FROM alpine
COPY --from=build /app /app
RUN rm -rf /app/.git

CMD [ "/app/pdf_viewer" ]
