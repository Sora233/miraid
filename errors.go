package miraid

import "github.com/pkg/errors"

var (
	ErrUnknownMethod      = errors.New("unknown method")
	ErrInvalidPasswordMD5 = errors.New("password is invalid md5")
)
