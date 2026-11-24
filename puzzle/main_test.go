package main

import (
	"slices"
	"testing"
	"time"
)

// TestShuffledBoardValid 校验 3x3 与 4x4 的洗牌结果：空格在右下角、
// 数字 1~n²-1 各出现一次、逆序数为偶（有解）、且不是开局即过关的局面。
func TestShuffledBoardValid(t *testing.T) {
	for _, n := range []int{3, 4} {
		for i := 0; i < 300; i++ {
			b := shuffledBoard(n)
			if len(b) != n*n {
				t.Fatalf("n=%d 棋盘长度应为 %d，实际 %d", n, n*n, len(b))
			}
			if b[n*n-1] != 0 {
				t.Fatalf("n=%d 空格应固定在右下角: %v", n, b)
			}
			seen := map[int]bool{}
			for _, v := range b[:n*n-1] {
				if v < 1 || v > n*n-1 || seen[v] {
					t.Fatalf("n=%d 数字 %d 非法或重复: %v", n, v, b)
				}
				seen[v] = true
			}
			if inversions(b[:n*n-1])%2 != 0 {
				t.Fatalf("n=%d 生成了不可解局面: %v", n, b)
			}
			if isSolved(b) {
				t.Fatalf("n=%d 开局即过关: %v", n, b)
			}
		}
	}
}

// TestAdjacent 校验两种规模下的格子相邻判断。
func TestAdjacent(t *testing.T) {
	cases := []struct {
		n    int
		a, b int
		want bool
	}{
		{3, 0, 1, true}, {3, 0, 3, true}, {3, 4, 5, true}, {3, 4, 7, true},
		{3, 0, 4, false}, {3, 1, 3, false}, {3, 0, 2, false}, {3, 0, 6, false},
		{4, 0, 1, true}, {4, 0, 4, true}, {4, 10, 14, true}, {4, 5, 9, true},
		{4, 5, 6, true}, {4, 5, 1, true}, {4, 15, 11, true}, {4, 15, 14, true},
		{4, 0, 5, false}, {4, 3, 8, false}, {4, 0, 3, false}, {4, 0, 12, false},
		{4, 5, 10, false}, {4, 6, 9, false},
	}
	for _, c := range cases {
		if got := adjacent(c.a, c.b, c.n); got != c.want {
			t.Errorf("adjacent(%d, %d, n=%d) = %v, want %v", c.a, c.b, c.n, got, c.want)
		}
	}
}

// TestCellSize 校验两种规模的格子边长。
func TestCellSize(t *testing.T) {
	if c := (&Game{n: 3}).cellSize(); c != 100 {
		t.Errorf("3x3 格子边长应为 100，实际 %d", c)
	}
	if c := (&Game{n: 4}).cellSize(); c != 75 {
		t.Errorf("4x4 格子边长应为 75，实际 %d", c)
	}
}

// TestFinishAnimCountsMoveAndWins3x3 校验 3x3 动画落定后的计步与过关判定：
// 差一步过关的局面，把 8 滑入空格后应过关，且右下角自动补上 9。
func TestFinishAnimCountsMoveAndWins3x3(t *testing.T) {
	g := &Game{n: 3}
	g.board = []int{1, 2, 3, 4, 5, 6, 7, 0, 8}
	g.startTime = time.Now()
	g.anim = &slideAnim{tile: 8, from: 8, to: 7, startTime: time.Now().Add(-time.Second)}

	g.finishAnim()

	if g.moves != 1 {
		t.Fatalf("步数应为 1，实际 %d", g.moves)
	}
	if !g.won {
		t.Fatalf("应判定过关，棋盘 %v", g.board)
	}
	if want := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}; !slices.Equal(g.board, want) {
		t.Fatalf("过关后棋盘应为 %v，实际 %v", want, g.board)
	}
}

