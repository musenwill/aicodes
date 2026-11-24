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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	textv2 "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// ===== 布局与窗口常量 =====

// 游戏区固定 300x300（3x3 每格 100，4x4 每格 75），底部状态栏 40，
// 窗口整体 300x340，大小固定不可调整。
const (
	boardSize    = 300
	barHeight    = 40
	screenWidth  = boardSize
	screenHeight = boardSize + barHeight // 340

	cellInset  = 2                      // 底格相对格子边缘的内缩，形成格线缝隙
	tileMargin = 4                      // 方块相对格子边缘的留白
	animTime   = 120 * time.Millisecond // 方块平移动画时长

	tileFontRatio = 0.46 // 方块数字字号 = 格子边长 × 该比例
	uiFontSize    = 15

	dumpDelay = 1500 * time.Millisecond // 开发辅助：PUZZLE_DUMP 截图前的渲染等待
)

// ===== 渲染样式 =====

// 界面配色。
var (
	bgColor     = color.RGBA{0x16, 0x21, 0x2e, 0xff}
	cellColor   = color.RGBA{0x20, 0x2d, 0x3d, 0xff}
	barColor    = color.RGBA{0x0f, 0x17, 0x21, 0xff}
	sepColor    = color.RGBA{0x2c, 0x3d, 0x51, 0xff}
	textColor   = color.RGBA{0xd8, 0xe2, 0xec, 0xff}
	btnColor    = color.RGBA{0x2e, 0x75, 0xc8, 0xff}
	btnHotColor = color.RGBA{0x46, 0x92, 0xe6, 0xff}
)

// 方块底色渐变端点（数字 1 为蓝、最大数字为青）与过关补块的绿色。
var (
	gradientStart = color.RGBA{0x2f, 0x6d, 0xb3, 0xff}
	gradientEnd   = color.RGBA{0x29, 0x9c, 0x8a, 0xff}
	winTileColor  = color.RGBA{0x2f, 0x9e, 0x6b, 0xff}
)

// ===== 规模按钮（命中判断与绘制共用同一张表）=====

// supportedSizes 是支持的棋盘规模，按钮表、方块字号表与启动规模校验
// 由此生成；defaultGridSize 是未指定 PUZZLE_SIZE 时的默认规模。
var supportedSizes = []int{3, 4}

const defaultGridSize = 3

// sizeButton 描述状态栏右侧的一个规模按钮。
type sizeButton struct {
	rect  image.Rectangle
	label string
	size  int // 点击后开一局的棋盘规模
}

// sizeButtons 是 [3x3] / [4x4] 按钮表，从右向左等距排列在状态栏右侧。
var sizeButtons = newSizeButtons()

// newSizeButtons 生成规模按钮表，矩形由按钮尺寸常量推导。
func newSizeButtons() []sizeButton {
	const (
		btnWidth   = 48 // 按钮宽度
		btnGap     = 8  // 按钮间距
		btnRight   = 8  // 距窗口右边缘
		btnVMargin = 6  // 距状态栏上下边缘
	)
	total := len(supportedSizes)*btnWidth + (len(supportedSizes)-1)*btnGap
	x := screenWidth - btnRight - total

	buttons := make([]sizeButton, 0, len(supportedSizes))
	for _, n := range supportedSizes {
		buttons = append(buttons, sizeButton{
			rect:  image.Rect(x, boardSize+btnVMargin, x+btnWidth, boardSize+barHeight-btnVMargin),
			label: fmt.Sprintf("%dx%d", n, n),
			size:  n,
		})
		x += btnWidth + btnGap
	}
	return buttons
}

// ===== 游戏状态 =====

// Game 实现 ebiten.Game 接口。
type Game struct {
	gridSize  int        // 棋盘规模：3 或 4
	board     []int      // 长度 gridSize²；0 表示空格，1~gridSize² 表示方块数字
	moves     int        // 本局已平移的步数
	startTime time.Time  // 本局开始时间
	endTime   time.Time  // 过关时刻（用于冻结计时）
	won       bool       // 本局是否已过关
	anim      *slideAnim // 当前平移动画，nil 表示空闲

	fonts *fontSet   // 渲染所需字体集（构造时注入）
	dump  *devDumper // 开发辅助截图器，nil 表示未启用
}

// slideAnim 表示一次方块平移：方块从 from 格滑向 to 格（to 为空格所在格）。
type slideAnim struct {
	tile      int
	from, to  int
	startTime time.Time
}

// ===== 入口 =====

