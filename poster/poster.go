package poster

import (
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io/ioutil"
	"os"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/skip2/go-qrcode"
)

//新PNG载体
type Rect struct {
	X0 int
	X1 int
	Y0 int
	Y1 int
}

// Pt 坐标
type Pt struct {
	X int
	Y int
}

// DImage 图片切片
type DImage struct {
	PNG draw.Image //合并到的PNG切片,可用image.NewrRGBA设置
	X   int        //横坐标
	Y   int        //纵坐标
}

// DText 文字切片
type DText struct {
	PNG   draw.Image //合并到的PNG切片,可用image.NewrRGBA设置
	Title string     //文字
	X     int        //横坐标
	Y     int        //纵坐标
	Size  float64
	R     uint8
	G     uint8
	B     uint8
	A     uint8
}

// NewMerged 新建文件载体
func NewMerged(path string) (*os.File, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// NewPNG 新建图片载体
func NewPNG(X0 int, Y0 int, X1 int, Y1 int) *image.RGBA {
	return image.NewRGBA(image.Rect(X0, Y0, X1, Y1))
}

// MergeImage 合并图片到载体
func MergeImage(PNG draw.Image, image image.Image, imageBound image.Point) {
	draw.Draw(PNG, PNG.Bounds(), image, imageBound, draw.Over)
}

// LoadTextType 读取字体类型
func LoadTextType(path string) (*truetype.Font, error) {
	fbyte, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	trueTypeFont, err := freetype.ParseFont(fbyte)
	if err != nil {
		return nil, err
	}
	return trueTypeFont, nil
}

// NewDrawText 创建新字体切片
func NewDrawText(png draw.Image) *DText {
	return &DText{
		PNG:  png,
		Size: 18,
		X:    0,
		Y:    0,
		R:    0,
		G:    0,
		B:    0,
		A:    255,
	}
}

// SetColor 设置字体颜色
func (dtext *DText) SetColor(R uint8, G uint8, B uint8) {
	dtext.R = R
	dtext.G = G
	dtext.B = B
}

// MergeText 合并字体到载体
func (dtext *DText) MergeText(title string, tf *truetype.Font, x int, y int, rect image.Rectangle) error {
	fc := freetype.NewContext()
	//设置屏幕每英寸的分辨率
	fc.SetDPI(72)
	//设置用于绘制文本的字体
	fc.SetFont(tf)
	//以磅为单位设置字体大小
	fc.SetFontSize(dtext.Size)
	//设置剪裁矩形以进行绘制
	fc.SetClip(rect)
	//设置目标图像
	fc.SetDst(dtext.PNG)
	//设置绘制操作的源图像，通常为 image.Uniform
	fc.SetSrc(image.NewUniform(color.RGBA{dtext.R, dtext.G, dtext.B, dtext.A}))

	pt := freetype.Pt(x, y)
	_, err := fc.DrawString(title, pt)
	if err != nil {
		return err
	}
	return nil
}

// Merge 合并到图片
func Merge(png draw.Image, merged *os.File) error {
	err := jpeg.Encode(merged, png, nil)
	if err != nil {
		return err
	}
	return nil
}

// GetQRImage 获取二维码图像
func GetQRImage(url string, level qrcode.RecoveryLevel, size int) (image.Image, error) {
	newQr, err := qrcode.New(url, level)
	if err != nil {
		return nil, err
	}
	qrImage := newQr.Image(size)
	return qrImage, nil
}
