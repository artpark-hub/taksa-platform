package topicbrowser

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// DecodeStatusPayload decodes persisted Push message content into a JSON object
// (same pipeline as syncProtocolConvertersFromStatusMessage: optional outer base64,
// optional gzip/zstd, then JSON object with optional Payload wrapper).
func DecodeStatusPayload(messageContent string) (map[string]interface{}, error) {
	decodedBytes := []byte(messageContent)
	if messageContent != "" {
		if b, err := base64.StdEncoding.DecodeString(messageContent); err == nil && len(b) > 0 {
			decodedBytes = b
		}
	}
	if b, err := decompressIfNeeded(decodedBytes); err != nil {
		return nil, fmt.Errorf("decompress status: %w", err)
	} else if len(b) > 0 {
		decodedBytes = b
	}
	var messageData map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(decodedBytes))
	dec.UseNumber()
	if err := dec.Decode(&messageData); err != nil {
		return nil, fmt.Errorf("json decode: %w", err)
	}
	if messageData == nil {
		return nil, fmt.Errorf("empty json")
	}
	payload := messageData
	if p, ok := messageData["Payload"].(map[string]interface{}); ok {
		payload = p
	} else if p, ok := messageData["payload"].(map[string]interface{}); ok {
		payload = p
	}
	return payload, nil
}

func decompressIfNeeded(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	if len(data) >= 4 && data[0] == 0x28 && data[1] == 0xb5 && data[2] == 0x2f && data[3] == 0xfd {
		decoder, err := zstd.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer decoder.Close()
		return io.ReadAll(decoder)
	}
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(reader)
	}
	return data, nil
}

func jsonMap(m map[string]interface{}, keys ...string) map[string]interface{} {
	if m == nil {
		return nil
	}
	for _, k := range keys {
		if v, ok := m[k].(map[string]interface{}); ok {
			return v
		}
		for alt, val := range m {
			if strings.EqualFold(alt, k) {
				if v, ok := val.(map[string]interface{}); ok {
					return v
				}
			}
		}
	}
	return nil
}
