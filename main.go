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

func NewHttpError(code int, message string) *HttpError {
	return &HttpError{
		StatusCode: code,
		Message:    message,
	}
}

func (e *HttpError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

type RequestLogger struct {
	Request *http.Request
}

func (l *RequestLogger) Print(v interface{}) {
	log.Printf("%s - %s", l.Request.RemoteAddr, v)
}

type FileUploadHandler struct {
	UploadDir string
}

func (h *FileUploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := &RequestLogger{r}
	res, err := h.handleFileUpload(w, r)
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

func (h *FileUploadHandler) handleFileUpload(w http.ResponseWriter, r *http.Request) (*HttpResponse, error) {
	if r.Method != "POST" {
		return nil, NewHttpError(404, "Not Found")
	}

	mr, err := r.MultipartReader()
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

		f, err := h.createFile(part)
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
	}

	return &HttpResponse{200, "File uploaded"}, nil
}

func (h *FileUploadHandler) createFile(part *multipart.Part) (*os.File, error) {
	ts := time.Now().Unix()
	filename := fmt.Sprintf("%d-%s", ts, part.FileName())
	return os.Create(fmt.Sprintf("%s/%s", h.UploadDir, filename))
}

func main() {
	err := start("0.0.0.0:4500", "./uploaded")
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
