package api

import "jasperlee/backend/internal/models"

// fileMetaLike 只读判定所需的最小字段集（避免 api 与 models 循环引用外的额外依赖）
type fileMetaLike interface {
	getOwner() string
	getPath() string
	getIsArtifact() bool
}

type fileMetaWrap struct {
	models.FileMeta
}

func (w fileMetaWrap) getOwner() string    { return w.Owner }
func (w fileMetaWrap) getPath() string     { return w.Path }
func (w fileMetaWrap) getIsArtifact() bool { return w.IsArtifact }
