## https://github.com/foursquare/fsqio/issues/55
FROM andrewblaney/twofishes

# install Python 3 via pyenv-build, see:
# https://github.com/pyenv/pyenv
# https://github.com/pyenv/pyenv-installer
# https://stackoverflow.com/questions/41422826/install-python-of-specific-version-system-wide-with-pyenv

RUN curl -L https://github.com/pyenv/pyenv-installer/raw/master/bin/pyenv-installer | bash
RUN /root/.pyenv/plugins/python-build/bin/python-build 3.6.5 /usr/local/python-3.6.5/

RUN ln -s /usr/local/python-3.6.5/bin/python3 /usr/local/bin
RUN ln -s /usr/local/python-3.6.5/bin/pip3 /usr/local/bin

RUN pip3 install --upgrade pip
RUN pip3 install pyyaml aiohttp cchardet aiodns asyncpg certifi

COPY *.py /opt/app/
COPY *.sh /opt/app/
COPY config.yaml /opt/app/

ENV PYTHONUNBUFFERED 1

# reset base image CMD
CMD []

# NOTE: use absolute path to not interfere with the settings of source image
ENTRYPOINT ["/opt/app/entrypoint.py", "-c", "/opt/app/config.yaml"]
