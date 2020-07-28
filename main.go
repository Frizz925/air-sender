package main

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"time"
)

type HttpResponse struct {
	StatusCode int
	Message    string
}

type HttpError struct {
	StatusCode int
	Message    string
}

type RequestLogger struct {
	Request *http.Request
}

type FileUploadHandler struct {
	UploadDir string
}

type FileUploadContext struct {
	Request *http.Request
	Logger  *RequestLogger
}

func NewHttpError(code int, message string) *HttpError {
	return &HttpError{
		StatusCode: code,
		Message:    message,
	}
}

func (e *HttpError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

func (l *RequestLogger) Print(v interface{}) {
	log.Printf("%s - %s", l.Request.RemoteAddr, v)
}

func (l *RequestLogger) Printf(format string, a ...interface{}) {
	l.Print(fmt.Sprintf(format, a...))
}

func (h *FileUploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := &RequestLogger{r}
	context := &FileUploadContext{r, logger}
	res, err := h.handleFileUpload(context)
	if res != nil {
		w.WriteHeader(res.StatusCode)
		_, err := w.Write([]byte(res.Message))
		if err != nil {
			logger.Print(err)
		}
		return
	}
	if err == nil {
		return
	}
	logger.Print(err)
	code := 500
	message := "Internal server error"
	if v, ok := err.(*HttpError); ok {
		code = v.StatusCode
		message = v.Message
	}
	w.WriteHeader(code)
	_, err = w.Write([]byte(message))
	if err != nil {
		logger.Print(err)
	}
}

func (h *FileUploadHandler) handleFileUpload(c *FileUploadContext) (*HttpResponse, error) {
	logger := c.Logger
	req := c.Request

	if req.Method != "POST" {
		return nil, NewHttpError(404, "Not Found")
	}

	mr, err := req.MultipartReader()
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 4096)
	for {
		part, err := mr.NextPart()
		if err != nil {
			if err != io.EOF {
				return nil, err
			}
			break
		}

		filename := h.createFilename(part)
		f, err := h.createFile(filename)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		uploaded := false
		for !uploaded {
			n, err := part.Read(buf)
			if err != nil {
				if err != io.EOF {
					return nil, err
				}
				uploaded = true
			}
			if _, err = f.Write(buf[:n]); err != nil {
				return nil, err
			}
		}
		logger.Printf("File uploaded: %s", filename)
	}

	return &HttpResponse{200, "File uploaded"}, nil
}

func (h *FileUploadHandler) createFilename(part *multipart.Part) string {
	ts := time.Now().Unix()
	return fmt.Sprintf("%d-%s", ts, part.FileName())
}

func (h *FileUploadHandler) createFile(filename string) (*os.File, error) {
	return os.Create(fmt.Sprintf("%s/%s", h.UploadDir, filename))
}

func main() {
	err := start("0.0.0.0:4500", "./uploads")
	if err != nil {
		log.Fatal(err)
	}
}

func start(addr string, uploadDir string) error {
	err := checkUploadDir(uploadDir)
	if err != nil {
		return err
	}
	http.Handle("/", http.FileServer(http.Dir("./public")))
	http.Handle("/upload", &FileUploadHandler{uploadDir})
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	log.Printf("Listening on %s", addr)
	return http.Serve(l, nil)
}

func checkUploadDir(dir string) error {
	_, err := os.Stat(dir)
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return os.Mkdir(dir, 0755)
	}
	return err
}
