package s3ds

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/credentials/endpointcreds"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	ds "github.com/ipfs/go-datastore"
	dsq "github.com/ipfs/go-datastore/query"
)

const (
	// listMax is the largest amount of objects you can request from S3 in a list
	// call.
	listMax = 1000

	// deleteMax is the largest amount of objects you can delete from S3 in a
	// delete objects call.
	deleteMax = 1000

	defaultWorkers = 100

	// credsRefreshWindow, subtracted from the endpointcred's expiration time, is the
	// earliest time the endpoint creds can be refreshed.
	credsRefreshWindow = 2 * time.Minute
)

var _ ds.Datastore = (*S3Bucket)(nil)

type S3Bucket struct {
	Config
	S3 *s3.S3
}

type Config struct {
	AccessKey           string
	SecretKey           string
	SessionToken        string
	Bucket              string
	Buckets             []string
	Region              string
	RegionEndpoint      string
	RootDirectory       string
	Workers             int
	CredentialsEndpoint string
}

func NewS3Datastore(conf Config) (*S3Bucket, error) {
	if conf.Workers == 0 {
		conf.Workers = defaultWorkers
	}

	awsConfig := aws.NewConfig()
	sess, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create new session: %s", err)
	}

	d := defaults.Get()
	creds := credentials.NewChainCredentials([]credentials.Provider{
		&credentials.StaticProvider{Value: credentials.Value{
			AccessKeyID:     conf.AccessKey,
			SecretAccessKey: conf.SecretKey,
			SessionToken:    conf.SessionToken,
		}},
		&credentials.EnvProvider{},
		&credentials.SharedCredentialsProvider{},
		&ec2rolecreds.EC2RoleProvider{Client: ec2metadata.New(sess)},
		endpointcreds.NewProviderClient(*d.Config, d.Handlers, conf.CredentialsEndpoint,
			func(p *endpointcreds.Provider) { p.ExpiryWindow = credsRefreshWindow },
		),
	})

	if conf.RegionEndpoint != "" {
		awsConfig.WithS3ForcePathStyle(true)
		awsConfig.WithEndpoint(conf.RegionEndpoint)
	}

	awsConfig.WithCredentials(creds)
	awsConfig.CredentialsChainVerboseErrors = aws.Bool(true)
	awsConfig.WithRegion(conf.Region)

	sess, err = session.NewSession(awsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create new session with aws config: %s", err)
	}
	s3obj := s3.New(sess)

	return &S3Bucket{
		S3:     s3obj,
		Config: conf,
	}, nil
}

func (s *S3Bucket) Put(ctx context.Context, k ds.Key, value []byte) error {
	_, err := s.S3.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.s3Path(k.String())),
		Body:   bytes.NewReader(value),
	})
	return err
}

func (s *S3Bucket) Sync(ctx context.Context, prefix ds.Key) error {
	return nil
}

func (s *S3Bucket) Get(ctx context.Context, k ds.Key) ([]byte, error) {
	var data []byte
	err := s.withReadBuckets(func(bucket string) error {
		resp, err := s.S3.GetObjectWithContext(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(s.s3Path(k.String())),
		})
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		data, err = ioutil.ReadAll(resp.Body)
		return err
	})
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *S3Bucket) Has(ctx context.Context, k ds.Key) (exists bool, err error) {
	_, err = s.GetSize(ctx, k)
	if err != nil {
		if err == ds.ErrNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *S3Bucket) GetSize(ctx context.Context, k ds.Key) (size int, err error) {
	err = s.withReadBuckets(func(bucket string) error {
		resp, err := s.S3.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(s.s3Path(k.String())),
		})
		if err != nil {
			return err
		}
		size = int(*resp.ContentLength)
		return nil
	})
	if err != nil {
		return -1, err
	}
	return size, nil
}

func (s *S3Bucket) Delete(ctx context.Context, k ds.Key) error {
	_, err := s.S3.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s.s3Path(k.String())),
	})
	if isNotFound(err) {
		// delete is idempotent
		err = nil
	}
	return err
}

func (s *S3Bucket) Query(ctx context.Context, q dsq.Query) (dsq.Results, error) {
	if q.Orders != nil || q.Filters != nil {
		return nil, fmt.Errorf("s3ds: filters or orders are not supported")
	}

	// S3 store a "/foo" key as "foo" so we need to trim the leading "/"
	q.Prefix = strings.TrimPrefix(q.Prefix, "/")

	limit := q.Limit + q.Offset
	if limit == 0 || limit > listMax {
		limit = listMax
	}

	resp, err := s.S3.ListObjectsV2WithContext(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.Bucket),
		Prefix:  aws.String(s.s3Path(q.Prefix)),
		MaxKeys: aws.Int64(int64(limit)),
	})
	if err != nil {
		return nil, err
	}

	index := q.Offset
	nextValue := func() (dsq.Result, bool) {
		for index >= len(resp.Contents) {
			if !*resp.IsTruncated {
				return dsq.Result{}, false
			}

			index -= len(resp.Contents)

			resp, err = s.S3.ListObjectsV2WithContext(ctx, &s3.ListObjectsV2Input{
				Bucket:            aws.String(s.Bucket),
				Prefix:            aws.String(s.s3Path(q.Prefix)),
				Delimiter:         aws.String("/"),
				MaxKeys:           aws.Int64(listMax),
				ContinuationToken: resp.NextContinuationToken,
			})
			if err != nil {
				return dsq.Result{Error: err}, false
			}
		}

		entry := dsq.Entry{
			Key:  ds.NewKey(*resp.Contents[index].Key).String(),
			Size: int(*resp.Contents[index].Size),
		}
		if !q.KeysOnly {
			value, err := s.Get(ctx, ds.NewKey(entry.Key))
			if err != nil {
				return dsq.Result{Error: err}, false
			}
			entry.Value = value
		}

		index++
		return dsq.Result{Entry: entry}, true
	}

	return dsq.ResultsFromIterator(q, dsq.Iterator{
		Close: func() error {
			return nil
		},
		Next: nextValue,
	}), nil
}

