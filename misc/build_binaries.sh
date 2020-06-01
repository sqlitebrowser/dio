#!/bin/sh

# This is just a small sh script to generate the Dio release binaries

export GOARCH=386
for GOOS in android darwin freebsd netbsd openbsd plan9 windows linux; do
  echo Building Dio for ${GOOS}-${GOARCH}
  go build -o dio-${GOOS}-x86 ..
  sha256sum dio-${GOOS}-x86 > dio-${GOOS}-x86.SHA256
done

export GOARCH=amd64
for GOOS in android darwin freebsd netbsd openbsd plan9 solaris windows linux; do
  echo Building Dio for ${GOOS}-${GOARCH}
  go build -o dio-${GOOS}-${GOARCH} ..
  sha256sum dio-${GOOS}-${GOARCH} > dio-${GOOS}-x86.SHA256
done

export GOARCH=arm
for GOOS in android darwin freebsd netbsd openbsd plan9 windows linux; do
  echo Building Dio for ${GOOS}-${GOARCH}
  go build -o dio-${GOOS}-${GOARCH} ..
  sha256sum dio-${GOOS}-${GOARCH} > dio-${GOOS}-x86.SHA256
done

export GOARCH=arm64
for GOOS in android darwin freebsd illumos netbsd openbsd linux; do
  echo Building Dio for ${GOOS}-${GOARCH}
  go build -o dio-${GOOS}-${GOARCH} ..
  sha256sum dio-${GOOS}-${GOARCH} > dio-${GOOS}-x86.SHA256
done

GOOS=linux
for GOARCH in mips mips64 mips64le mipsle ppc64 ppc64le s390x; do
  echo Building Dio for ${GOOS}-${GOARCH}
  go build -o dio-${GOOS}-${GOARCH} ..
  sha256sum dio-${GOOS}-${GOARCH} > dio-${GOOS}-${GOARCH}.SHA256
done

echo Building Dio for ${GOOS}-ARMv6
GOARCH=arm GOARM=6 go build -o dio-${GOOS}-armv6 ..
sha256sum dio-${GOOS}-armv6 > dio-${GOOS}-armv6.SHA256

echo Building Dio for aix-ppc64
go build -o dio-aix-ppc64 ..
sha256sum dio-aix-ppc64 > dio-aix-ppc64.SHA256

echo Building Dio for js-wasm
go build -o dio-js-wasm ..
sha256sum dio-js-wasm > dio-js-wasm.SHA256
