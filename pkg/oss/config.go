package oss

type UploadPolicy struct {
	Key         string `json:"key"`
	CallbackUrl string `json:"callbackUrl"`
	MaxSize     int64  `json:"maxSize"`
	IsPublic    bool   `json:"isPublic"`
}

type CallbackBody struct {
	Key      string `json:"key"`
	Hash     string `json:"hash"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
}
