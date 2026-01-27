package mproxy

import "time"

type ConnectionInfo struct {
	Session     int64     `json:"id"`
	ParentSess  int64     `json:"parentId"` // 父连接Session
	Host        string    `json:"host"`
	Method      string    `json:"method"`
	URL         string    `json:"url"`
	RemoteAddr  string    `json:"remote"`
	Protocol    string    `json:"protocol"`  // HTTP / HTTPS-Tunnel / HTTPS-MITM
	StartTime   time.Time `json:"startTime"`
	PuploadRef	*int64	  `json:"-"`
	PdownloadRef *int64   `json:"-"`   
	UploadRef   *int64    `json:"-"`         // 用于读取实时值，这是活动连接流量实时更新的关键
	DownloadRef *int64    `json:"-"`
	OnClose     func()    `json:"-"`
}