#!/bin/bash -xe

# temp while developing - TODO move whatever we need from here into Go tests

go install .

cd testdata

gocode_mongocrud -dry-run -package=a -type=T1
