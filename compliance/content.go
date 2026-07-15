package compliance

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"time"
)

// DeletionReceipt is returned by every destructive Compliance content API.
type DeletionReceipt struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type FileMetadata struct {
	ID            string   `json:"id"`
	ClaudeChatIDs []string `json:"claude_chat_ids"`
	CreatedAt     string   `json:"created_at"`
	Filename      string   `json:"filename"`
	MD5           *string  `json:"md5"`
	MessageIDs    []string `json:"message_ids"`
	MimeType      *string  `json:"mime_type"`
	SizeBytes     *int64   `json:"size_bytes"`
}

type GeneratedFileMetadata struct {
	ID           string  `json:"id"`
	ClaudeChatID string  `json:"claude_chat_id"`
	CreatedAt    *string `json:"created_at"`
	Filename     string  `json:"filename"`
	MD5          *string `json:"md5"`
	MimeType     *string `json:"mime_type"`
	SizeBytes    *int64  `json:"size_bytes"`
}

type ChatArtifactMetadata struct {
	ID           string `json:"id"`
	ArtifactType string `json:"artifact_type"`
	ClaudeChatID string `json:"claude_chat_id"`
	CreatedAt    string `json:"created_at"`
	MD5          string `json:"md5"`
	SizeBytes    int64  `json:"size_bytes"`
	Title        string `json:"title"`
	VersionID    string `json:"version_id"`
}

// Download is a streaming content response. The caller must close Body.
type Download struct {
	Body        io.ReadCloser
	Filename    string
	ContentType string
	ContentMD5  string
}

func (c *Client) DeleteChat(ctx context.Context, chatID string) (*DeletionReceipt, error) {
	return c.deleteResource(ctx, "/v1/compliance/apps/chats/", chatID)
}

func (c *Client) GetFileMetadata(ctx context.Context, fileID string) (*FileMetadata, error) {
	if fileID == "" {
		return nil, fmt.Errorf("file ID is required")
	}
	var metadata FileMetadata
	if err := c.get(ctx, "/v1/compliance/apps/chats/files/"+url.PathEscape(fileID), nil, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

func (c *Client) DeleteFile(ctx context.Context, fileID string) (*DeletionReceipt, error) {
	return c.deleteResource(ctx, "/v1/compliance/apps/chats/files/", fileID)
}

func (c *Client) DownloadFileContent(ctx context.Context, fileID string) (*Download, error) {
	return c.download(ctx, "/v1/compliance/apps/chats/files/", fileID, "/content")
}

func (c *Client) GetGeneratedFileMetadata(ctx context.Context, generatedFileID string) (*GeneratedFileMetadata, error) {
	if generatedFileID == "" {
		return nil, fmt.Errorf("generated file ID is required")
	}
	var metadata GeneratedFileMetadata
	endpoint := "/v1/compliance/apps/chats/generated-files/" + url.PathEscape(generatedFileID)
	if err := c.get(ctx, endpoint, nil, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

func (c *Client) DownloadGeneratedFile(ctx context.Context, generatedFileID string) (*Download, error) {
	return c.download(ctx, "/v1/compliance/apps/chats/generated-files/", generatedFileID, "/content")
}

func (c *Client) GetChatArtifactMetadata(ctx context.Context, artifactVersionID string) (*ChatArtifactMetadata, error) {
	if artifactVersionID == "" {
		return nil, fmt.Errorf("artifact version ID is required")
	}
	var metadata ChatArtifactMetadata
	endpoint := "/v1/compliance/apps/artifacts/" + url.PathEscape(artifactVersionID)
	if err := c.get(ctx, endpoint, nil, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

func (c *Client) DownloadChatArtifact(ctx context.Context, artifactVersionID string) (*Download, error) {
	return c.download(ctx, "/v1/compliance/apps/artifacts/", artifactVersionID, "/content")
}

func (c *Client) deleteResource(ctx context.Context, prefix, id string) (*DeletionReceipt, error) {
	if id == "" {
		return nil, fmt.Errorf("resource ID is required")
	}
	var receipt DeletionReceipt
	if err := c.delete(ctx, prefix+url.PathEscape(id), &receipt); err != nil {
		return nil, err
	}
	return &receipt, nil
}

func (c *Client) download(ctx context.Context, prefix, id, suffix string) (*Download, error) {
	if id == "" {
		return nil, fmt.Errorf("resource ID is required")
	}
	endpoint := prefix + url.PathEscape(id) + suffix
	u, err := url.Parse(c.baseURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint %q: %w", endpoint, err)
	}

	for attempt := 0; attempt < 5; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", c.apiKey)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request to %s failed: %w", endpoint, err)
		}
		if shouldRetry(resp.StatusCode, resp.Header) && attempt < 4 {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryDelay(resp.Header.Get("Retry-After"), attempt)):
				continue
			}
		}
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return nil, &APIError{
				StatusCode: resp.StatusCode,
				Endpoint:   endpoint,
				RequestID:  resp.Header.Get("request-id"),
				Body:       string(body),
			}
		}

		filename := id
		if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil {
			if value := params["filename"]; value != "" {
				filename = value
			}
		}
		return &Download{
			Body:        resp.Body,
			Filename:    filename,
			ContentType: resp.Header.Get("Content-Type"),
			ContentMD5:  resp.Header.Get("Content-MD5"),
		}, nil
	}
	return nil, fmt.Errorf("exceeded retry limit for %s", endpoint)
}
