package webdav

const (
	ErrUnknown = iota
	ErrInvalid
	ErrPermission
	ErrExist
	ErrNotExist
)

type Error struct {
	Op   string
	Path string
	Type int
	Msg  string
}

func (e *Error) Error() string {
	return e.Op + " " + e.Path + ": " + e.Msg
}

func TypeOf(err error) int {
	if err == nil {
		return -1
	}

	e, ok := err.(*Error)
	if !ok {
		return ErrUnknown
	}

	return e.Type
}

func TypeEqual(err error, t int) bool {
	return TypeOf(err) == t
}

func IsInvalid(err error) bool {
	return TypeEqual(err, ErrInvalid)
}

func IsPermission(err error) bool {
	return TypeEqual(err, ErrPermission)
}

func IsExist(err error) bool {
	return TypeEqual(err, ErrExist)
}

func IsNotExist(err error) bool {
	return TypeEqual(err, ErrNotExist)
}