func main() {
	fonts, err := loadFonts()
	if err != nil {
		log.Fatal(err)
	}

	ebiten.SetWindowTitle("puzzle")
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeDisabled)

	if err := ebiten.RunGame(newGame(fonts)); err != nil && !errors.Is(err, ebiten.Termination) {
		log.Fatal(err)
	}
}

// newGame 构造一局新游戏：注入字体集；开发辅助环境变量
// （PUZZLE_SIZE / PUZZLE_DUMP）也在此解析，日常游玩不设置即可。
func newGame(fonts *fontSet) *Game {
	g := &Game{fonts: fonts, dump: newDevDumper()}
	g.restart(startSize())
	return g
}

// startSize 解析 PUZZLE_SIZE 作为启动规模：合法值须在 supportedSizes 内，
// 缺省或非法值告警后回退 defaultGridSize。
func startSize() int {
	s := os.Getenv("PUZZLE_SIZE")
	if s == "" {
		return defaultGridSize
	}
	if n, err := strconv.Atoi(s); err == nil && slices.Contains(supportedSizes, n) {
		return n
	}
	log.Printf("忽略无效的 PUZZLE_SIZE=%q（支持 %v），默认 %d", s, supportedSizes, defaultGridSize)
	return defaultGridSize
}

// ===== ebiten.Game 接口实现 =====

// Update 是每帧的编排入口：退出控制 → 窗口守卫 → 动画推进 → 输入分发。
func (g *Game) Update() error {
	if g.dump != nil && g.dump.wantsQuit() {
		return ebiten.Termination
	}
	g.keepWindowSizeFixed()
	g.advanceAnim()
	g.dispatchInput()
	return nil
}

// keepWindowSizeFixed 在窗口大小偏离固定值时立即拉回。
// 尺寸提示（WindowResizingModeDisabled 已设置）在多数平台足以锁死窗口，
// 但 WSLg 的窗口管理器会忽略提示（microsoft/wslg#1223），此守卫兜底。
func (g *Game) keepWindowSizeFixed() {
	if w, h := ebiten.WindowSize(); w != screenWidth || h != screenHeight {
		ebiten.SetWindowSize(screenWidth, screenHeight)
	}
}

// advanceAnim 推进平移动画：时间到则落定、计步并判断是否过关。
func (g *Game) advanceAnim() {
	if g.anim != nil && time.Since(g.anim.startTime) >= animTime {
		g.finishAnim()
	}
}

// dispatchInput 处理本帧的鼠标左键点击：先判规模按钮，再判棋盘方块。
func (g *Game) dispatchInput() {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	x, y := ebiten.CursorPosition()

	// 规模按钮：任何时候（含动画中、过关后）都可点击，
	// 点击即开一局对应规模的新游戏（对当前规模相当于再来一局）。
	for _, b := range sizeButtons {
		if image.Pt(x, y).In(b.rect) {
			g.restart(b.size)
			return
		}
	}
	g.tryMoveTileAt(x, y)
}

// tryMoveTileAt 尝试把 (x, y) 处的方块平移进相邻空格：点击越界、
// 点中空格、方块与空格不相邻、动画进行中或已过关时均无动作。
func (g *Game) tryMoveTileAt(x, y int) {
	if g.anim != nil || g.won {
		return
	}
	if x < 0 || y < 0 || x >= boardSize || y >= boardSize {
		return
	}
	cell := g.cellSize()
	idx := (y/cell)*g.gridSize + x/cell
	if g.board[idx] == 0 {
		return
	}
	if blank := g.blankIndex(); adjacent(idx, blank, g.gridSize) {
		g.anim = &slideAnim{
			tile:      g.board[idx],
			from:      idx,
			to:        blank,
			startTime: time.Now(),
		}
	}
}

