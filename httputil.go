package libmangal

import (
	"io"
)

func readResponseBody(contentLength int64, body io.Reader) ([]byte, error) {
	if contentLength > 0 {
		buffer := make([]byte, contentLength)
		_, err := io.ReadFull(body, buffer)
		if err != nil {
			return nil, err
		}

		return buffer, nil
	}

	return io.ReadAll(body)
}
