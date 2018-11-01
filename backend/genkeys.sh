#!/bin/sh

scriptdir="$(cd "`dirname "$0"`"; pwd)"

keysdir="$scriptdir/keys"

mkdir -p "$keysdir"

# openssl genrsa -out keys/jwt.rsa 1024
openssl genrsa -out "$keysdir/jwt.rsa" 512
openssl rsa -in "$keysdir/jwt.rsa" -pubout > "$keysdir/jwt.rsa.pub"