// Draw 每帧渲染：棋盘 → 状态栏 → 开发辅助截图（如启用）。
func (g *Game) Draw(screen *ebiten.Image) {
	g.drawBoard(screen)
	g.drawStatusBar(screen)
	if g.dump != nil {
		g.dump.maybeCapture(screen)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

// ===== 渲染 =====

// drawBoard 绘制棋盘的底格与方块（平移中的方块画在插值位置上）。
func (g *Game) drawBoard(screen *ebiten.Image) {
	for i := range g.board {
		x, y := g.cellPos(i)
		vector.DrawFilledRect(screen, float32(x)+cellInset, float32(y)+cellInset,
			float32(g.cellSize()-2*cellInset), float32(g.cellSize()-2*cellInset), cellColor, false)
	}

	for i, v := range g.board {
		if v == 0 {
			continue
		}
		fx, fy := g.tileDrawPos(i)
		g.drawTile(screen, fx, fy, v)
	}
}

// tileDrawPos 返回方块 i 当前的绘制位置（像素）：若正处于平移动画中，
// 返回从起点格到空格的缓动插值位置，否则返回格子本身的位置。
func (g *Game) tileDrawPos(i int) (fx, fy float32) {
	x, y := g.cellPos(i)
	fx, fy = float32(x), float32(y)
	if g.anim == nil || g.anim.from != i {
		return
	}
	tx, ty := g.cellPos(g.anim.to)
	p := easeOut(float64(time.Since(g.anim.startTime)) / float64(animTime))
	fx += float32(p) * float32(tx-x)
	fy += float32(p) * float32(ty-y)
	return
}

// drawStatusBar 绘制底部状态栏：步数、耗时与规模按钮。
func (g *Game) drawStatusBar(screen *ebiten.Image) {
	vector.DrawFilledRect(screen, 0, boardSize, screenWidth, barHeight, barColor, false)
	vector.DrawFilledRect(screen, 0, boardSize, screenWidth, 2, sepColor, false) // 分隔线

	msg := fmt.Sprintf("步数 %d   时间 %ds", g.moves, int(g.elapsed().Seconds()))
	drawText(screen, msg, g.fonts.uiFace, textv2.AlignStart, 10,
		boardSize+barHeight/2, textColor)

	for _, b := range sizeButtons {
		drawButton(screen, b.rect, b.label, g.fonts.uiFace)
	}
}

// drawButton 绘制一个小按钮（鼠标悬停时高亮），文本用 face 渲染。
func drawButton(screen *ebiten.Image, r image.Rectangle, label string, face textv2.Face) {
	mx, my := ebiten.CursorPosition()
	col := btnColor
	if image.Pt(mx, my).In(r) {
		col = btnHotColor
	}
	vector.DrawFilledRect(screen, float32(r.Min.X), float32(r.Min.Y),
		float32(r.Dx()), float32(r.Dy()), col, true)
	drawText(screen, label, face, textv2.AlignCenter,
		float64(r.Min.X+r.Dx()/2), float64(r.Min.Y+r.Dy()/2), color.White)
}

// drawTile 在 (x, y)（所在格子的左上角）绘制数字为 v 的方块。
func (g *Game) drawTile(screen *ebiten.Image, x, y float32, v int) {
	cell := g.cellSize()
	w := float32(cell - 2*tileMargin)
	col := tileColor(g.gridSize, v)
	vector.DrawFilledRect(screen, x+tileMargin, y+tileMargin, w, w, col, true)
	vector.StrokeRect(screen, x+tileMargin, y+tileMargin, w, w, 2, dimColor(col), true)
	drawText(screen, strconv.Itoa(v), g.fonts.tileFaces[g.gridSize], textv2.AlignCenter,
		float64(x)+float64(cell)/2, float64(y)+float64(cell)/2, color.White)
}

// ===== 游戏逻辑 =====

// cellSize 返回当前规模下每个格子的边长（3x3 → 100，4x4 → 75）。
func (g *Game) cellSize() int {
	return boardSize / g.gridSize
}

// cellPos 返回格子 i 左上角的像素坐标。
func (g *Game) cellPos(i int) (x, y int) {
	cell := g.cellSize()
	return (i % g.gridSize) * cell, (i / g.gridSize) * cell
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
	g.gridSize = n
	g.board = shuffledBoard(n)
	g.moves = 0
	g.startTime = time.Now()
	g.won = false
	g.anim = nil
}

// checkWin 判断过关：1~gridSize²-1 按序排列（此时空格必然已在右下角）。
// 过关后冻结计时，并自动在右下角补上数字 gridSize² 的方块。
func (g *Game) checkWin() {
	if !isSolved(g.board) {
		return
	}
	g.won = true
	g.endTime = time.Now()
	g.board[len(g.board)-1] = g.gridSize * g.gridSize
}

// elapsed 返回本局已进行的时间；过关后冻结在过关那一刻。
func (g *Game) elapsed() time.Duration {
	if g.won {
		return g.endTime.Sub(g.startTime)
	}
	return time.Since(g.startTime)
}

// blankIndex 返回空格所在格的下标。
func (g *Game) blankIndex() int {
	for i, v := range g.board {
		if v == 0 {
			return i
		}
	}
	return -1 // 不会发生：棋盘上恒有一个空格
}

// ===== 棋盘纯函数 =====

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
	count := 0
	for i := range nums {
		for j := i + 1; j < len(nums); j++ {
			if nums[i] > nums[j] {
				count++
			}
		}
	}
	return count
}

// adjacent 判断 n x n 棋盘上两个格子的下标是否上下左右相邻。
func adjacent(a, b, n int) bool {
	return abs(a%n-b%n)+abs(a/n-b/n) == 1
}

// abs 返回整数的绝对值。
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// easeOut 是先快后慢的二次缓动（p 为 0~1 的动画进度）。
func easeOut(p float64) float64 {
	if p > 1 {
		p = 1
	}
	return 1 - (1-p)*(1-p)
}

// lerp 计算颜色通道 a→b 的线性插值（t 为 0~1 的比例），四舍五入。
func lerp(a, b uint8, t float64) uint8 {
	return uint8(float64(a) + (float64(b)-float64(a))*t + 0.5)
}

// dimColor 返回颜色的暗化版本（各通道 × 3/4），用作方块描边。
func dimColor(c color.RGBA) color.RGBA {
	return color.RGBA{c.R * 3 / 4, c.G * 3 / 4, c.B * 3 / 4, 0xff}
}

// ===== 文本绘制 =====

// drawText 在指定位置绘制文本：水平方向按 hAlign 对齐（x 为该对齐边的
// 坐标），垂直方向以 cy 为中心。
func drawText(dst *ebiten.Image, s string, face textv2.Face, hAlign textv2.Align, x, cy float64, clr color.Color) {
	op := &textv2.DrawOptions{}
	op.PrimaryAlign = hAlign
	op.SecondaryAlign = textv2.AlignCenter
	op.GeoM.Translate(x, cy)
	op.ColorScale.ScaleWithColor(clr)
	textv2.Draw(dst, s, face, op)
}

// ===== 字体 =====

// fontSet 是渲染所需的字体集：各规模的方块数字 face 与状态栏文本 face。
type fontSet struct {
	tileFaces map[int]*textv2.GoTextFace // 按规模索引的方块数字 face（粗体）
	uiFace    *textv2.GoTextFace         // 状态栏文本 face（常规）
}

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

// loadFonts 加载渲染所需的中文字体（运行时从系统字体目录读取）。
func loadFonts() (*fontSet, error) {
	tileSrc := loadFontSource(boldFontCandidates)
	if tileSrc == nil {
		tileSrc = loadFontSource(regularFontCandidates)
	}
	uiSrc := loadFontSource(regularFontCandidates)
	if uiSrc == nil {
		uiSrc = tileSrc
	}
	if tileSrc == nil {
		return nil, fmt.Errorf("未找到可用的中文字体（Bold 候选 %v；Regular 候选 %v）",
			boldFontCandidates, regularFontCandidates)
	}
	faces := make(map[int]*textv2.GoTextFace, len(supportedSizes))
	for _, n := range supportedSizes {
		faces[n] = &textv2.GoTextFace{
			Source: tileSrc,
			Size:   tileFontRatio * float64(boardSize/n),
		}
	}
	return &fontSet{
		tileFaces: faces,
		uiFace:    &textv2.GoTextFace{Source: uiSrc, Size: uiFontSize},
	}, nil
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

// ===== 开发辅助（PUZZLE_DUMP）=====

// devDumper 是开发辅助截图器：渲染 dumpDelay 后把画面保存为 PNG 并请求
// 退出，用于自动化视觉验证。通过 PUZZLE_DUMP 环境变量启用（newDevDumper），
// 日常游玩不设置即可。
type devDumper struct {
	file string
	at   time.Time
	done bool
	quit bool // 已截图保存，请求退出
}

// newDevDumper 解析 PUZZLE_DUMP 环境变量；未设置时返回 nil（功能关闭）。
func newDevDumper() *devDumper {
	file := os.Getenv("PUZZLE_DUMP")
	if file == "" {
		return nil
	}
	return &devDumper{file: file, at: time.Now().Add(dumpDelay)}
}

// wantsQuit 报告是否已截图完毕并请求退出。
func (d *devDumper) wantsQuit() bool {
	return d.quit
}

// maybeCapture 到点后截图保存并置退出标记；未到点或已保存时无动作。
func (d *devDumper) maybeCapture(screen *ebiten.Image) {
	if d.done || time.Now().Before(d.at) {
		return
	}
	if err := dumpScreen(d.file, screen); err != nil {
		log.Print("截图失败: ", err)
	}
	d.done = true
	d.quit = true
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
