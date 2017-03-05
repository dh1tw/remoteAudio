#!/bin/sh
set -e
# check to see if protobuf folder is empty
if [ ! -d "$HOME/protobuf/lib" ]; then
  CURRDIR= pwd #cache current directory
  wget -P $HOME https://github.com/google/protobuf/archive/v3.1.0.tar.gz
  tar -xzvf $HOME/v3.1.0.tar.gz -C $HOME
  cd $HOME/protobuf-3.1.0 && ./autogen.sh && ./configure --prefix=$HOME/protobuf && make && make install
  chmod +x $HOME/protobuf/bin/protoc
  cd $CURDIR #get back to initial directory
  export PATH="$PATH:$HOME/protobuf/bin"
else
  echo "Using cached directory."
fi