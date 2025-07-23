package fileserve

type CodedHttpError interface {
	error
	HttpCode() int
}

// HttpError allows client code to distinguish between 400 errors and 500 errors.
type HttpError struct {
	Code    int
	Message string
}

func (h HttpError) HttpCode() int {
	return h.Code
}

func (h HttpError) Error() string {
	return h.Message
}

var InternalServerError = HttpError{500, "Internal ServerError"}
var NotFoundError = HttpError{404, "Not found"}
