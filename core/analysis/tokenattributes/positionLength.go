package tokenattributes

import (
	"github.com/gzg1984/golucene/core/util"
)

type PositionLengthAttribute interface {
	util.Attribute
	SetPositionLength(int)
}
