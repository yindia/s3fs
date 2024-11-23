package route

import (
	"context"
	"s3fs/pkg/cache"
	"s3fs/pkg/filesystem"
	v1 "s3fs/pkg/gen/cloud/v1"
	"testing"

	"connectrpc.com/connect"
	"github.com/bufbuild/protovalidate-go"
	"github.com/stretchr/testify/assert"
)

func TestUpload(t *testing.T) {
	mockCache := cache.NewCache("memory", nil)
	validator, err := protovalidate.New()
	assert.NoError(t, err)

	storage := &Storage{
		cache:         mockCache,
		filesystem:    &filesystem.FilesystemImpl{},
		validator:     validator,
		DataDirectory: "testdata",
	}

	req := connect.NewRequest(&v1.UploadRequestMsg{
		ObjectKey: "testfile.txt",
		Data:      []byte("test data"),
	})

	resp, err := storage.Upload(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, resp.Msg.Status)

}

func TestGet(t *testing.T) {
	mockCache := cache.NewCache("memory", nil)

	validator, err := protovalidate.New()
	assert.NoError(t, err)

	storage := &Storage{
		cache:         mockCache,
		filesystem:    &filesystem.FilesystemImpl{},
		validator:     validator,
		DataDirectory: "testdata",
	}

	req := connect.NewRequest(&v1.GetObjectRequest{
		ObjectKey: "testfile.txt",
	})

	resp, err := storage.Get(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, "test data", string(resp.Msg.Data))

}

func TestDelete(t *testing.T) {
	mockCache := cache.NewCache("memory", nil)

	validator, err := protovalidate.New()
	assert.NoError(t, err)

	storage := &Storage{
		cache:         mockCache,
		filesystem:    &filesystem.FilesystemImpl{},
		validator:     validator,
		DataDirectory: "testdata",
	}

	req := connect.NewRequest(&v1.DeleteRequestMsg{
		ObjectKey: "testfile.txt",
	})

	resp, err := storage.Delete(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, resp.Msg.Status)

}

func TestList(t *testing.T) {
	mockCache := cache.NewCache("memory", nil)

	validator, err := protovalidate.New()
	assert.NoError(t, err)

	storage := &Storage{
		cache:         mockCache,
		filesystem:    &filesystem.FilesystemImpl{},
		validator:     validator,
		DataDirectory: "testdata",
	}

	// Add a file to the storage to ensure it can be listed
	uploadReq := connect.NewRequest(&v1.UploadRequestMsg{
		ObjectKey: "testfile.txt",
		Data:      []byte("test data"),
	})
	_, err = storage.Upload(context.Background(), uploadReq)
	assert.NoError(t, err)

	req := connect.NewRequest(&v1.ListObjectsRequest{})

	resp, err := storage.List(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, []string{"testfile.txt"}, resp.Msg.ObjectKeys)

}
