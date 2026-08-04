package feishu

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Test_FetchImage_DownloadFile 验证 T005 修复：FetchImage/DownloadFile 能正确读取响应体全部内容。
// 修复前使用空 slice 调 res.Body.Read，永远读 0 字节。
func Test_FetchImage_DownloadFile(t *testing.T) {
	imageContent := []byte("fake-image-bytes-12345")
	fileContent := []byte("fake-file-content-abcdef")

	mux := http.NewServeMux()
	mux.HandleFunc("/open-apis/image/v4/get", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("image_key") != "img-test-key" {
			t.Errorf("unexpected image_key: %s", r.URL.Query().Get("image_key"))
		}
		_, _ = w.Write(imageContent)
	})
	mux.HandleFunc("/open-apis/im/v1/files/file-test-key", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(fileContent)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	bot := &FeishuGroupBot{
		addr:   srv.URL,
		token:  "test-token",
		client: srv.Client(),
	}

	got, err := bot.FetchImage("img-test-key")
	if err != nil {
		t.Fatalf("FetchImage failed: %v", err)
	}
	if string(got) != string(imageContent) {
		t.Fatalf("FetchImage content mismatch, got %d bytes: %q, want %q", len(got), got, imageContent)
	}

	got, err = bot.DownloadFile("file-test-key")
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}
	if string(got) != string(fileContent) {
		t.Fatalf("DownloadFile content mismatch, got %d bytes: %q, want %q", len(got), got, fileContent)
	}

	fmt.Println("T005 verified: FetchImage/DownloadFile read full body")
}
