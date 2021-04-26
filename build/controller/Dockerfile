FROM golang:1.16-buster
RUN apt update && apt install -y sudo
ENV APP_USER app
ENV APP_HOME /go/app
ARG GROUP_ID=1002
ARG USER_ID=1002
RUN groupadd --gid $GROUP_ID app && useradd -m -l --uid $USER_ID --gid $GROUP_ID $APP_USER
RUN mkdir -p $APP_HOME
COPY controller /go/app/controller
COPY entrypoint.sh /go/app/entrypoint.sh
RUN chown -R $APP_USER:$APP_USER $APP_HOME
USER root
WORKDIR $APP_HOME
EXPOSE 10000
EXPOSE 11000
CMD ["./entrypoint.sh"]
