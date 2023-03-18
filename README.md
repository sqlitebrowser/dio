# dio

Dio is our reference command line interface (CLI) application for working with [DBHub.io](https://dbhub.io/).

It can be used used to:

* transfer databases to and from the cloud (pushing and pulling)
* check their version history
* create branches, tags, releases, and commits
* diff changes (in a future release)
* and more... (eventually)

It's at a fairly early stage in its development, though the main pieces should
all work.  It certainly needs more polish to be more user-friendly though.

## Building from source

Dio requires Go to be installed (version 1.17+ is known to work).  Building should
just require:

```bash
$ go get github.com/sqlitebrowser/dio
$ go install github.com/sqlitebrowser/dio
```

## Getting Started

To use it, do the following:
1. Create a folder named `.dio` in your home directory;
```bash
$ cd ~
$ mkdir .dio
```
2. Download [`ca-chain-cert.pem`](https://github.com/sqlitebrowser/dio/blob/master/cert/ca-chain.cert.pem) to `~/.dio/`. For example:
```bash
$ cd ~/.dio
$ wget https://github.com/sqlitebrowser/dio/raw/master/cert/ca-chain.cert.pem
```
3. Generate a certificate file for yourself at [DBHub.io](https://dbhub.io/) and save it in `~/.dio/`.
4. Create the following text file, and name it `~/.dio/config.toml`:
```toml
[user]
name = "Your Name"
email = "youremail@example.org"

[certs]
cachain = "/home/username/.dio/ca-chain.cert.pem"
cert = "/home/username/.dio/username.cert.pem"

[general]
cloud = "https://db4s.dbhub.io"

```
5. Change the `name` and `email` values to your name and email address
6. Change `/home/username` to the path to your home directory
7. Make sure `cachain` points to the downloaded ca-chain.cert.pem file
8. Make sure `cert` points to your generated DBHub.io certificate
* Leave the `cloud` value pointing to https://db4s.dbhub.io

To verify this file is set up correctly, type:
```bash
$ dio info
```
which will display the information loaded from this configuration file.

Dio has a `help` option (`dio help`) which is useful for listing the available dio
commands, explaining their purpose, etc.
