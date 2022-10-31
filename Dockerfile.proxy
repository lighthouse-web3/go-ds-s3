# FROM ubuntu:20.04
FROM golang:1.18.1

LABEL maintainer "Perfection <perfection@lighthouse.storage>"

# update apt and install dependencies
# RUN export TZ="Asia/Jakarta" && \ 
#     export DEBIAN_FRONTEND=noninteractive && \
#     apt update && \
#     apt install -y golang-go git && \
#     echo "GIT VERSION IS -- $(git --version)" 

# clone ipfs 
RUN git clone https://github.com/ipfs/go-ipfs && \
    cd go-ipfs && \
    go env -w GOPRIVATE="github.com/lighthouse-web3" && \
    #git checkout release-v0.15.0 && \
    go get github.com/lighthouse-web3/go-ds-s3/plugin@v0.14.0


# Add the plugin to the preload list.   
RUN cd go-ipfs && \
    echo "\ns3ds github.com/lighthouse-web3/go-ds-s3/plugin 0" >> plugin/loader/preload_list &&\
    cat plugin/loader/preload_list && \
    cat plugin/loader/preload.go && \
    make build && \
    go mod tidy && \
    make build && \
    make install && \
    cp cmd/ipfs/ipfs /usr/local/bin/ipfs

# checkpoint --- echo ipfs version
RUN ipfs version
RUN  echo "IPFS VERSION IS -- $(ipfs version)"

ENV IPFS_PATH /data/ipfs

ENV API_PORT 5001
ENV GATEWAY_PORT 8080
ENV SWARM_PORT 4001
ENV PROXY_PORT 5050

# defining args to be passed in at build time
ARG AWS_SECRET_KEY
ARG AWS_ACCESS_KEY
ARG AWS_REGION
ARG AWS_S3_BUCKET
ARG GITHUB_TOKEN
ARG IPFS_ENDPOINT

ENV AWS_SECRET_KEY  $AWS_SECRET_KEY
ENV AWS_ACCESS_KEY  $AWS_ACCESS_KEY
ENV AWS_REGION      $AWS_REGION
ENV AWS_S3_BUCKET   $AWS_S3_BUCKET
ENV GITHUB_TOKEN    $GITHUB_TOKEN
ENV IPFS_ENDPOINT   $IPFS_ENDPOINT

EXPOSE ${SWARM_PORT}
# This may introduce security risk to expose API_PORT public
EXPOSE ${API_PORT}
EXPOSE ${GATEWAY_PORT}
EXPOSE ${PROXY_PORT}

RUN mkdir -p ${IPFS_PATH} 

# no need to for this as docker runs in root user mode
# RUN chown ubuntu:ubuntu ${IPFS_PATH}

# configure ipfs for production
RUN ipfs init -p server
    
RUN ipfs config Datastore.StorageMax 2TB && \
    ipfs config Routing.Type none && \
    ipfs bootstrap add /ip4/3.110.235.23/tcp/4001/p2p/12D3KooWGLVpG6uUMZoKhAdyJGbsqhPyea4qPA8CDqBxaiPhXe3e



RUN ipfs config --json Datastore.Spec "{\"mounts\":[{\"child\":{\"accessKey\":\"${AWS_ACCESS_KEY}\",\"bucket\":\"${AWS_S3_BUCKET}\",\"region\":\"${AWS_REGION}\",\"secretKey\":\"${AWS_SECRET_KEY}\",\"type\":\"s3ds\"},\"mountpoint\":\"/blocks\",\"prefix\":\"s3.datastore\",\"type\":\"measure\"},{\"child\": {\"compression\":\"none\",\"path\":\"datastore\",\"type\":\"levelds\"},\"mountpoint\": \"/\",\"prefix\":\"leveldb.datastore\",\"type\":\"measure\"}],\"type\":\"mount\"}"

RUN echo "{\"mounts\":[{\"bucket\":\"${AWS_S3_BUCKET}\",\"mountpoint\":\"/blocks\",\"region\":\"${AWS_REGION}\",\"rootDirectory\":\"\"},{\"mountpoint\":\"/\",\"path\":\"datastore\",\"type\":\"levelds\"}],\"type\":\"mount\"}" > $IPFS_PATH/datastore_spec

RUN apt update -y && \
    apt install -y nodejs && \
    echo "NODE VERSION $(node --version)"

RUN git clone https://lighthouse-web3:${GITHUB_TOKEN}@github.com/lighthouse-web3/node-authentication-middleware.git proxy && \
    cd proxy && \
    apt install -y npm && \
    npm install && \
    npm install -g pm2

RUN echo "PM2 VERSION -- $(pm2 --version)" && \
    stat proxy/app.js

# by default, run `ipfs daemon` to start as a running node
ENTRYPOINT pm2 start proxy/app.js && ipfs daemon