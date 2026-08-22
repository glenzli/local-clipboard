FROM scratch
COPY local-clipboard-linux-amd64 /usr/local/bin/local-clipboard
ENTRYPOINT ["/usr/local/bin/local-clipboard"]
