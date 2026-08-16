package plugin

import (
	"reflect"
	"testing"

	s3ds "github.com/lighthouse-web3/go-ds-s3"
)

func TestS3PluginDatastoreConfigParser(t *testing.T) {

	testcases := []struct {
		Input  map[string]interface{}
		Want   *S3Config
		HasErr bool
	}{
		{
			// Default case
			Input: map[string]interface{}{
				"region":    "someregion",
				"bucket":    "somebucket",
				"accessKey": "someaccesskey",
				"secretKey": "somesecretkey",
			},
			Want: &S3Config{cfg: s3ds.Config{
				Region:    "someregion",
				Bucket:    "somebucket",
				AccessKey: "someaccesskey",
				SecretKey: "somesecretkey",
			}},
		},
		{
			// Required fields missing
			Input: map[string]interface{}{
				"region": "someregion",
			},
			HasErr: true,
		},
		{
			// Optional fields included
			Input: map[string]interface{}{
				"region":              "someregion",
				"bucket":              "somebucket",
				"accessKey":           "someaccesskey",
				"secretKey":           "somesecretkey",
				"sessionToken":        "somesessiontoken",
				"rootDirectory":       "/some/path",
				"regionEndpoint":      "someendpoint",
				"workers":             42.0,
				"credentialsEndpoint": "somecredendpoint",
			},
			Want: &S3Config{cfg: s3ds.Config{
				Region:              "someregion",
				Bucket:              "somebucket",
				AccessKey:           "someaccesskey",
				SecretKey:           "somesecretkey",
				SessionToken:        "somesessiontoken",
				RootDirectory:       "/some/path",
				RegionEndpoint:      "someendpoint",
				Workers:             42,
				CredentialsEndpoint: "somecredendpoint",
			}},
		},
		{
			// Extra read buckets as a JSON array
			Input: map[string]interface{}{
				"region":    "someregion",
				"bucket":    "writebucket",
				"buckets":   []interface{}{"old-a", "old-b"},
				"accessKey": "someaccesskey",
				"secretKey": "somesecretkey",
			},
			Want: &S3Config{cfg: s3ds.Config{
				Region:    "someregion",
				Bucket:    "writebucket",
				Buckets:   []string{"old-a", "old-b"},
				AccessKey: "someaccesskey",
				SecretKey: "somesecretkey",
			}},
		},
		{
			// Extra read buckets as a comma-separated string
			Input: map[string]interface{}{
				"region":    "someregion",
				"bucket":    "writebucket",
				"buckets":   "old-a, old-b",
				"accessKey": "someaccesskey",
				"secretKey": "somesecretkey",
			},
			Want: &S3Config{cfg: s3ds.Config{
				Region:    "someregion",
				Bucket:    "writebucket",
				Buckets:   []string{"old-a", "old-b"},
				AccessKey: "someaccesskey",
				SecretKey: "somesecretkey",
			}},
		},
		{
			// Empty buckets string is ignored
			Input: map[string]interface{}{
				"region":    "someregion",
				"bucket":    "writebucket",
				"buckets":   "",
				"accessKey": "someaccesskey",
				"secretKey": "somesecretkey",
			},
			Want: &S3Config{cfg: s3ds.Config{
				Region:    "someregion",
				Bucket:    "writebucket",
				AccessKey: "someaccesskey",
				SecretKey: "somesecretkey",
			}},
		},
		{
			// buckets wrong type
			Input: map[string]interface{}{
				"region":    "someregion",
				"bucket":    "writebucket",
				"buckets":   1.0,
				"accessKey": "someaccesskey",
				"secretKey": "somesecretkey",
			},
			HasErr: true,
		},
	}

	for i, tc := range testcases {
		cfg, err := S3Plugin{}.DatastoreConfigParser()(tc.Input)
		if err != nil {
			if tc.HasErr {
				continue
			}
			t.Errorf("case %d: Failed to parse: %s", i, err)
			continue
		}
		if got, ok := cfg.(*S3Config); !ok {
			t.Errorf("wrong config type returned: %T", cfg)
		} else if !reflect.DeepEqual(got, tc.Want) {
			t.Errorf("case %d: got: %v; want %v", i, got, tc.Want)
		}
	}

}

func TestDiskSpecIgnoresReadBuckets(t *testing.T) {
	cfg := &S3Config{cfg: s3ds.Config{
		Region:        "someregion",
		Bucket:        "writebucket",
		Buckets:       []string{"old-a", "old-b"},
		RootDirectory: "",
	}}
	spec := cfg.DiskSpec()
	if spec["bucket"] != "writebucket" {
		t.Fatalf("DiskSpec bucket = %v; want writebucket", spec["bucket"])
	}
	if _, ok := spec["buckets"]; ok {
		t.Fatal("DiskSpec must not include read buckets")
	}
}
