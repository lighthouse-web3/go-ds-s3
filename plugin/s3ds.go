package plugin

import (
	"fmt"
	"strings"

	"github.com/ipfs/kubo/plugin"
	"github.com/ipfs/kubo/repo"
	"github.com/ipfs/kubo/repo/fsrepo"
	s3ds "github.com/lighthouse-web3/go-ds-s3"
)

var Plugins = []plugin.Plugin{
	&S3Plugin{},
}

type S3Plugin struct{}

func (s3p S3Plugin) Name() string {
	return "s3-datastore-plugin"
}

func (s3p S3Plugin) Version() string {
	return "0.0.1"
}

func (s3p S3Plugin) Init(env *plugin.Environment) error {
	return nil
}

func (s3p S3Plugin) DatastoreTypeName() string {
	return "s3ds"
}

func (s3p S3Plugin) DatastoreConfigParser() fsrepo.ConfigFromMap {
	return func(m map[string]interface{}) (fsrepo.DatastoreConfig, error) {
		region, ok := m["region"].(string)
		if !ok {
			return nil, fmt.Errorf("s3ds: no region specified")
		}

		bucket, ok := m["bucket"].(string)
		if !ok {
			return nil, fmt.Errorf("s3ds: no bucket specified")
		}

		accessKey, ok := m["accessKey"].(string)
		if !ok {
			return nil, fmt.Errorf("s3ds: no accessKey specified")
		}

		secretKey, ok := m["secretKey"].(string)
		if !ok {
			return nil, fmt.Errorf("s3ds: no secretKey specified")
		}

		// Optional.

		var sessionToken string
		if v, ok := m["sessionToken"]; ok {
			sessionToken, ok = v.(string)
			if !ok {
				return nil, fmt.Errorf("s3ds: sessionToken not a string")
			}
		}

		var endpoint string
		if v, ok := m["regionEndpoint"]; ok {
			endpoint, ok = v.(string)
			if !ok {
				return nil, fmt.Errorf("s3ds: regionEndpoint not a string")
			}
		}
		var rootDirectory string
		if v, ok := m["rootDirectory"]; ok {
			rootDirectory, ok = v.(string)
			if !ok {
				return nil, fmt.Errorf("s3ds: rootDirectory not a string")
			}
		}
		var workers int
		if v, ok := m["workers"]; ok {
			workersf, ok := v.(float64)
			workers = int(workersf)
			switch {
			case !ok:
				return nil, fmt.Errorf("s3ds: workers not a number")
			case workers <= 0:
				return nil, fmt.Errorf("s3ds: workers <= 0: %f", workersf)
			case float64(workers) != workersf:
				return nil, fmt.Errorf("s3ds: workers is not an integer: %f", workersf)
			}
		}
		var credentialsEndpoint string
		if v, ok := m["credentialsEndpoint"]; ok {
			credentialsEndpoint, ok = v.(string)
			if !ok {
				return nil, fmt.Errorf("s3ds: credentialsEndpoint not a string")
			}
		}

		var buckets []string
		if v, ok := m["buckets"]; ok {
			var err error
			buckets, err = parseBuckets(v)
			if err != nil {
				return nil, err
			}
		}

		return &S3Config{
			cfg: s3ds.Config{
				Region:              region,
				Bucket:              bucket,
				Buckets:             buckets,
				AccessKey:           accessKey,
				SecretKey:           secretKey,
				SessionToken:        sessionToken,
				RootDirectory:       rootDirectory,
				Workers:             workers,
				RegionEndpoint:      endpoint,
				CredentialsEndpoint: credentialsEndpoint,
			},
		}, nil
	}
}

// parseBuckets accepts a JSON array or a comma-separated string. The write
// bucket is always tried first on Get; this list is extra read buckets.
func parseBuckets(v interface{}) ([]string, error) {
	switch t := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(t))
		for i, item := range t {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("s3ds: buckets[%d] not a string", i)
			}
			s = strings.TrimSpace(s)
			if s == "" {
				return nil, fmt.Errorf("s3ds: buckets[%d] is empty", i)
			}
			out = append(out, s)
		}
		return out, nil
	case []string:
		out := make([]string, 0, len(t))
		for i, s := range t {
			s = strings.TrimSpace(s)
			if s == "" {
				return nil, fmt.Errorf("s3ds: buckets[%d] is empty", i)
			}
			out = append(out, s)
		}
		return out, nil
	case string:
		if strings.TrimSpace(t) == "" {
			return nil, nil
		}
		parts := strings.Split(t, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			out = append(out, p)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("s3ds: buckets must be a list or comma-separated string")
	}
}

type S3Config struct {
	cfg s3ds.Config
}

func (s3c *S3Config) DiskSpec() fsrepo.DiskSpec {
	return fsrepo.DiskSpec{
		"region":        s3c.cfg.Region,
		"bucket":        s3c.cfg.Bucket,
		"rootDirectory": s3c.cfg.RootDirectory,
	}
}

func (s3c *S3Config) Create(path string) (repo.Datastore, error) {
	return s3ds.NewS3Datastore(s3c.cfg)
}
