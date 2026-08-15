package s3ds

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	ds "github.com/ipfs/go-datastore"
	dstest "github.com/ipfs/go-datastore/test"
)

func TestSuiteLocalS3(t *testing.T) {
	// Only run tests when LOCAL_S3 is set, since the tests are only set up for a local S3 endpoint.
	// To run tests locally, run `docker-compose up` in this repo in order to get a local S3 running
	// on port 9000. Then run `LOCAL_S3=true go test -v ./...` to execute tests.
	if _, localS3 := os.LookupEnv("LOCAL_S3"); !localS3 {
		t.Skipf("skipping test suit; LOCAL_S3 is not set.")
	}

	config := Config{
		RegionEndpoint: "http://localhost:9000",
		Bucket:         "localbucketname",
		Region:         "local",
		AccessKey:      "test",
		SecretKey:      "testdslocal",
	}

	s3ds, err := NewS3Datastore(config)
	if err != nil {
		t.Fatal(err)
	}

	if err = devMakeBucket(s3ds.S3, "localbucketname"); err != nil {
		t.Fatal(err)
	}

	t.Run("basic operations", func(t *testing.T) {
		dstest.SubtestBasicPutGet(t, s3ds)
	})
	t.Run("not found operations", func(t *testing.T) {
		dstest.SubtestNotFounds(t, s3ds)
	})
	t.Run("many puts and gets, query", func(t *testing.T) {
		dstest.SubtestManyKeysAndQuery(t, s3ds)
	})
	t.Run("return sizes", func(t *testing.T) {
		dstest.SubtestReturnSizes(t, s3ds)
	})
}

func TestReadBucketsOrder(t *testing.T) {
	s := &S3Bucket{Config: Config{
		Bucket:  "write",
		Buckets: []string{"old-a", "write", "", "old-b", "old-a"},
	}}
	got := s.readBuckets()
	want := []string{"write", "old-a", "old-b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readBuckets() = %v; want %v", got, want)
	}
}

func TestMultiBucketGet(t *testing.T) {
	if _, localS3 := os.LookupEnv("LOCAL_S3"); !localS3 {
		t.Skipf("skipping test suit; LOCAL_S3 is not set.")
	}

	oldCfg := Config{
		RegionEndpoint: "http://localhost:9000",
		Bucket:         "oldbucket",
		Region:         "local",
		AccessKey:      "test",
		SecretKey:      "testdslocal",
	}
	oldDS, err := NewS3Datastore(oldCfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = devMakeBucket(oldDS.S3, "oldbucket"); err != nil {
		t.Fatal(err)
	}
	if err = devMakeBucket(oldDS.S3, "newbucket"); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	oldKey := ds.NewKey("old-block")
	if err := oldDS.Put(ctx, oldKey, []byte("from-old")); err != nil {
		t.Fatal(err)
	}

	newDS, err := NewS3Datastore(Config{
		RegionEndpoint: "http://localhost:9000",
		Bucket:         "newbucket",
		Buckets:        []string{"oldbucket"},
		Region:         "local",
		AccessKey:      "test",
		SecretKey:      "testdslocal",
	})
	if err != nil {
		t.Fatal(err)
	}

	newKey := ds.NewKey("new-block")
	if err := newDS.Put(ctx, newKey, []byte("from-new")); err != nil {
		t.Fatal(err)
	}

	got, err := newDS.Get(ctx, newKey)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "from-new" {
		t.Fatalf("Get(new) = %q; want from-new", got)
	}

	got, err = newDS.Get(ctx, oldKey)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "from-old" {
		t.Fatalf("Get(old) = %q; want from-old", got)
	}

	has, err := newDS.Has(ctx, oldKey)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("Has(old) = false; want true")
	}

	_, err = newDS.Get(ctx, ds.NewKey("missing"))
	if err != ds.ErrNotFound {
		t.Fatalf("Get(missing) err = %v; want ErrNotFound", err)
	}

	// Put must not write into the old bucket.
	if has, err := oldDS.Has(ctx, newKey); err != nil {
		t.Fatal(err)
	} else if has {
		t.Fatal("Put wrote into the old bucket")
	}
}

func devMakeBucket(s3obj *s3.S3, bucketName string) error {
	s3obj.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	_, err := s3obj.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})

	return err
}
