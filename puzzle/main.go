// puzzle 是一个基于 ebiten 的 3x3 数字华容道（8-puzzle）小游戏。
//
// 玩法：点击与空格上下左右相邻的方块，方块会平移滑入空格；
// 将数字 1~8 按从左到右、从上到下的顺序排好（空格位于右下角）即过关，
// 过关后右下角自动补上数字 9。
package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	textv2 "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// 窗口与布局常量：游戏区固定 300x300（每格 100x100），底部状态栏 40，
// 窗口整体 300x340，大小固定不可调整。
const (
	gridN        = 3
	cellSize     = 100
	boardSize    = gridN * cellSize // 300
	barHeight    = 40
	screenWidth  = boardSize
	screenHeight = boardSize + barHeight // 340

	tileMargin = 4                      // 方块与格子边缘的留白
	animTime   = 120 * time.Millisecond // 方块平移动画时长

	tileFontSize = 46
	uiFontSize   = 15
)

var (
	bgColor     = color.RGBA{0x16, 0x21, 0x2e, 0xff}
	cellColor   = color.RGBA{0x20, 0x2d, 0x3d, 0xff}
	barColor    = color.RGBA{0x0f, 0x17, 0x21, 0xff}
	sepColor    = color.RGBA{0x2c, 0x3d, 0x51, 0xff}
	textColor   = color.RGBA{0xd8, 0xe2, 0xec, 0xff}
	btnColor    = color.RGBA{0x2e, 0x75, 0xc8, 0xff}
	btnHotColor = color.RGBA{0x46, 0x92, 0xe6, 0xff}
)

// tileColors 是数字 1~9 的方块底色（蓝→青渐变；9 为过关时补上的方块，用绿色）。
var tileColors = [10]color.RGBA{
	{},
	{0x2f, 0x6d, 0xb3, 0xff},
	{0x2e, 0x74, 0xad, 0xff},
	{0x2d, 0x7a, 0xa7, 0xff},
	{0x2d, 0x81, 0xa1, 0xff},
	{0x2c, 0x88, 0x9c, 0xff},
	{0x2b, 0x8f, 0x96, 0xff},
	{0x2a, 0x95, 0x90, 0xff},
	{0x29, 0x9c, 0x8a, 0xff},
	{0x2f, 0x9e, 0x6b, 0xff},
}

// buttonRect 是状态栏中 [再来一局] 按钮的区域。
var buttonRect = image.Rect(screenWidth-98, boardSize+6, screenWidth-8, boardSize+barHeight-6)

// Game 实现 ebiten.Game 接口。
type Game struct {
	board     [gridN * gridN]int // 0 表示空格，1~9 表示方块上的数字
	moves     int                // 本局已平移的步数
	startTime time.Time          // 本局开始时间
	endTime   time.Time          // 过关时刻（用于冻结计时）
	won       bool
	anim      *slideAnim // 当前平移动画，nil 表示空闲

	// 开发辅助：设置 PUZZLE_DUMP=/path/x.png 后，渲染片刻把画面
	// 截图保存并退出，用于自动化视觉验证。日常游玩不设置即可。
	dumpFile      string
	dumpAt        time.Time
	dumpDone      bool
	quitAfterDump bool
}

// slideAnim 表示一次方块平移：方块从 from 格滑向 to 格（to 为空格所在格）。
type slideAnim struct {
	tile      int
	from, to  int
	startTime time.Time
}

func main() {
	if err := loadFonts(); err != nil {
		log.Fatal(err)
	}

	ebiten.SetWindowTitle("puzzle")
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeDisabled)

	g := &Game{}
	g.restart()
	g.dumpFile = os.Getenv("PUZZLE_DUMP")
	if g.dumpFile != "" {
		g.dumpAt = time.Now().Add(1500 * time.Millisecond)
	}
	if err := ebiten.RunGame(g); err != nil && !errors.Is(err, ebiten.Termination) {
		log.Fatal(err)
	}
}

