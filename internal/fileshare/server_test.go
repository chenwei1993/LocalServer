package fileshare

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) (*Server, *httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	s := New()
	s.root = root
	ts := httptest.NewServer(s.routes())
	t.Cleanup(ts.Close)
	return s, ts, root
}

func upload(t *testing.T, ts *httptest.Server, dir, name, body string) UploadResponse {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost,
		ts.URL+"/api/upload?dir="+dir+"&name="+name, strings.NewReader(body))
	if err != nil {
		t.Fatalf("创建请求失败: %v", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("上传请求失败: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(res.Body)
		t.Fatalf("上传返回 %d: %s", res.StatusCode, data)
	}
	var out UploadResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("解析上传响应失败: %v", err)
	}
	return out
}

func TestUploadListDownloadDelete(t *testing.T) {
	_, ts, root := newTestServer(t)

	first := upload(t, ts, "", "hello.txt", "hello world")
	if first.Name != "hello.txt" || first.Size != 11 || first.Dir != "." {
		t.Fatalf("首次上传结果异常: %+v", first)
	}

	// 同名文件应自动重命名，不覆盖原文件。
	second := upload(t, ts, "", "hello.txt", "second")
	if second.Name != "hello (1).txt" {
		t.Fatalf("同名文件应重命名，实际为 %q", second.Name)
	}

	res, err := http.Get(ts.URL + "/api/list?dir=")
	if err != nil {
		t.Fatalf("列表请求失败: %v", err)
	}
	var list ListResponse
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatalf("解析列表失败: %v", err)
	}
	res.Body.Close()
	if list.Dir != "." || len(list.Entries) != 2 {
		t.Fatalf("列表结果异常: %+v", list)
	}

	res, err = http.Get(ts.URL + "/api/download?path=hello.txt")
	if err != nil {
		t.Fatalf("下载请求失败: %v", err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || string(body) != "hello world" {
		t.Fatalf("下载内容异常: %d %q", res.StatusCode, body)
	}
	if cd := res.Header.Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Fatalf("应作为附件下载，实际: %q", cd)
	}

	// 子目录：创建、上传、下载、删除。
	mkdirReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/mkdir",
		bytes.NewBufferString(`{"dir":"","name":"docs"}`))
	mkdirReq.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(mkdirReq)
	if err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("创建目录返回 %d", res.StatusCode)
	}

	sub := upload(t, ts, "docs", "note.md", "# hi")
	if sub.Dir != "docs" {
		t.Fatalf("子目录上传位置异常: %+v", sub)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "note.md")); err != nil {
		t.Fatalf("文件未写入子目录: %v", err)
	}

	delReq, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/delete",
		bytes.NewBufferString(`{"path":"docs"}`))
	delReq.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("删除目录失败: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("删除目录返回 %d", res.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(root, "docs")); !os.IsNotExist(err) {
		t.Fatalf("目录未被删除: %v", err)
	}
}

func TestDownloadRejectsTraversal(t *testing.T) {
	_, ts, root := newTestServer(t)

	secret := filepath.Join(filepath.Dir(root), "outside.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatalf("准备测试文件失败: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(secret) })

	for _, p := range []string{
		"../outside.txt",
		"..%2Foutside.txt",
		"..\\outside.txt",
		"sub/../../outside.txt",
		"C:/Windows/win.ini",
	} {
		res, err := http.Get(ts.URL + "/api/download?path=" + p)
		if err != nil {
			t.Fatalf("请求失败: %v", err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode == http.StatusOK && string(body) == "secret" {
			t.Fatalf("路径 %q 越界读取到了文件", p)
		}
	}
}

func TestResolveStaysInsideRoot(t *testing.T) {
	s := New()
	s.root = t.TempDir()

	cases := map[string]string{
		"":            ".",
		"/":           ".",
		"a.txt":       "a.txt",
		"a/b.txt":     "a/b.txt",
		"./a/./b.txt": "a/b.txt",
		"../secret":   "secret",
		"a/../../x":   "x",
	}
	for input, want := range cases {
		full, rel, err := s.resolve(input)
		if err != nil {
			t.Fatalf("resolve(%q) 出错: %v", input, err)
		}
		if rel != want {
			t.Fatalf("resolve(%q) = %q，期望 %q", input, rel, want)
		}
		if !strings.HasPrefix(full, s.root) {
			t.Fatalf("resolve(%q) 逃出共享目录: %q", input, full)
		}
	}

	if _, _, err := s.resolve("C:/Windows"); err == nil {
		t.Fatal("带盘符的路径应被拒绝")
	}
}

func TestCleanFileName(t *testing.T) {
	valid := map[string]string{
		"a.txt":      "a.txt",
		"  b.md  ":   "b.md",
		"../c.txt":   "c.txt",
		"dir\\d.txt": "d.txt",
		"name.":      "name",
	}
	for input, want := range valid {
		got, err := cleanFileName(input)
		if err != nil || got != want {
			t.Fatalf("cleanFileName(%q) = %q, %v；期望 %q", input, got, err, want)
		}
	}

	for _, input := range []string{"", "..", ".", "a/b:c.txt", "a<b>.txt", "..."} {
		if got, err := cleanFileName(input); err == nil {
			t.Fatalf("cleanFileName(%q) 应报错，实际返回 %q", input, got)
		}
	}
}

func TestSameOriginBlocksCrossSite(t *testing.T) {
	_, ts, _ := newTestServer(t)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/upload?name=x.txt", strings.NewReader("x"))
	req.Header.Set("Origin", "http://evil.example")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("跨站请求应被拒绝，实际返回 %d", res.StatusCode)
	}
}

func TestIndexPageServed(t *testing.T) {
	_, ts, _ := newTestServer(t)

	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("请求首页失败: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("首页返回 %d", res.StatusCode)
	}
	if !strings.Contains(string(body), "局域网文件传输") {
		t.Fatal("首页内容异常")
	}
}
