package libmangal

import (
	"fmt"
	"io"
)

type Client struct {
	options *ClientOptions
}

func NewClient(options *ClientOptions) *Client {
	client := &Client{
		options: options,
	}

	return client
}

func (c *Client) NewProviderHandleFromReader(reader io.Reader) (*ProviderHandle, error) {
	contents, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	info, err := extractInfo(contents)
	if err != nil {
		return nil, err
	}

	return &ProviderHandle{
		client:    c,
		rawScript: contents,
		info:      info,
	}, nil
}

func (c *Client) NewProviderHandleFromPath(path string) (*ProviderHandle, error) {
	file, err := c.options.FS.Open(path)
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("not a file")
	}

	var buffer = make([]byte, stat.Size())
	_, err = file.Read(buffer)
	if err != nil {
		return nil, err
	}

	info, err := extractInfo(buffer)
	if err != nil {
		return nil, err
	}

	return &ProviderHandle{
		client:    c,
		rawScript: buffer,
		info:      info,
	}, nil
}
