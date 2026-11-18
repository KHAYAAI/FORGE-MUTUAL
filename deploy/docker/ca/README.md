# Extra certificate authorities for image builds

Any `*.crt` file in this directory is installed into the build stage's trust store before modules are downloaded.
Use it where HTTPS is intercepted by a proxy with its own CA (a corporate or sandbox proxy). The files are local
to each machine and are not committed (see .gitignore).
