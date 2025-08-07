FROM debian
COPY web-console/webconsole /usr/local/bin/webconsole
EXPOSE 8090
CMD ["/usr/local/bin/webconsole","--debug","--localOnly","false"]
