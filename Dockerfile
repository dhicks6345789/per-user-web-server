FROM python:3.12-slim-bookworm
COPY web-console/webconsole /usr/local/bin/webconsole
# RUN apt-get update && apt-get install -y python3-pandas
RUN pip install --no-cache-dir pandas
EXPOSE 8090