// TestFinishAnimCountsMoveAndWins4x4 同上，4x4 规模：把 15 滑入空格后
// 应过关，且右下角自动补上 16。
func TestFinishAnimCountsMoveAndWins4x4(t *testing.T) {
	b := make([]int, 16)
	for i := 0; i < 14; i++ {
		b[i] = i + 1
	}
	b[15] = 15 // b[14] 保持空格

	g := &Game{n: 4, board: b}
	g.startTime = time.Now()
	g.anim = &slideAnim{tile: 15, from: 15, to: 14, startTime: time.Now().Add(-time.Second)}

	g.finishAnim()

	if g.moves != 1 {
		t.Fatalf("步数应为 1，实际 %d", g.moves)
	}
	if !g.won {
		t.Fatalf("应判定过关，棋盘 %v", g.board)
	}
	want := make([]int, 16)
	for i := range want {
		want[i] = i + 1
	}
	if !slices.Equal(g.board, want) {
		t.Fatalf("过关后棋盘应为 %v，实际 %v", want, g.board)
	}
}

// TestFinishAnimNotWin 校验未达成顺序时不误判过关。
func TestFinishAnimNotWin(t *testing.T) {
	g := &Game{n: 3}
	g.board = []int{2, 1, 3, 4, 5, 6, 7, 0, 8}
	g.startTime = time.Now()
	g.anim = &slideAnim{tile: 8, from: 8, to: 7, startTime: time.Now().Add(-time.Second)}

	g.finishAnim()

	if g.won {
		t.Fatal("顺序未达成不应过关")
	}
	if g.board[len(g.board)-1] != 0 {
		t.Fatalf("未过关时右下角应保持空格: %v", g.board)
	}
}

// TestRestartResetsAndSwitchesSize 校验规模按钮行为：重开会重置步数、
// 过关状态，并在 3x3 / 4x4 之间正确切换规模。
func TestRestartResetsAndSwitchesSize(t *testing.T) {
	g := &Game{n: 3}
	g.board = shuffledBoard(3)
	g.moves = 42
	g.won = true
	g.endTime = time.Now()
	g.anim = &slideAnim{tile: 1, from: 0, to: 1, startTime: time.Now()}

	g.restart(4)

	if g.n != 4 {
		t.Fatalf("应切换到 4x4，实际 n=%d", g.n)
	}
	if g.moves != 0 || g.won || g.anim != nil {
		t.Fatalf("restart 未完全重置: moves=%d won=%v anim=%v", g.moves, g.won, g.anim)
	}
	if len(g.board) != 16 || g.board[15] != 0 {
		t.Fatalf("4x4 新开局应为 16 格且空格在右下角: %v", g.board)
	}
	if g.startTime.IsZero() {
		t.Fatal("新开局应重新开始计时")
	}

	g.restart(3)

	if g.n != 3 || len(g.board) != 9 || g.board[8] != 0 {
		t.Fatalf("切回 3x3 异常: n=%d board=%v", g.n, g.board)
	}
}

// TestElapsedFreezesOnWin 校验过关后计时冻结。
func TestElapsedFreezesOnWin(t *testing.T) {
	g := &Game{n: 3}
	start := time.Now().Add(-10 * time.Second)
	g.startTime = start
	g.won = true
	g.endTime = start.Add(3 * time.Second)

	if got := g.elapsed(); got != 3*time.Second {
		t.Fatalf("过关后耗时应冻结在 3s，实际 %v", got)
	}
}

// TestTileColor 校验渐变端点与过关色。
func TestTileColor(t *testing.T) {
	if got, want := tileColor(3, 1), gradientStart; got != want {
		t.Errorf("1 号方块应为渐变起点色 %v，实际 %v", want, got)
	}
	if got, want := tileColor(3, 8), gradientEnd; got != want {
		t.Errorf("3x3 的 8 号方块应为渐变终点色 %v，实际 %v", want, got)
	}
	if got, want := tileColor(4, 15), gradientEnd; got != want {
		t.Errorf("4x4 的 15 号方块应为渐变终点色 %v，实际 %v", want, got)
	}
	if got, want := tileColor(4, 16), winTileColor; got != want {
		t.Errorf("过关补上的 16 号方块应为绿色 %v，实际 %v", want, got)
	}
}
