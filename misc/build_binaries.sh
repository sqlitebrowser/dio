#!/bin/sh

# This is just a small sh script to generate the Dio release binaries
export GOOS=darwin
export GOARCH=unknown
for GOARCH in 386 amd64; do
  echo Building Dio for ${GOOS} + ${GOARCH}
  go build -o dio-${GOOS}-${GOARCH} ..
  sha256sum dio-${GOOS}-${GOARCH} > dio-${GOOS}-${GOARCH}.SHA256
done

GOOS=freebsd
for GOARCH in 386 amd64; do
  echo Building Dio for ${GOOS} + ${GOARCH}
  go build -o dio-${GOOS}-${GOARCH} ..
  sha256sum dio-${GOOS}-${GOARCH} > dio-${GOOS}-${GOARCH}.SHA256
done

GOOS=windows
for GOARCH in 386 amd64; do
  echo Building Dio for ${GOOS} + ${GOARCH}
  go build -o dio-${GOOS}-${GOARCH}.exe ..
  sha256sum dio-${GOOS}-${GOARCH}.exe > dio-${GOOS}-${GOARCH}.exe.SHA256
done

GOOS=linux
for GOARCH in 386 amd64 arm64 ppc64 ppc64le s390x; do
  echo Building Dio for ${GOOS} + ${GOARCH}
  go build -o dio-${GOOS}-${GOARCH} ..
  sha256sum dio-${GOOS}-${GOARCH} > dio-${GOOS}-${GOARCH}.SHA256
done

echo Building Dio for ${GOOS} + ARMv6
GOARCH=arm GOARM=6 go build -o dio-${GOOS}-armv6 ..
sha256sum dio-${GOOS}-armv6 > dio-${GOOS}-armv6.SHA256
