# S3 Datastore Implementation

This is an implementation of the datastore interface backed by amazon s3.

**NOTE:** Plugins only work on Linux and MacOS at the moment. You can track the progress of this issue here: https://github.com/golang/go/issues/19282

## Building and Installing

You must build the plugin with the _exact_ version of go used to build the go-ipfs binary you will use it with. You can find the go version for go-ipfs builds from dist.ipfs.io in the build-info file, e.g. https://dist.ipfs.io/go-ipfs/v0.4.22/build-info or by running `ipfs version --all`.

In addition to needing the exact version of go, you need to build the correct version of this plugin.

- To build against a released version of go-ipfs, checkout the `release/v$VERSION` branch and build.
- To build against a custom (local) build of go-ipfs, run `make IPFS_VERSION=/path/to/go-ipfs/source`.

You can then install it into your local IPFS repo by running `make install`.

## Bundling

As go plugins can be finicky to correctly compile and install, you may want to consider bundling this plugin and re-building go-ipfs. If you do it this way, you won't need to install the `.so` file in your local repo, i.e following the above Building and Installing section, and you won't need to worry about getting all the versions to match up.

```bash
# We use go modules for everything.
> export GO111MODULE=on

# Clone go-ipfs.
> git clone https://github.com/ipfs/go-ipfs
> cd go-ipfs

# Pull in the datastore plugin (you can specify a version other than latest if you'd like).
> go get github.com/lighthouse-web3/go-ds-s3/plugin@latest

# Add the plugin to the preload list.
> echo -en "\ns3ds github.com/lighthouse-web3/go-ds-s3/plugin 0" >> plugin/loader/preload_list

# ( this first pass will fail ) Try to build go-ipfs with the plugin
> make build

# Update the deptree
> go mod tidy

# Now rebuild go-ipfs with the plugin
> make build

# (Optionally) install go-ipfs
> make install
```

## Detailed Installation

For a brand new ipfs instance (no data stored yet):

1. Copy s3plugin.so $IPFS_DIR/plugins/go-ds-s3.so (or run `make install` if you are installing locally).
2. Run `ipfs init`.
3. Edit $IPFS_DIR/config to include s3 details (see Configuration below).
4. Overwrite `$IPFS_DIR/datastore_spec` as specified below (_Don't do this on an instance with existing data - it will be lost_).

### Configuration

The config file should include the following:

```json
{
  "Datastore": {
  ...

    "Spec": {
      "mounts": [
        {
          "child": {
            "type": "s3ds",
            "region": "us-east-1",
            "bucket": "$bucketname",
            "rootDirectory": "$bucketsubdirectory",
            "accessKey": "",
            "secretKey": ""
          },
          "mountpoint": "/blocks",
          "prefix": "s3.datastore",
          "type": "measure"
        },
```

If the access and secret key are blank they will be loaded from the usual ~/.aws/.
If you are on another S3 compatible provider, e.g. Linode, then your config should be:

```json
{
  "Datastore": {
  ...

    "Spec": {
      "mounts": [
        {
          "child": {
            "type": "s3ds",
            "region": "us-east-1",
            "bucket": "$bucketname",
            "buckets": ["$olderbucket"],
            "rootDirectory": "$bucketsubdirectory",
            "regionEndpoint": "us-east-1.linodeobjects.com",
            "accessKey": "",
            "secretKey": ""
          },
          "mountpoint": "/blocks",
          "prefix": "s3.datastore",
          "type": "measure"
        },
```

### Multiple buckets

Hetzner (and some other S3-compatible stores) cap the number of objects per bucket. When the current write bucket is full, point `bucket` at the new write bucket and list older buckets in `buckets`.

- **Put / Delete / Query** use `bucket` only. Writes stay on one bucket.
- **Get / Has / GetSize** try `bucket` first, then each name in `buckets`, and return not-found only after every bucket misses.
- There is no CID routing. One Kubo process, one plugin config.
- `buckets` can be a JSON array (`["old-a","old-b"]`) or a comma-separated string (`"old-a,old-b"`). The write bucket is always prepended and duplicates are ignored.
- `datastore_spec` still identifies the write location (`bucket`). Adding read-only buckets does not require a datastore migration.

```json
"child": {
  "type": "s3ds",
  "region": "fsn1",
  "bucket": "lighthouse-blocks-2",
  "buckets": ["lighthouse-blocks-1"],
  "regionEndpoint": "fsn1.your-objectstorage.com",
  "accessKey": "",
  "secretKey": ""
}
```

If you are configuring a brand new ipfs instance without any data, you can overwrite the datastore_spec file with:

```
{"mounts":[{"bucket":"$bucketname","mountpoint":"/blocks","region":"us-east-1","rootDirectory":"$bucketsubdirectory"},{"mountpoint":"/","path":"datastore","type":"levelds"}],"type":"mount"}
```

Otherwise, you need to do a datastore migration.

## Contribute

Feel free to join in. All welcome. Open an [issue](https://github.com/lighthouse-web3/go-ds-s3/issues)!

### Want to hack on IPFS?

[![](https://cdn.rawgit.com/jbenet/contribute-ipfs-gif/master/img/contribute.gif)](https://github.com/ipfs/community/blob/master/CONTRIBUTING.md)

## License

MIT
