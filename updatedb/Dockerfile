FROM ubuntu:16.04

RUN apt update && apt install -y software-properties-common curl
RUN add-apt-repository ppa:jonathonf/python-3.6
RUN ln -sf python3.6 /usr/bin/python3
# RUN apt update && apt install -y python3 python3-pip python3-dev
RUN apt update && apt install -y python3.6 python3.6-dev
RUN curl https://bootstrap.pypa.io/get-pip.py | python3.6
# RUN pip3 install -U pip
RUN pip3 install asyncpg==0.15.0 aiohttp==3.0.1 aiodns==1.1.1 cchardet==2.1.1 certifi==2018.1.18 PyYAML==3.12

WORKDIR /app

COPY updatedb.py /app
COPY config.yaml /app

ENTRYPOINT ["/app/updatedb.py"]
