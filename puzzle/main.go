// puzzle 是一个基于 ebiten 的数字华容道（sliding puzzle）小游戏，
// 支持 3x3 与 4x4 两种规模。
//
// 玩法：点击与空格上下左右相邻的方块，方块会平移滑入空格；
// 将数字 1~n²-1 按从左到右、从上到下的顺序排好（空格位于右下角）即过关，
// 过关后右下角自动补上数字 n²（9 或 16）。底部状态栏可通过
// [3x3] / [4x4] 按钮随时开一局对应规模的新游戏。
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

// 窗口与布局常量：游戏区固定 300x300（3x3 每格 100，4x4 每格 75），
// 底部状态栏 40，窗口整体 300x340，大小固定不可调整。
const (
	boardSize    = 300
	barHeight    = 40
	screenWidth  = boardSize
	screenHeight = boardSize + barHeight // 340

	tileMargin = 4                      // 方块与格子边缘的留白
	animTime   = 120 * time.Millisecond // 方块平移动画时长

	tileFontRatio = 0.46 // 方块数字字号 = 格子边长 × 该比例
	uiFontSize    = 15
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

// 方块底色渐变端点：数字 1 为蓝，最大数字（8 或 15）为青；
// winTileColor 是过关时补上的最后一块（9 或 16）的绿色。
var (
	gradientStart = color.RGBA{0x2f, 0x6d, 0xb3, 0xff}
	gradientEnd   = color.RGBA{0x29, 0x9c, 0x8a, 0xff}
	winTileColor  = color.RGBA{0x2f, 0x9e, 0x6b, 0xff}
)

// 状态栏右侧的规模按钮区域：点击对应按钮开一局该规模的新游戏。
var (
	button3Rect = image.Rect(188, boardSize+6, 236, boardSize+barHeight-6)
	button4Rect = image.Rect(244, boardSize+6, 292, boardSize+barHeight-6)
)

// Game 实现 ebiten.Game 接口。
type Game struct {
	n         int       // 棋盘规模：3 或 4
	board     []int     // 长度 n²；0 表示空格，1~n² 表示方块上的数字
	moves     int       // 本局已平移的步数
	startTime time.Time // 本局开始时间
	endTime   time.Time // 过关时刻（用于冻结计时）
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
	g.restart(startSize())
	g.dumpFile = os.Getenv("PUZZLE_DUMP")
	if g.dumpFile != "" {
		g.dumpAt = time.Now().Add(1500 * time.Millisecond)
	}
	if err := ebiten.RunGame(g); err != nil && !errors.Is(err, ebiten.Termination) {
		log.Fatal(err)
	}
}

// startSize 解析启动规模：默认 3x3；开发/验证可用 PUZZLE_SIZE=4 指定。
func startSize() int {
	switch s := os.Getenv("PUZZLE_SIZE"); s {
	case "":
		return 3
	case "3", "4":
		v, _ := strconv.Atoi(s)
		return v
	default:
		log.Printf("忽略无效的 PUZZLE_SIZE=%q（仅支持 3 或 4），默认 3x3", s)
		return 3
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

	// 规模按钮：任何时候（含动画中、过关后）都可以点击，
	// 点击即开一局对应规模的新游戏（对当前规模而言相当于再来一局）。
	if image.Pt(x, y).In(button3Rect) {
		g.restart(3)
		return nil
	}
	if image.Pt(x, y).In(button4Rect) {
		g.restart(4)
		return nil
	}

	// 点击游戏区：动画进行中或已过关时不响应。
	if g.anim != nil || g.won || y >= boardSize {
		return nil
	}
	c := g.cellSize()
	idx := (y/c)*g.n + x/c
	if g.board[idx] == 0 {
		return nil
	}
	if blank := g.blankIndex(); adjacent(idx, blank, g.n) {
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
	screen.Fill(bgColor)

	// 棋盘底格
	c := g.cellSize()
	for i := 0; i < g.n*g.n; i++ {
		x, y := g.cellPos(i)
		vector.DrawFilledRect(screen, float32(x)+2, float32(y)+2,
			float32(c-4), float32(c-4), cellColor, false)
	}

	// 方块（正在平移的方块画在插值位置上）
	for i, v := range g.board {
		if v == 0 {
			continue
		}
		x, y := g.cellPos(i)
		fx, fy := float32(x), float32(y)
		if g.anim != nil && g.anim.from == i {
			tx, ty := g.cellPos(g.anim.to)
			p := easeOut(float64(time.Since(g.anim.startTime)) / float64(animTime))
			fx += float32(p) * float32(tx-x)
			fy += float32(p) * float32(ty-y)
		}
		g.drawTile(screen, fx, fy, v)
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

// cellSize 返回当前规模下每个格子的边长（3x3 → 100，4x4 → 75）。
func (g *Game) cellSize() int {
	return boardSize / g.n
}

func (g *Game) cellPos(i int) (x, y int) {
	c := g.cellSize()
	return (i % g.n) * c, (i / g.n) * c
}

// finishAnim 落定当前平移动画：更新棋盘、计步并判断是否过关。
func (g *Game) finishAnim() {
	g.board[g.anim.to] = g.anim.tile
	g.board[g.anim.from] = 0
	g.moves++
	g.anim = nil
	g.checkWin()
}

// restart 开始一局 n x n 规模的新游戏：重新洗牌并重置步数与计时。
func (g *Game) restart(n int) {
	g.n = n
	g.board = shuffledBoard(n)
	g.moves = 0
	g.startTime = time.Now()
	g.won = false
	g.anim = nil
}

// checkWin 判断过关：1~n²-1 按序排列（此时空格必然已在右下角）。
// 过关后冻结计时，并自动在右下角补上数字 n² 的方块。
func (g *Game) checkWin() {
	if !isSolved(g.board) {
		return
	}
	g.won = true
	g.endTime = time.Now()
	g.board[len(g.board)-1] = g.n * g.n
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

// drawStatusBar 绘制底部状态栏：步数、耗时与 [3x3] / [4x4] 规模按钮。
func (g *Game) drawStatusBar(screen *ebiten.Image) {
	vector.DrawFilledRect(screen, 0, boardSize, screenWidth, barHeight, barColor, false)
	vector.DrawFilledRect(screen, 0, boardSize, screenWidth, 2, sepColor, false)

	msg := fmt.Sprintf("步数 %d   时间 %ds", g.moves, int(g.elapsed().Seconds()))
	drawTextLeft(screen, msg, uiFace, 10, boardSize+barHeight/2, textColor)

	drawButton(screen, button3Rect, "3x3")
	drawButton(screen, button4Rect, "4x4")
}

// drawButton 绘制一个小按钮，鼠标悬停时高亮。
func drawButton(screen *ebiten.Image, r image.Rectangle, label string) {
	mx, my := ebiten.CursorPosition()
	c := btnColor
	if image.Pt(mx, my).In(r) {
		c = btnHotColor
	}
	vector.DrawFilledRect(screen, float32(r.Min.X), float32(r.Min.Y),
		float32(r.Dx()), float32(r.Dy()), c, true)
	drawTextCenter(screen, label, uiFace,
		float64(r.Min.X+r.Dx()/2), float64(r.Min.Y+r.Dy()/2), color.White)
}

// drawTile 在 (x, y)（所在格子的左上角）绘制数字为 v 的方块。
func (g *Game) drawTile(screen *ebiten.Image, x, y float32, v int) {
	c := g.cellSize()
	w := float32(c - 2*tileMargin)
	col := tileColor(g.n, v)
	vector.DrawFilledRect(screen, x+tileMargin, y+tileMargin, w, w, col, true)
	vector.StrokeRect(screen, x+tileMargin, y+tileMargin, w, w, 2, dimColor(col), true)
	drawTextCenter(screen, strconv.Itoa(v), tileFaces[g.n],
		float64(x)+float64(c)/2, float64(y)+float64(c)/2, color.White)
}

// tileColor 返回规模 n 下数字 v 方块的底色：1~n²-1 沿蓝→青线性渐变，
// 过关时补上的最后一块 n² 用绿色。
func tileColor(n, v int) color.RGBA {
	if v == n*n {
		return winTileColor
	}
	t := float64(v-1) / float64(n*n-2)
	return color.RGBA{
		lerp(gradientStart.R, gradientEnd.R, t),
		lerp(gradientStart.G, gradientEnd.G, t),
		lerp(gradientStart.B, gradientEnd.B, t),
		0xff,
	}
}

// shuffledBoard 生成一局 n x n 的新棋盘：空格固定在右下角，数字
// 1~n²-1 随机分布在其余格。为保证每局都有解，要求随机排列的逆序数为
// 偶数——由于空格初始位于最后一行，这一判据对 3x3 和 4x4 都成立
// （4x4 的完整判据还与空格所在行有关，空格在最后一行时退化为偶逆序）。
// 同时排除开局即已过关的排列。
func shuffledBoard(n int) []int {
	nums := make([]int, n*n-1)
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
		b := make([]int, n*n)
		copy(b, nums)
		if isSolved(b) {
			continue
		}
		return b
	}
}

// isSolved 判断棋盘是否处于过关局面：前 n²-1 格依次为 1~n²-1，
// 空格在最后一格。
func isSolved(b []int) bool {
	for i := 0; i < len(b)-1; i++ {
		if b[i] != i+1 {
			return false
		}
	}
	return b[len(b)-1] == 0
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

// adjacent 判断 n x n 棋盘上两个格子的下标是否上下左右相邻。
func adjacent(a, b, n int) bool {
	return abs(a%n-b%n)+abs(a/n-b/n) == 1
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// easeOut 是先快后慢的二次缓动，让平移动画更自然。
func easeOut(p float64) float64 {
	if p > 1 {
		p = 1
	}
	return 1 - (1-p)*(1-p)
}

func lerp(a, b uint8, t float64) uint8 {
	return uint8(float64(a) + (float64(b)-float64(a))*t + 0.5)
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
	tileFaces map[int]*textv2.GoTextFace // 各规模的方块数字 face（粗体）
	uiFace    *textv2.GoTextFace         // 状态栏文本（常规）
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
	tileFaces = map[int]*textv2.GoTextFace{
		3: {Source: tileSrc, Size: tileFontRatio * float64(boardSize/3)},
		4: {Source: tileSrc, Size: tileFontRatio * float64(boardSize/4)},
	}
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
