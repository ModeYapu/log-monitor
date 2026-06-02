package model

import "time"

// SourceMap represents a uploaded source map file
type SourceMap struct {
	ID          int64     `json:"id"`
	AppID       string    `json:"appId"`
	Release     string    `json:"release"`
	Env         string    `json:"env"`
	BuildID     string    `json:"buildId"`
	FilePath    string    `json:"filePath"`
	OriginalURL string    `json:"originalUrl"`
	FileSize    int64     `json:"fileSize"`
	UploadedAt  time.Time `json:"uploadedAt"`
}

// SourceMapUploadRequest represents the source map upload request
type SourceMapUploadRequest struct {
	AppID       string `json:"appId"`
	Release     string `json:"release"`
	Env         string `json:"env"`
	BuildID     string `json:"buildId"`
	OriginalURL string `json:"originalUrl"`
}

// StackFrame represents a single stack frame
type StackFrame struct {
	Filename    string `json:"filename"`
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	FunctionName string `json:"functionName,omitempty"`
}

// StackTrace represents a full stack trace
type StackTrace struct {
	Frames []StackFrame `json:"frames"`
}

// SourceMapRequest represents a request to deobfuscate a stack trace
type SourceMapRequest struct {
	AppID      string       `json:"appId"`
	Release    string       `json:"release"`
	Env        string       `json:"env"`
	BuildID    string       `json:"buildId"`
	StackTrace StackTrace   `json:"stackTrace"`
}
