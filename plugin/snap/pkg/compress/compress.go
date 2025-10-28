package compress

type CompressMethod int

const (
	// LetterBox 默认
	LetterBox CompressMethod = iota // 0
	// Crop 裁剪
	Crop
	// Stretch 拉伸
	Stretch
)

func Image2Base64() {

}
