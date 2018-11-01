FROM ubuntu:18.04

RUN apt update && apt install -y python3-pip curl
RUN pip3 install --upgrade pip
RUN pip3 install pyyaml==3.12 gensim==3.4.0 asyncpg==0.17.0

WORKDIR /opt/app

COPY models/ /opt/app/models/
COPY *.py /opt/app/
COPY *.sh /opt/app/
COPY config.yaml /opt/app/

VOLUME /opt/app/state

# for unbuffered stdout
ENV PYTHONUNBUFFERED y

ENTRYPOINT ["/opt/app/clustering_service.py"]
