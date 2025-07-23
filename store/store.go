package store

import (
	"database/sql"
	"errors"
	"fileserve"
	"log/slog"
	"os"
	"path/filepath"

	_ "github.com/glebarez/go-sqlite"
)

var dbfile = "fileserv.db"
var createTableQuery = "create table if not exists files(hash char(32) not null primary key, size bigint, name varchar(512) not null, contentType varchar(128) not null, data blob);"
var insertFileQuery = "insert into files(hash, size, name, contentType, data) values(?, ?, ?, ?, ?);"
var getFileQuery = "select * from files where hash = ?;"
var deleteFileQuery = "delete from files where hash = ?;"

// Use two SQL tables.  The first table stores metadata, with the primary key being the hash.
// The second table stores the file bytes using the hash as the primary key.

type FileStore interface {
	// AddFile Add a file to the storage.
	AddFile(fd fileserve.FileData) (fileserve.FileMetadata, error)

	// GetFile Retrieves the content addressed by hash, if any.
	GetFile(hash string) (fileserve.FileData, error)

	// DeleteFile Removes a file addressed by hash, if any.
	DeleteFile(hash string) error

	Close()
}

type SQL3FileStore struct {
	db *sql.DB
}

func NewSQL3FileStore(directory string) (*SQL3FileStore, error) {
	err := os.MkdirAll(directory, 0755)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}

	path := filepath.Join(directory, dbfile)

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// create tables if they don't exist
	err = createTables(db)
	if err != nil {
		return nil, errors.Join(errors.New("failed to create tables"), err)
	}
	return &SQL3FileStore{db: db}, nil
}

func ResetDB(directory string) error {
	path := filepath.Join(directory, dbfile)
	return os.Remove(path)
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(createTableQuery)
	if err != nil {
		return logISEAndError(err.Error())
	}
	return nil
}

func (s *SQL3FileStore) AddFile(f fileserve.FileData) (fileserve.FileMetadata, error) {
	slog.Info("add file", "content-type", f.ContentType)
	result, err := s.db.Exec(insertFileQuery, f.Hash, f.Size, f.Name, f.ContentType, f.Data)
	if err != nil {
		return f.FileMetadata, logISEAndError(err.Error())
	}

	count, err := result.RowsAffected()
	if err != nil {
		return f.FileMetadata, logISEAndError(err.Error())
	} else if count == 0 {
		return f.FileMetadata, logNotFoundAndError("no rows affected")
	}

	return f.FileMetadata, nil
}

func (s *SQL3FileStore) GetFile(hash string) (fileserve.FileData, error) {
	row := s.db.QueryRow(getFileQuery, hash)

	fd := fileserve.FileData{}
	err := row.Scan(&fd.Hash, &fd.Size, &fd.Name, &fd.ContentType, &fd.Data)

	if err != nil {
		if errors.Is(sql.ErrNoRows, err) {
			return fd, logNotFoundAndError(err.Error())
		}
		return fd, logISEAndError(err.Error())
	}

	return fd, nil
}

func (s *SQL3FileStore) DeleteFile(hash string) error {
	result, err := s.db.Exec(deleteFileQuery, hash)
	if err != nil {
		return fileserve.InternalServerError
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fileserve.InternalServerError
	} else if count == 0 {
		return fileserve.NotFoundError
	}

	return nil
}

func (s *SQL3FileStore) Close() {
	s.db.Close()
}

func logISEAndError(message string) error {
	slog.Error("internal server error", "mesg", message)
	return fileserve.InternalServerError
}

func logNotFoundAndError(message string) error {
	slog.Info("mesg", message)
	return fileserve.NotFoundError
}
