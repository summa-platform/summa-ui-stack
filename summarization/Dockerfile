FROM summaleta/priberam-summarization:0.0.0_alt

RUN apt-get update && apt install -y python3-pip curl
RUN pip3 install --upgrade pip
RUN pip3 install pyyaml aiohttp cchardet aiodns asyncpg certifi

WORKDIR /opt/app

COPY *.py /opt/app/
COPY *.sh /opt/app/
COPY config.yaml /opt/app/

# for unbuffered stdout
ENV PYTHONUNBUFFERED y

# reset base image CMD
CMD []

ENTRYPOINT ["/opt/app/entrypoint.py"]
