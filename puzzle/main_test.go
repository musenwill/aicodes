package main

import (
	"testing"
	"time"
)

// TestShuffledBoardValid 校验洗牌结果：空格在右下角、1~8 各出现一次、
// 逆序数为偶（有解）、且不是开局即过关的局面。
func TestShuffledBoardValid(t *testing.T) {
	for i := 0; i < 500; i++ {
		b := shuffledBoard()
		if b[gridN*gridN-1] != 0 {
			t.Fatalf("空格应固定在右下角: %v", b)
		}
		seen := map[int]bool{}
		for _, v := range b[:gridN*gridN-1] {
			if v < 1 || v > gridN*gridN-1 || seen[v] {
				t.Fatalf("数字 %d 非法或重复: %v", v, b)
			}
			seen[v] = true
		}
		if inversions(b[:gridN*gridN-1])%2 != 0 {
			t.Fatalf("生成了不可解局面: %v", b)
		}
		if b == solvedBoard() {
			t.Fatalf("开局即过关: %v", b)
		}
	}
}

// TestAdjacent 校验格子相邻判断。
func TestAdjacent(t *testing.T) {
	cases := []struct {
		a, b int
		want bool
	}{
		{0, 1, true}, {0, 3, true}, {4, 5, true}, {4, 7, true},
		{0, 4, false}, {1, 3, false}, {0, 2, false}, {0, 6, false},
		{2, 5, true}, {8, 5, true}, {8, 7, true}, {2, 6, false},
	}
	for _, c := range cases {
		if got := adjacent(c.a, c.b); got != c.want {
			t.Errorf("adjacent(%d, %d) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

// TestFinishAnimCountsMoveAndWins 校验动画落定后的计步与过关判定：
// 差一步过关的局面，把 8 滑入空格后应过关，且右下角自动补上 9。
func TestFinishAnimCountsMoveAndWins(t *testing.T) {
	g := &Game{}
	g.board = [gridN * gridN]int{1, 2, 3, 4, 5, 6, 7, 0, 8}
	g.startTime = time.Now()
	g.anim = &slideAnim{tile: 8, from: 8, to: 7, startTime: time.Now().Add(-time.Second)}

	g.finishAnim()

	if g.moves != 1 {
		t.Fatalf("步数应为 1，实际 %d", g.moves)
	}
	if !g.won {
		t.Fatalf("应判定过关，棋盘 %v", g.board)
	}
	want := [gridN * gridN]int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	if g.board != want {
		t.Fatalf("过关后棋盘应为 %v，实际 %v", want, g.board)
	}
}

// TestFinishAnimNotWin 校验未达成顺序时不误判过关。
func TestFinishAnimNotWin(t *testing.T) {
	g := &Game{}
	g.board = [gridN * gridN]int{2, 1, 3, 4, 5, 6, 7, 0, 8}
	g.startTime = time.Now()
	g.anim = &slideAnim{tile: 8, from: 8, to: 7, startTime: time.Now().Add(-time.Second)}

	g.finishAnim()

	if g.won {
		t.Fatal("顺序未达成不应过关")
	}
	if g.board[gridN*gridN-1] != 0 {
		t.Fatalf("未过关时右下角应保持空格: %v", g.board)
	}
}

// TestRestartResets 校验再来一局会重置步数、过关状态并重新洗牌。
func TestRestartResets(t *testing.T) {
	g := &Game{}
	g.moves = 42
	g.won = true
	g.endTime = time.Now()
	g.anim = &slideAnim{tile: 1, from: 0, to: 1, startTime: time.Now()}

	g.restart()

	if g.moves != 0 || g.won || g.anim != nil {
		t.Fatalf("restart 未完全重置: moves=%d won=%v anim=%v", g.moves, g.won, g.anim)
	}
	if g.board[gridN*gridN-1] != 0 {
		t.Fatalf("新开局空格应在右下角: %v", g.board)
	}
	if g.startTime.IsZero() {
		t.Fatal("新开局应重新开始计时")
	}
}

// TestElapsedFreezesOnWin 校验过关后计时冻结。
func TestElapsedFreezesOnWin(t *testing.T) {
	g := &Game{}
	start := time.Now().Add(-10 * time.Second)
	g.startTime = start
	g.won = true
	g.endTime = start.Add(3 * time.Second)

	if got := g.elapsed(); got != 3*time.Second {
		t.Fatalf("过关后耗时应冻结在 3s，实际 %v", got)
	}
}
