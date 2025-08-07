FROM python:3.12-slim-bookworm
COPY web-console/webconsole /usr/local/bin/webconsole
EXPOSE 8090
