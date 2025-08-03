package sourceproc

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"sync"

	"m3u-stream-merger/logger"

	"github.com/goccy/go-json"
	"github.com/klauspost/compress/zstd"
	"github.com/puzpuzpuz/xsync/v3"
)

var (
	encoderPool sync.Pool
	decoderPool sync.Pool
)

func init() {
	encoderPool = sync.Pool{
		New: func() interface{} {
			encoder, err := zstd.NewWriter(nil)
			if err != nil {
				logger.Default.DebugEvent().
					Str("component", "SourceProcessor").
					Str("operation", "init_encoder_pool").
					Err(err).
					Msg("Error creating zstd encoder")
				return nil
			}
			return encoder
		},
	}

	decoderPool = sync.Pool{
		New: func() interface{} {
			decoder, err := zstd.NewReader(nil)
			if err != nil {
				logger.Default.DebugEvent().
					Str("component", "SourceProcessor").
					Str("operation", "init_decoder_pool").
					Err(err).
					Msg("Error creating zstd decoder")
				return nil
			}
			return decoder
		},
	}
}

func EncodeSlug(stream *StreamInfo) string {
	jsonData, err := json.Marshal(stream)
	if err != nil {
		logger.Default.DebugEvent().
			Str("component", "SourceProcessor").
			Str("operation", "encode_slug").
			Err(err).
			Msg("Error json marshal for slug")
		return ""
	}

	encoder := encoderPool.Get().(*zstd.Encoder)
	defer encoderPool.Put(encoder)
	encoder.Reset(nil)

	var compressedData bytes.Buffer
	encoder.Reset(&compressedData)

	if _, err := encoder.Write(jsonData); err != nil {
		logger.Default.DebugEvent().
			Str("component", "SourceProcessor").
			Str("operation", "encode_slug").
			Err(err).
			Msg("Error zstd compression for slug")
		return ""
	}
	encoder.Close()

	encodedData := base64.RawURLEncoding.EncodeToString(compressedData.Bytes())
	return encodedData
}

func DecodeSlug(encodedSlug string) (*StreamInfo, error) {
	decodedData, err := base64.RawURLEncoding.DecodeString(encodedSlug)
	if err != nil {
		return nil, fmt.Errorf("error decoding Base64 data: %v", err)
	}

	decoder := decoderPool.Get().(*zstd.Decoder)
	defer decoderPool.Put(decoder)
	_ = decoder.Reset(bytes.NewReader(decodedData))

	decompressedData, err := io.ReadAll(decoder)
	if err != nil {
		return nil, fmt.Errorf("error reading decompressed data: %v", err)
	}

	var result StreamInfo
	if err := json.Unmarshal(decompressedData, &result); err != nil {
		return nil, fmt.Errorf("error deserializing data: %v", err)
	}

	result.URLs = xsync.NewMapOf[string, map[string]string]()
	return &result, nil
}
