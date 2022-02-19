# dio
[![Go](https://github.com/alexDtorres/dio/actions/workflows/go.yml/badge.svg?branch=master&event=push)](https://github.com/alexDtorres/dio/actions/workflows/go.yml)

Dio is our reference command line interface (CLI) for working with DBHub.io.

It can be used used to:

* transfer databases to and from the cloud (pushing and pulling)
* check their version history
* create branches, tags, releases, and commits
* diff changes (in a future release)
* and more... (eventually)

It's at a fairly early stage in it's development, though the main pieces should
all work.  It's not yet polished and user friendly though.

## Building from source

Dio requires Go to be installed (version 1.11.4+ is known to work).  Building should
just require:

```
$ go get github.com/sqlitebrowser/dio
```

## Getting Started

To use it, generate a certificate file for yourself on [DBHub.io](https://dbhub.io),
save it somewhere, and create a text file called `config.toml` in a `.dio` folder
off your home directory:

```
[certs]
cachain = "ca-chain.cert.pem"
cert = "/path/to/your/certificate.cert.pem"

[general]
cloud = "https://db4s.dbhub.io"

[user]
name = "Your Name"
email = "youremail@example.org"
```

* The `ca-chain-cert.pem` file is from [here](https://github.com/sqlitebrowser/dio/blob/master/cert/ca-chain.cert.pem)
  * Download it and save it on your computer, then update that path to point to it
* The `cert` path should point to your generated DBHub.io certificate
* The `cloud` value should be left alone (eg pointing to https://db4s.dbhub.io)
* The name and email values should be set to your name and email address

You can check the information from Dio's point of view by running `dio info`, which
will display the information it has loaded from the configuration file.

Dio has a `help` option (`dio help`) which is useful for listing the available dio
commands, explaining their purpose, etc.
