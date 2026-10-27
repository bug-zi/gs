package command

import (
	"github.com/bugzi/gs/internal/edit"
)

func editOpen(ctx *Ctx, path string) error {
	return edit.Open(path, ctx.Cfg)
}