func (g *Game) Update() error {
	if g.quitAfterDump {
		return ebiten.Termination
	}

	// 推进平移动画：时间到则落定、计步并判断是否过关。
	if g.anim != nil && time.Since(g.anim.startTime) >= animTime {
		g.finishAnim()
	}

	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return nil
	}
	x, y := ebiten.CursorPosition()

	// [再来一局] 按钮：任何时候（含动画中、过关后）都可以点击。
	if image.Pt(x, y).In(buttonRect) {
		g.restart()
		return nil
	}

	// 点击游戏区：动画进行中或已过关时不响应。
	if g.anim != nil || g.won || y >= boardSize {
		return nil
	}
	idx := (y/cellSize)*gridN + x/cellSize
	if g.board[idx] == 0 {
		return nil
	}
	if blank := g.blankIndex(); adjacent(idx, blank) {
		g.anim = &slideAnim{
			tile:      g.board[idx],
			from:      idx,
			to:        blank,
			startTime: time.Now(),
		}
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// 棋盘底格
	for i := 0; i < gridN*gridN; i++ {
		x, y := cellPos(i)
		vector.DrawFilledRect(screen, float32(x)+2, float32(y)+2,
			cellSize-4, cellSize-4, cellColor, false)
	}

	// 方块（正在平移的方块画在插值位置上）
	for i, v := range g.board {
		if v == 0 {
			continue
		}
		x, y := cellPos(i)
		fx, fy := float32(x), float32(y)
		if g.anim != nil && g.anim.from == i {
			tx, ty := cellPos(g.anim.to)
			p := easeOut(float64(time.Since(g.anim.startTime)) / float64(animTime))
			fx += float32(p) * float32(tx-x)
			fy += float32(p) * float32(ty-y)
		}
		drawTile(screen, fx, fy, v)
	}

	g.drawStatusBar(screen)

	// 开发辅助：到点后截图并请求退出（见 Game.dumpFile 注释）。
	if g.dumpFile != "" && !g.dumpDone && time.Now().After(g.dumpAt) {
		if err := dumpScreen(g.dumpFile, screen); err != nil {
			log.Print("截图失败: ", err)
		}
		g.dumpDone = true
		g.quitAfterDump = true
	}
}