func (s *S3Bucket) Batch(_ context.Context) (ds.Batch, error) {
	return &s3Batch{
		s:          s,
		ops:        make(map[string]batchOp),
		numWorkers: s.Workers,
	}, nil
}

func (s *S3Bucket) Close() error {
	return nil
}

func (s *S3Bucket) s3Path(p string) string {
	return path.Join(s.RootDirectory, p)
}

// readBuckets is the Get/Has/GetSize walk order: write bucket first, then
// any extra buckets. Put/Delete/Query stay on Config.Bucket only.
func (s *S3Bucket) readBuckets() []string {
	n := 1 + len(s.Buckets)
	out := make([]string, 0, n)
	seen := make(map[string]struct{}, n)
	add := func(b string) {
		if b == "" {
			return
		}
		if _, ok := seen[b]; ok {
			return
		}
		seen[b] = struct{}{}
		out = append(out, b)
	}
	add(s.Bucket)
	for _, b := range s.Buckets {
		add(b)
	}
	return out
}

// withReadBuckets calls fn for each read bucket until one succeeds. A
// not-found error continues the walk; any other error stops it.
func (s *S3Bucket) withReadBuckets(fn func(bucket string) error) error {
	for _, bucket := range s.readBuckets() {
		err := fn(bucket)
		if err == nil {
			return nil
		}
		if isNotFound(err) {
			continue
		}
		return err
	}
	return ds.ErrNotFound
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if err == ds.ErrNotFound {
		return true
	}
	s3Err, ok := err.(awserr.Error)
	if !ok {
		return false
	}
	switch s3Err.Code() {
	case s3.ErrCodeNoSuchKey, s3.ErrCodeNoSuchBucket, "NotFound":
		return true
	}
	if reqErr, ok := err.(awserr.RequestFailure); ok && reqErr.StatusCode() == 404 {
		return true
	}
	return false
}

type s3Batch struct {
	s          *S3Bucket
	ops        map[string]batchOp
	numWorkers int
}

type batchOp struct {
	val    []byte
	delete bool
}

func (b *s3Batch) Put(ctx context.Context, k ds.Key, val []byte) error {
	b.ops[k.String()] = batchOp{
		val:    val,
		delete: false,
	}
	return nil
}

func (b *s3Batch) Delete(ctx context.Context, k ds.Key) error {
	b.ops[k.String()] = batchOp{
		val:    nil,
		delete: true,
	}
	return nil
}

func (b *s3Batch) Commit(ctx context.Context) error {
	var (
		deleteObjs []*s3.ObjectIdentifier
		putKeys    []ds.Key
	)
	for k, op := range b.ops {
		if op.delete {
			deleteObjs = append(deleteObjs, &s3.ObjectIdentifier{
				Key: aws.String(k),
			})
		} else {
			putKeys = append(putKeys, ds.NewKey(k))
		}
	}

	numJobs := len(putKeys) + (len(deleteObjs) / deleteMax)
	jobs := make(chan func() error, numJobs)
	results := make(chan error, numJobs)

	numWorkers := b.numWorkers
	if numJobs < numWorkers {
		numWorkers = numJobs
	}

	var wg sync.WaitGroup
	wg.Add(numWorkers)
	defer wg.Wait()

	for w := 0; w < numWorkers; w++ {
		go func() {
			defer wg.Done()
			worker(jobs, results)
		}()
	}

	for _, k := range putKeys {
		jobs <- b.newPutJob(ctx, k, b.ops[k.String()].val)
	}

	if len(deleteObjs) > 0 {
		for i := 0; i < len(deleteObjs); i += deleteMax {
			limit := deleteMax
			if len(deleteObjs[i:]) < limit {
				limit = len(deleteObjs[i:])
			}

			jobs <- b.newDeleteJob(ctx, deleteObjs[i:i+limit])
		}
	}
	close(jobs)

	var errs []string
	for i := 0; i < numJobs; i++ {
		err := <-results
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("s3ds: failed batch operation:\n%s", strings.Join(errs, "\n"))
	}

	return nil
}

func (b *s3Batch) newPutJob(ctx context.Context, k ds.Key, value []byte) func() error {
	return func() error {
		return b.s.Put(ctx, k, value)
	}
}

func (b *s3Batch) newDeleteJob(ctx context.Context, objs []*s3.ObjectIdentifier) func() error {
	return func() error {
		resp, err := b.s.S3.DeleteObjectsWithContext(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(b.s.Bucket),
			Delete: &s3.Delete{
				Objects: objs,
			},
		})
		if err != nil && !isNotFound(err) {
			return err
		}

		var errs []string
		for _, err := range resp.Errors {
			if err.Code != nil && *err.Code == s3.ErrCodeNoSuchKey {
				// idempotent
				continue
			}
			errs = append(errs, err.String())
		}

		if len(errs) > 0 {
			return fmt.Errorf("failed to delete objects: %s", errs)
		}

		return nil
	}
}

func worker(jobs <-chan func() error, results chan<- error) {
	for j := range jobs {
		results <- j()
	}
}

var _ ds.Batching = (*S3Bucket)(nil)
