// Package fileshare 实现局域网文件共享服务：在指定端口上提供一个内嵌网页，
// 让同一局域网内的设备可以浏览、上传、下载和删除共享目录中的文件。
package fileshare

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

//go:embed web/index.html
var pageFS embed.FS

// maxJSONBody 限制 JSON 接口的请求体大小，文件上传接口不受此限制。
const maxJSONBody = 1 << 20

var (
	// ErrAlreadyRunning 表示服务已经在运行。
	ErrAlreadyRunning = errors.New("共享服务已在运行")
	// ErrNotRunning 表示服务尚未启动。
	ErrNotRunning  = errors.New("共享服务尚未启动")
	errInvalidPath = errors.New("路径无效")
)

// Entry 是文件列表中的一项。
type Entry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"modTime"`
}

// ListResponse 是 /api/list 的响应体。
type ListResponse struct {
	Dir     string  `json:"dir"`
	Parent  string  `json:"parent"`
	Entries []Entry `json:"entries"`
}

// UploadResponse 是 /api/upload 的响应体。
type UploadResponse struct {
	Name string `json:"name"`
	Dir  string `json:"dir"`
	Size int64  `json:"size"`
}

// Status 描述服务的当前状态。
type Status struct {
	Running bool     `json:"running"`
	Port    int      `json:"port"`
	Root    string   `json:"root"`
	URLs    []string `json:"urls"`
	Error   string   `json:"error"`
}

// Server 是局域网文件共享服务。
type Server struct {
	mu      sync.RWMutex
	root    string
	port    int
	httpSrv *http.Server
	ln      net.Listener
	lastErr error
}

// New 创建一个尚未启动的服务实例。
func New() *Server {
	return &Server{}
}

// Start 在指定端口上开始提供共享服务。
func (s *Server) Start(root string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("端口 %d 不合法，应为 1-65535", port)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("共享目录无效: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		return fmt.Errorf("无法创建共享目录: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ln != nil {
		return ErrAlreadyRunning
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		if isAddrInUse(err) {
			return fmt.Errorf("端口 %d 已被占用，请换一个端口", port)
		}
		return fmt.Errorf("无法监听端口 %d: %w", port, err)
	}

	s.root = absRoot
	s.port = port
	s.ln = ln
	s.lastErr = nil
	s.httpSrv = &http.Server{
		Handler:           s.routes(),
		ReadHeaderTimeout: 20 * time.Second,
	}

	srv := s.httpSrv
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.mu.Lock()
			s.lastErr = err
			s.mu.Unlock()
			log.Printf("fileshare: 服务异常退出: %v", err)
		}
	}()

	log.Printf("fileshare: 正在共享 %s（端口 %d）", absRoot, port)
	return nil
}

// Stop 停止服务，并等待进行中的请求结束（最多 3 秒）。
func (s *Server) Stop() error {
	s.mu.Lock()
	if s.ln == nil {
		s.mu.Unlock()
		return ErrNotRunning
	}
	srv, ln := s.httpSrv, s.ln
	s.httpSrv, s.ln = nil, nil
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if srv != nil {
		_ = srv.Shutdown(ctx)
	}
	_ = ln.Close()
	return nil
}

// Root 返回当前共享目录。
func (s *Server) Root() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.root
}

// Status 返回服务状态快照。
func (s *Server) Status() Status {
	s.mu.RLock()
	running := s.ln != nil
	root, port, lastErr := s.root, s.port, s.lastErr
	s.mu.RUnlock()

	st := Status{Running: running, Port: port, Root: root, URLs: []string{}}
	if running {
		st.URLs = LANURLs(port)
	}
	if lastErr != nil {
		st.Error = lastErr.Error()
	}
	return st
}

// LANURLs 返回本机所有非回环 IPv4 地址对应的访问链接。
func LANURLs(port int) []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return []string{}
	}

	seen := make(map[string]bool)
	urls := make([]string, 0, 4)
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			ip4 := ip.To4()
			if ip4 == nil || ip4.IsLoopback() || ip4.IsLinkLocalUnicast() {
				continue
			}
			u := fmt.Sprintf("http://%s:%d", ip4.String(), port)
			if !seen[u] {
				seen[u] = true
				urls = append(urls, u)
			}
		}
	}
	sort.Strings(urls)
	return urls
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/list", s.handleList)
	mux.HandleFunc("/api/upload", s.handleUpload)
	mux.HandleFunc("/api/download", s.handleDownload)
	mux.HandleFunc("/api/mkdir", s.handleMkdir)
	mux.HandleFunc("/api/delete", s.handleDelete)
	return sameOrigin(mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)
		return
	}
	data, err := pageFS.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "页面加载失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	full, rel, err := s.resolve(r.URL.Query().Get("dir"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	info, err := os.Stat(full)
	if err != nil {
		writeError(w, http.StatusNotFound, "目录不存在")
		return
	}
	if !info.IsDir() {
		writeError(w, http.StatusBadRequest, "目标不是目录")
		return
	}

	dirents, err := os.ReadDir(full)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "无法读取目录")
		return
	}

	entries := make([]Entry, 0, len(dirents))
	for _, de := range dirents {
		fi, err := de.Info()
		if err != nil {
			continue
		}
		entries = append(entries, Entry{
			Name:    de.Name(),
			IsDir:   de.IsDir(),
			Size:    fi.Size(),
			ModTime: fi.ModTime().UnixMilli(),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	parent := ""
	if rel != "." {
		if p := path.Dir(rel); p != "." {
			parent = p
		}
	}

	writeJSON(w, http.StatusOK, ListResponse{Dir: rel, Parent: parent, Entries: entries})
}

// handleUpload 接收原始请求体作为文件内容，文件名与目标目录通过查询参数传入：
//
//	POST /api/upload?dir=docs&name=hello.txt
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	q := r.URL.Query()
	name, err := cleanFileName(q.Get("name"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dirFull, dirRel, err := s.resolve(q.Get("dir"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	info, err := os.Stat(dirFull)
	if err != nil || !info.IsDir() {
		writeError(w, http.StatusNotFound, "目标目录不存在")
		return
	}

	f, finalName, err := createExclusive(filepath.Join(dirFull, name))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "无法创建文件")
		return
	}

	written, copyErr := io.Copy(f, r.Body)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(filepath.Join(dirFull, finalName))
		writeError(w, http.StatusInternalServerError, "写入文件失败")
		return
	}

	writeJSON(w, http.StatusOK, UploadResponse{Name: finalName, Dir: dirRel, Size: written})
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	full, _, err := s.resolve(r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	info, err := os.Lstat(full)
	if err != nil {
		writeError(w, http.StatusNotFound, "文件不存在")
		return
	}
	if info.IsDir() {
		writeError(w, http.StatusBadRequest, "不能下载文件夹")
		return
	}
	// 不跟随符号链接，避免共享目录内的链接指向目录外。
	if info.Mode()&os.ModeSymlink != 0 {
		writeError(w, http.StatusForbidden, "不能下载符号链接")
		return
	}

	f, err := os.Open(full)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "无法打开文件")
		return
	}
	defer f.Close()

	inline := r.URL.Query().Get("inline") == "1" && previewable(info.Name())
	w.Header().Set("Content-Disposition", contentDisposition(info.Name(), inline))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

func (s *Server) handleMkdir(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	var req struct {
		Dir  string `json:"dir"`
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	name, err := cleanFileName(req.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dirFull, dirRel, err := s.resolve(req.Dir)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if info, err := os.Stat(dirFull); err != nil || !info.IsDir() {
		writeError(w, http.StatusNotFound, "目标目录不存在")
		return
	}

	target := filepath.Join(dirFull, name)
	if err := os.Mkdir(target, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			writeJSON(w, http.StatusOK, map[string]any{"path": path.Join(dirRel, name), "existed": true})
			return
		}
		writeError(w, http.StatusInternalServerError, "创建文件夹失败")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"path": path.Join(dirRel, name), "existed": false})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	full, rel, err := s.resolve(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if rel == "." {
		writeError(w, http.StatusBadRequest, "不能删除共享根目录")
		return
	}

	info, err := os.Lstat(full)
	if err != nil {
		writeError(w, http.StatusNotFound, "文件不存在")
		return
	}

	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		err = os.Remove(full)
	} else {
		err = os.RemoveAll(full)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "删除失败")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// resolve 把客户端传入的相对路径转换成共享目录内的绝对路径。
// 返回的第二个值是规范化后的相对路径（根目录为 "."）。
func (s *Server) resolve(rel string) (string, string, error) {
	root := s.Root()
	if root == "" {
		return "", "", errors.New("共享目录未设置")
	}

	clean := strings.ReplaceAll(strings.TrimSpace(rel), "\\", "/")
	clean = strings.TrimPrefix(clean, "/")
	clean = path.Clean("/" + clean)
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" {
		clean = "."
	}
	// Windows 下冒号可指向盘符或备用数据流，直接拒绝。
	if strings.Contains(clean, ":") || strings.ContainsRune(clean, 0) {
		return "", "", errInvalidPath
	}

	full := filepath.Join(root, filepath.FromSlash(clean))
	back, err := filepath.Rel(root, full)
	if err != nil || back == ".." || strings.HasPrefix(back, ".."+string(os.PathSeparator)) {
		return "", "", errInvalidPath
	}
	return full, clean, nil
}

// cleanFileName 校验客户端提供的文件名/文件夹名。
func cleanFileName(raw string) (string, error) {
	name := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	name = strings.TrimRight(path.Base(name), " .")
	if name == "" || name == "." || name == ".." || name == "/" {
		return "", errors.New("名称无效")
	}
	if strings.ContainsAny(name, `<>:"/\|?*`) {
		return "", errors.New(`名称不能包含 < > : " / \ | ? * 等字符`)
	}
	if utf8.RuneCountInString(name) > 200 {
		return "", errors.New("名称过长")
	}
	return name, nil
}

