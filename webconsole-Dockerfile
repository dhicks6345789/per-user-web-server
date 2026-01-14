FROM python:3.12-slim-bookworm
COPY web-console/webconsole /usr/local/bin/webconsole
RUN pip install --no-cache-dir requests
RUN pip install --no-cache-dir pandas
EXPOSE 8090
