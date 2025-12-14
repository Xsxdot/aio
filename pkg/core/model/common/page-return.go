package common

type PageReturn struct {
	Total   int64       `json:"total"`
	Content interface{} `json:"content"`
}