// createExclusive 以独占方式创建文件，若同名文件已存在则自动追加序号。
func createExclusive(full string) (*os.File, string, error) {
	dir := filepath.Dir(full)
	base := filepath.Base(full)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	for i := 0; i < 1000; i++ {
		name := base
		if i > 0 {
			name = fmt.Sprintf("%s (%d)%s", stem, i, ext)
		}
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			return f, name, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", errors.New("同名文件过多")
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, maxJSONBody))
	return dec.Decode(v)
}

// sameOrigin 拒绝来自其他站点的写请求，降低 CSRF 风险。
func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !strings.EqualFold(u.Host, r.Host) {
				writeError(w, http.StatusForbidden, "跨站请求已被拒绝")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// previewable 判断文件是否适合在浏览器中直接预览（避免在共享站点内执行 HTML/SVG 脚本）。
func previewable(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".ico",
		".mp4", ".webm", ".mov", ".mp3", ".wav", ".ogg", ".m4a",
		".pdf":
		return true
	}
	return false
}

func contentDisposition(name string, inline bool) string {
	disp := "attachment"
	if inline {
		disp = "inline"
	}

	var fallback strings.Builder
	for _, r := range name {
		if r < 32 || r > 126 || r == '"' || r == '\\' {
			fallback.WriteRune('_')
			continue
		}
		fallback.WriteRune(r)
	}

	encoded := strings.ReplaceAll(url.QueryEscape(name), "+", "%20")
	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, disp, fallback.String(), encoded)
}

func isAddrInUse(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "address already in use") ||
		strings.Contains(msg, "only one usage of each socket address")
}