// dumpScreen 把当前画面保存为 PNG（逐像素读取，仅用于低频截图）。
func dumpScreen(path string, screen *ebiten.Image) error {
	img := image.NewNRGBA(image.Rect(0, 0, screenWidth, screenHeight))
	for y := 0; y < screenHeight; y++ {
		for x := 0; x < screenWidth; x++ {
			img.Set(x, y, screen.At(x, y))
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

// finishAnim 落定当前平移动画：更新棋盘、计步并判断是否过关。
func (g *Game) finishAnim() {
	g.board[g.anim.to] = g.anim.tile
	g.board[g.anim.from] = 0
	g.moves++
	g.anim = nil
	g.checkWin()
}

// restart 开始一局新游戏：重新洗牌并重置步数与计时。
func (g *Game) restart() {
	g.board = shuffledBoard()
	g.moves = 0
	g.startTime = time.Now()
	g.won = false
	g.anim = nil
}

// checkWin 判断过关：1~8 按序排列（此时空格必然已在右下角）。
// 过关后冻结计时，并自动在右下角补上数字 9 的方块。
func (g *Game) checkWin() {
	for i := 0; i < gridN*gridN-1; i++ {
		if g.board[i] != i+1 {
			return
		}
	}
	g.won = true
	g.endTime = time.Now()
	g.board[gridN*gridN-1] = 9
}

// elapsed 返回本局已进行的时间；过关后冻结在过关那一刻。
func (g *Game) elapsed() time.Duration {
	if g.won {
		return g.endTime.Sub(g.startTime)
	}
	return time.Since(g.startTime)
}

func (g *Game) blankIndex() int {
	for i, v := range g.board {
		if v == 0 {
			return i
		}
	}
	return -1 // 不会发生：棋盘上恒有一个空格
}

// drawStatusBar 绘制底部状态栏：步数、耗时与 [再来一局] 按钮。
func (g *Game) drawStatusBar(screen *ebiten.Image) {
	vector.DrawFilledRect(screen, 0, boardSize, screenWidth, barHeight, barColor, false)
	vector.DrawFilledRect(screen, 0, boardSize, screenWidth, 2, sepColor, false)

	msg := fmt.Sprintf("步数 %d   时间 %ds", g.moves, int(g.elapsed().Seconds()))
	drawTextLeft(screen, msg, uiFace, 10, boardSize+barHeight/2, textColor)

	// 按钮：悬停时高亮
	mx, my := ebiten.CursorPosition()
	c := btnColor
	if image.Pt(mx, my).In(buttonRect) {
		c = btnHotColor
	}
	vector.DrawFilledRect(screen, float32(buttonRect.Min.X), float32(buttonRect.Min.Y),
		float32(buttonRect.Dx()), float32(buttonRect.Dy()), c, true)
	drawTextCenter(screen, "再来一局", uiFace,
		float64(buttonRect.Min.X+buttonRect.Dx()/2),
		float64(buttonRect.Min.Y+buttonRect.Dy()/2), color.White)
}

// drawTile 在 (x, y)（所在格子的左上角）绘制数字为 v 的方块。
func drawTile(screen *ebiten.Image, x, y float32, v int) {
	w := float32(cellSize - 2*tileMargin)
	c := tileColors[v]
	vector.DrawFilledRect(screen, x+tileMargin, y+tileMargin, w, w, c, true)
	vector.StrokeRect(screen, x+tileMargin, y+tileMargin, w, w, 2, dimColor(c), true)
	drawTextCenter(screen, strconv.Itoa(v), tileFace,
		float64(x+cellSize/2), float64(y+cellSize/2), color.White)
}

// shuffledBoard 生成一局新棋盘：空格固定在右下角，数字 1~8 随机分布在
// 其余 8 格。为保证每局都有解，要求随机排列的逆序数为偶数（3x3 奇数宽
// 棋盘的可解性只取决于逆序数奇偶）；同时排除开局即已过关的排列。
func shuffledBoard() [gridN * gridN]int {
	nums := make([]int, gridN*gridN-1)
	for i := range nums {
		nums[i] = i + 1
	}
	for {
		rand.Shuffle(len(nums), func(i, j int) {
			nums[i], nums[j] = nums[j], nums[i]
		})
		if inversions(nums)%2 != 0 {
			continue
		}
		var b [gridN * gridN]int
		copy(b[:], nums)
		if b == solvedBoard() {
			continue
		}
		return b
	}
}

func solvedBoard() [gridN * gridN]int {
	var b [gridN * gridN]int
	for i := 0; i < gridN*gridN-1; i++ {
		b[i] = i + 1
	}
	return b
}

// inversions 统计排列中的逆序对个数。
func inversions(nums []int) int {
	n := 0
	for i := range nums {
		for j := i + 1; j < len(nums); j++ {
			if nums[i] > nums[j] {
				n++
			}
		}
	}
	return n
}

// adjacent 判断两个格子的下标是否上下左右相邻。
func adjacent(a, b int) bool {
	return abs(a%gridN-b%gridN)+abs(a/gridN-b/gridN) == 1
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func cellPos(i int) (x, y int) {
	return (i % gridN) * cellSize, (i / gridN) * cellSize
}

// easeOut 是先快后慢的二次缓动，让平移动画更自然。
func easeOut(p float64) float64 {
	if p > 1 {
		p = 1
	}
	return 1 - (1-p)*(1-p)
}

func dimColor(c color.RGBA) color.RGBA {
	return color.RGBA{c.R * 3 / 4, c.G * 3 / 4, c.B * 3 / 4, 0xff}
}

// drawTextCenter 以 (cx, cy) 为中心绘制文本。
func drawTextCenter(dst *ebiten.Image, s string, face textv2.Face, cx, cy float64, clr color.Color) {
	op := &textv2.DrawOptions{}
	op.PrimaryAlign = textv2.AlignCenter
	op.SecondaryAlign = textv2.AlignCenter
	op.GeoM.Translate(cx, cy)
	op.ColorScale.ScaleWithColor(clr)
	textv2.Draw(dst, s, face, op)
}

// drawTextLeft 以 (x, cy) 为左边缘、垂直居中绘制文本。
func drawTextLeft(dst *ebiten.Image, s string, face textv2.Face, x, cy float64, clr color.Color) {
	op := &textv2.DrawOptions{}
	op.PrimaryAlign = textv2.AlignStart
	op.SecondaryAlign = textv2.AlignCenter
	op.GeoM.Translate(x, cy)
	op.ColorScale.ScaleWithColor(clr)
	textv2.Draw(dst, s, face, op)
}

var (
	tileFace *textv2.GoTextFace // 方块数字（粗体）
	uiFace   *textv2.GoTextFace // 状态栏文本（常规）
)

// 候选字体文件按优先级排列：方块数字用 Bold、界面文本用 Regular，
// 各自缺失时兜底到另一组。同时覆盖 Linux 与 Windows 的常见字体路径。
var boldFontCandidates = []string{
	// Linux: Noto Sans CJK Bold / 文泉驿
	"/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc",
	"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
	// Windows: 微软雅黑 Bold
	`C:\Windows\Fonts\msyhbd.ttc`,
}

var regularFontCandidates = []string{
	// Linux: Noto Sans CJK / 文泉驿
	"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
	// Windows: 微软雅黑 / 黑体
	`C:\Windows\Fonts\msyh.ttc`,
	`C:\Windows\Fonts\simhei.ttf`,
}

// loadFonts 加载状态栏与方块数字所需的中文字体（运行时从系统字体目录读取）。
func loadFonts() error {
	tileSrc := loadFontSource(boldFontCandidates)
	if tileSrc == nil {
		tileSrc = loadFontSource(regularFontCandidates)
	}
	uiSrc := loadFontSource(regularFontCandidates)
	if uiSrc == nil {
		uiSrc = tileSrc
	}
	if tileSrc == nil {
		return fmt.Errorf("未找到可用的中文字体（Bold 候选 %v；Regular 候选 %v）",
			boldFontCandidates, regularFontCandidates)
	}
	tileFace = &textv2.GoTextFace{Source: tileSrc, Size: tileFontSize}
	uiFace = &textv2.GoTextFace{Source: uiSrc, Size: uiFontSize}
	return nil
}

// loadFontSource 依次尝试字体文件，优先返回简体（SC）face。
// TTC 集合与单个 TTF 均支持。
func loadFontSource(paths []string) *textv2.GoTextFaceSource {
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if srcs, err := textv2.NewGoTextFaceSourcesFromCollection(bytes.NewReader(data)); err == nil && len(srcs) > 0 {
			for _, s := range srcs {
				if strings.Contains(s.Metadata().Family, "SC") {
					return s
				}
			}
			return srcs[0]
		}
		if src, err := textv2.NewGoTextFaceSource(bytes.NewReader(data)); err == nil {
			return src
		}
	}
	return nil
}
