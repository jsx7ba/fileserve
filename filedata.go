package fileserve

type FileMetadata struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	Hash        string `json:"hash"`
	ContentType string `json:"contentType"`
}

type FileData struct {
	FileMetadata
	Data []byte
}
