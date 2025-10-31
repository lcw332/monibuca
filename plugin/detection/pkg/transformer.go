package pkg

import (
	task "github.com/langhuihui/gotask"
	"m7s.live/v5"
)

type Transformer struct {
	task.Job
	TransformJob m7s.TransformJob
}

func (r *Transformer) GetTransformJob() *m7s.TransformJob {
	return &r.TransformJob
}

func NewTransform() m7s.ITransformer {
	return &Transformer{}
}
